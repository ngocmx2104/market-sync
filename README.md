# market-sync

A Go service that keeps marketplace orders in sync with a local store **without losing, duplicating or stalling** — built around the failure modes that real marketplace and POS APIs produce in production.

Every feature exists because of an incident I have dealt with while running multi-tenant e-commerce data pipelines. The incidents are reproduced by a fake marketplace with chaos modes, and each one has an *incident replay test*.

> Status: **work in progress.** The skeleton, local stack, fake marketplace, SQS adapter and gRPC control API run today; the core sync and reconcile logic is being written (see the roadmap below).

## Incidents → features

| # | Incident (generalized) | Feature | Storage |
|---|---|---|---|
| 1 | Orders deleted at the source stay "new" forever: an `updated_at` window never sees them again | Reconciler re-checks open orders by id and writes tombstones, behind a **guard** that refuses to mark anything when too many orders look deleted at once | MySQL |
| 2 | The source's `updated_at` filter misses late and bulk updates | Watermark with an overlap window, plus periodic bucketed checksums to find drift without a full rescan | DynamoDB, MySQL |
| 3 | One id the source rejects fails the whole detail batch; retries and the DLQ do not help, the watermark never moves | Poison-record isolation: drop the ids the source names (or bisect), quarantine them, keep going | MySQL, SQS DLQ |
| 4 | A large shop loaded fully into memory took the host down | Streaming pagination with bounded memory and backpressure | SQS |
| 5 | Two environments shared one credential and starved each other's quota | Quota-aware scheduling: one token bucket per credential, shared by all workers | DynamoDB |
| 6 | A field changed type at the source and whole orders silently vanished | Per-record contract validation: reject the record with a reason, not the batch | MySQL |
| 7 | Errors were truncated strings; nobody knew what to retry | Structured `ErrorDetail` (code, step, ids, retryable, attempts), retry with backoff + jitter | MySQL |

## Architecture

```
 tenant ──REST (Gin)──► api ─────────────────► MySQL / Aurora MySQL
                                                 ▲  orders, sync_runs, quarantine,
 operator ──gRPC──► control ──enqueue──► SQS     │  rejections, reconcile_reports
                                          │      │
                                          ▼      │ conditional upsert (newer wins)
                                        worker ──┘
                                          │  ▲
                        watermark / quota │  │ list ids by updated_at, fetch details in batches
                                          ▼  │
                                     DynamoDB   fake-market (chaos: 429, 5xx, poison ids,
                                                 deletions, hidden updates, schema drift)
                reconciler (periodic) ──► lookup by id ──► refresh / tombstone / guard
```

- **Hexagonal layout:** `internal/domain` (types, no dependencies) → `internal/ports` (interfaces) → `internal/app` (use cases) → `internal/adapters` (MySQL, DynamoDB, SQS, HTTP).
- **Delivery:** at-least-once (SQS) + idempotent, order-aware upserts, instead of trying to get exactly-once.

## Run locally

Requirements: Go 1.25+, Docker.

```bash
make up                # MySQL 8.4, ElasticMQ (SQS API), DynamoDB Local, fake-market + migrations
make test              # unit tests (race detector on)
make test-integration  # needs `make up`
make run-control       # gRPC on :9090
make run-worker
make down
```

Play with the fake marketplace:

```bash
curl 'localhost:8090/v1/shops/shop-a/orders?updated_from=2026-01-01T00:00:00Z&updated_to=2027-01-01T00:00:00Z&page_size=5'
curl -X PUT localhost:8090/admin/chaos -d '{"poison_ids":["shop-a-0000003"],"rate_limit_rps":5,"rate_burst":5}'
curl -X POST 'localhost:8090/admin/shops/shop-a/delete?count=5'
curl -X POST 'localhost:8090/admin/shops/shop-a/late-update?count=10'
```

## Layout

```
cmd/            api (Gin) · control (gRPC) · worker · reconciler · fake-market
internal/
  domain/       Order, SyncJob, Cursor, ErrorDetail
  ports/        Source, OrderStore, CursorStore, Queue, QuotaLimiter
  app/          syncer (worker), reconcile
  adapters/     mysqlstore, dynamostore, sqsqueue, marketclient
  fakemarket/   fake marketplace + chaos modes
api/proto/      gRPC contract (buf) → gen/
migrations/     MySQL schema (golang-migrate)
deploy/local/   docker compose stack
deploy/terraform/  AWS (ECS Fargate, Aurora Serverless v2, DynamoDB, SQS)
test/           integration and incident replay tests (-tags=integration)
docs/adr/       architecture decision records
```

## Roadmap

- [x] Skeleton, local stack, fake marketplace with chaos modes, SQS adapter, gRPC control API, CI
- [ ] MySQL order store: conditional upsert, keyset pagination, tombstones (`docs/queries.md` with `EXPLAIN`)
- [ ] DynamoDB cursor store with optimistic concurrency
- [ ] Worker: streaming pages, poison isolation, retries, watermark with overlap (incidents 2, 3, 4, 7)
- [ ] Reconciler with guard (incident 1)
- [ ] Incident replay tests green
- [ ] Quota limiter (5) and contract validation (6)
- [ ] Terraform deploy to AWS + load test numbers
- [ ] Observability: OpenTelemetry traces, Prometheus metrics

## How this repo was built

Design decisions, schema and the core sync/reconcile logic are mine; I use AI assistants (Claude Code) for scaffolding, boilerplate and review, with `AGENTS.md` and `docs/` as the context they work from.
