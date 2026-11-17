# AGENTS.md

Context for AI coding agents working in this repo.

## What this is
A Go sync engine for marketplace orders. Read `README.md` (incidents → features) and `docs/adr/` before changing behaviour.

## Rules
- Layering: `domain` imports nothing internal; `app` depends on `ports` only; vendor SDKs live in `adapters`. Do not import adapters from `app`.
- Delivery is at-least-once. Every write must be idempotent and order-aware (newer `source_updated_at` wins).
- Never hard-delete orders; use tombstones (`deleted_at`).
- Errors crossing a boundary are `*domain.ErrorDetail` with a code and `Retryable` set correctly.
- Pass `context.Context` through every I/O call; no goroutine without a way to stop it.
- Code, comments and commit messages in English.
- `TODO(core)` marks logic the maintainer writes by hand. Do not implement it unless asked; you may write tests for it.

## Commands
- `make test` — unit tests (race)
- `make up && make test-integration` — integration tests against the local stack
- `make lint` — golangci-lint v2
- `make proto` — regenerate gRPC code into `gen/` (never edit `gen/` by hand)

## Where things are
- Chaos modes of the fake marketplace: `internal/fakemarket/server.go` (`Chaos`)
- Schema: `migrations/`
- Incident replay tests (the spec): `test/incidents/`
