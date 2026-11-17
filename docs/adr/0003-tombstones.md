# ADR 0003: Tombstones instead of hard deletes, behind a guard

- Status: proposed
- Date: 2026-11-17

## Context
Orders deleted at the source must stop appearing as open downstream (incident 1). But "missing at the source" can also mean an outage, an expired credential, a wrong shop id or a pagination bug at the source. A hard delete in those cases destroys data that cannot be re-fetched.

## Options
1. Hard delete missing orders.
2. Tombstone (`deleted_at`) missing orders, always.
3. Tombstone, but stop and alert when the share of missing orders in one pass exceeds a threshold.

## Decision
TODO

## Consequences
TODO (how is the threshold chosen? how does an operator release a tripped guard? what do reports show for tombstoned orders?)
