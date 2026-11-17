// Package reconcile finds orders that changed or disappeared at the source
// without ever showing up again in an updated_at window.
package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ngocmx2104/market-sync/internal/ports"
)

var ErrNotImplemented = errors.New("not implemented")

// ErrGuardTripped means too many orders looked deleted at once. That is far
// more likely to be a source outage, a wrong credential or a bug than real
// deletions, so nothing is marked and an operator must look.
type ErrGuardTripped struct {
	Checked, Missing int
	MaxRatio         float64
}

func (e *ErrGuardTripped) Error() string {
	return fmt.Sprintf("reconcile guard tripped: %d of %d open orders missing (max ratio %.2f)", e.Missing, e.Checked, e.MaxRatio)
}

// Report summarizes one reconcile pass.
type Report struct {
	Checked   int
	Refreshed int // still present, newer version stored
	Deleted   int // tombstoned
}

type Reconciler struct {
	Source ports.Source
	Orders ports.OrderStore
	Log    *slog.Logger

	// MaxMissingRatio is the guard: above it, nothing is tombstoned.
	MaxMissingRatio float64
}

// RunOnce checks every open order of one tenant against the source.
func (r *Reconciler) RunOnce(ctx context.Context, tenantID string) (Report, error) {
	// TODO(core) (incidents #1 and #2):
	//  1. Page through Orders.ListOpenOrderIDs (keyset).
	//  2. Look them up with Source.GetOrders in batches of MaxBatch().
	//  3. Present → upsert (catches updates the list filter missed).
	//     Absent  → candidate for deletion.
	//  4. If candidates/checked > MaxMissingRatio → return *ErrGuardTripped
	//     and mark nothing. Otherwise Orders.MarkDeleted.
	//  Later: bucketed checksums per day to find drift without a full scan.
	return Report{}, ErrNotImplemented
}
