# ADR 0002: At-least-once delivery with idempotent, order-aware upserts

- Status: proposed
- Date: 2026-11-17

## Context
SQS standard queues deliver at least once and do not preserve order. Workers crash mid-job. The same order can arrive from an incremental sync, a retry and the reconciler, in any order.

## Options
1. Exactly-once via FIFO queues + deduplication ids.
2. At-least-once + idempotent upsert keyed on (tenant, source, external_id), applied only when the incoming `source_updated_at` is newer.
3. Outbox / event log with sequence numbers.

## Decision
TODO

## Consequences
TODO (ties on equal `updated_at`? clock skew at the source?)
