# ADR 0004: Isolate poison records instead of failing the batch

- Status: proposed
- Date: 2026-11-17

## Context
Some marketplace APIs reject a whole detail call when one id in it is invalid, and only name the bad id inside a free-text message (incident 3). Retrying the same batch fails forever, the job ends in the DLQ and the watermark stops.

## Options
1. Retry, then DLQ the whole job (status quo).
2. Parse the bad ids from the error, drop them, retry the rest.
3. Bisect the batch until the bad ids are found (works when the error names nothing).
4. 2 first, 3 as a fallback; quarantine what is found.

## Decision
TODO

## Consequences
TODO (cost of bisecting: how many extra calls in the worst case? how are quarantined ids retried later?)
