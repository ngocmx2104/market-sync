// Package syncer turns queued sync jobs into stored orders.
package syncer

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/ports"
)

// ErrNotImplemented marks the parts still to be written.
var ErrNotImplemented = errors.New("not implemented")

// Worker consumes sync jobs. Delivery is at-least-once: a job that fails is
// not acked, reappears after the visibility timeout, and moves to the DLQ
// after the queue's maxReceiveCount.
type Worker struct {
	Queue      ports.Queue
	Source     ports.Source
	Orders     ports.OrderStore
	Quarantine ports.QuarantineStore
	Cursors    ports.CursorStore
	Log        *slog.Logger

	PageSize    int           // refs per list call
	Concurrency int           // jobs processed at once
	PollWait    time.Duration // SQS long-poll wait
}

// Run polls until ctx is cancelled. In-flight jobs finish before it returns.
func (w *Worker) Run(ctx context.Context) error {
	// TODO(core): process up to w.Concurrency jobs at once (semaphore or
	// fixed pool), extend the lease of long jobs, and drain in-flight jobs on
	// shutdown instead of dropping them. This version is sequential.
	for ctx.Err() == nil {
		msgs, err := w.Queue.Receive(ctx, 1, w.PollWait)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			w.Log.Error("receive failed", "err", err)
			time.Sleep(time.Second)
			continue
		}
		for _, m := range msgs {
			log := w.Log.With("job_id", m.Job.JobID, "tenant", m.Job.TenantID, "window", m.Job.Window.String(), "receive_count", m.ReceiveCount)
			if err := w.Handle(ctx, m.Job); err != nil {
				log.Error("job failed, will be redelivered", "err", err)
				continue
			}
			if err := w.Queue.Ack(ctx, m); err != nil {
				log.Error("ack failed", "err", err)
			}
		}
	}
	return nil
}

// Handle runs one job end to end.
func (w *Worker) Handle(ctx context.Context, job domain.SyncJob) error {
	// TODO(core) — the heart of market-sync. Suggested steps:
	//  1. Stream pages of refs (Source.ListOrders) without holding the whole
	//     window in memory (incident #4: bounded memory, backpressure).
	//  2. Fetch details in batches of Source.MaxBatch(). On CodePoisonRecord,
	//     drop the named ids (or bisect when none are named), quarantine them,
	//     and keep going (incident #3).
	//  3. Store records the source returned as rejected (incident #6).
	//  4. UpsertOrders idempotently (only newer source_updated_at wins).
	//  5. Retry retryable errors with backoff + jitter; fail fast on the rest
	//     and record a domain.ErrorDetail on the sync run (incident #7).
	//  6. Advance the watermark only once the whole window is stored, minus
	//     an overlap so late writes are seen again (incident #2).
	return ErrNotImplemented
}
