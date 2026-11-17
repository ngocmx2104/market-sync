//go:build integration

// Package incidents holds "incident replay" tests: each one turns on a
// fake-market chaos mode that reproduces a production failure, runs the real
// worker/reconciler against MySQL + SQS + DynamoDB, and asserts the system
// recovers. They are the executable spec of market-sync; remove t.Skip as
// each feature lands.
package incidents_test

import "testing"

// Incident #1: an order deleted at the source never shows up in an
// updated_at window again, so it stays "new" forever downstream.
func TestDeletedAtSourceBecomesTombstone(t *testing.T) {
	t.Skip("TODO: sync shop → admin delete 5 open orders → reconcile → 5 rows have deleted_at, others untouched")
}

// Incident #1 (guard): a source outage or wrong credential looks like mass
// deletion. Nothing may be marked.
func TestGuardStopsMassDeletion(t *testing.T) {
	t.Skip("TODO: delete 50% of open orders → reconcile returns ErrGuardTripped → 0 rows tombstoned → report has guard_hit")
}

// Incident #2: the source's updated_at filter misses late or bulk updates.
func TestLateUpdateIsCaughtByReconcile(t *testing.T) {
	t.Skip("TODO: admin late-update 10 orders (hidden from list) → incremental sync misses them → reconcile stores status=completed")
}

// Incident #3: one poison id fails the whole detail batch; retries and the
// DLQ do not help and the watermark never moves.
func TestPoisonIDDoesNotBlockChunk(t *testing.T) {
	t.Skip("TODO: chaos poison_ids=[x] → sync window → all orders but x stored, x in quarantine, watermark advanced, run status partial")
}

// Incident #4: a big shop loaded fully into memory took the host down.
func TestMemoryBoundedOnLargeShop(t *testing.T) {
	t.Skip("TODO: seed 1M orders → sync → peak heap (runtime/metrics) stays under a fixed budget independent of shop size")
}

// Incident #5: two environments shared one credential and starved each other.
func TestSharedQuotaIsRespected(t *testing.T) {
	t.Skip("TODO: chaos rate_limit_rps=5 → two workers on one credential → zero 429s observed, both tenants progress")
}

// Incident #6: a field changed type at the source and whole orders vanished.
func TestSchemaDriftRejectsRecordNotBatch(t *testing.T) {
	t.Skip("TODO: chaos schema_drift_rate=0.1 → ~10% in rejections with reason, ~90% stored, run status partial")
}

// Incident #7: operators could not tell retryable from fatal errors.
func TestTransientErrorsAreRetriedWithBackoff(t *testing.T) {
	t.Skip("TODO: chaos transient_error_rate=0.3 → run succeeds, attempts recorded in error_detail, no job in DLQ")
}
