# ADR 0001: MySQL for orders, DynamoDB for cursors and quota

- Status: proposed
- Date: 2026-11-17

## Context
Orders are relational and queried in many ways: by tenant and updated_at for the API, by open status for the reconciler, aggregated for reports. Cursors (watermarks) and quota buckets are tiny, hot, key-addressed records that many workers update concurrently and that need conditional writes.

## Options
1. Everything in MySQL (cursor rows with `SELECT … FOR UPDATE` or version columns).
2. MySQL for orders, DynamoDB for cursors and quota.
3. Everything in DynamoDB.

## Decision
TODO: write the decision and the reasons in your own words.

## Consequences
TODO: what gets easier, what gets harder (two stores to operate, no cross-store transaction: what happens if the upsert succeeds and the cursor advance fails?).
