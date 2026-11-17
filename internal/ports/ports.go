// Package ports declares the interfaces the application core depends on.
// Adapters (MySQL, DynamoDB, SQS, HTTP clients) implement them; tests can
// replace them with in-memory fakes.
package ports

import (
	"context"
	"time"

	"github.com/ngocmx2104/market-sync/internal/domain"
)

// OrderRef is the summary a source returns when listing: id + updated_at.
type OrderRef struct {
	ExternalID string
	UpdatedAt  time.Time
}

// Page is one page of order refs returned by a source.
type Page struct {
	Refs    []OrderRef
	Page    int
	HasMore bool
	Total   int // total matching records, if the source reports it
}

// Source is a marketplace API. Like most marketplaces it is two-step:
// list ids by updated_at, then fetch details in batches.
type Source interface {
	Name() string
	// ListOrders returns one page of order refs whose updated_at is in w.
	// Pages are 1-based.
	ListOrders(ctx context.Context, tenantID string, w domain.Window, page, pageSize int) (Page, error)
	// GetOrders fetches order details by id (at most MaxBatch ids). Ids
	// missing from both results do not exist (any more) at the source.
	// Records that break the payload contract come back in rejected instead
	// of failing the call. If the source refuses the whole call because of
	// specific ids, the error is a *domain.ErrorDetail with
	// Code == CodePoisonRecord and ObjectIDs set when the source names them.
	GetOrders(ctx context.Context, tenantID string, ids []string) (orders []domain.Order, rejected []Rejected, err error)
	MaxBatch() int
}

// Rejected is one record that could not be decoded or validated.
type Rejected struct {
	ExternalID string
	Raw        []byte
	Detail     domain.ErrorDetail
}

// UpsertResult reports what an upsert batch did.
type UpsertResult struct {
	Inserted int
	Updated  int
	Skipped  int // stale: the stored copy was already newer
}

// OrderStore persists orders.
type OrderStore interface {
	// UpsertOrders writes orders idempotently. A row is only overwritten when
	// the incoming SourceUpdatedAt is newer than the stored one.
	UpsertOrders(ctx context.Context, orders []domain.Order) (UpsertResult, error)
	// ListOpenOrderIDs returns external ids of non-terminal, non-deleted
	// orders, using keyset pagination on afterID.
	ListOpenOrderIDs(ctx context.Context, tenantID, source, afterID string, limit int) ([]string, error)
	// MarkDeleted writes tombstones for orders that disappeared at the source.
	MarkDeleted(ctx context.Context, keys []domain.Key, at time.Time) (int, error)
}

// QuarantineStore keeps records the source refuses to serve, so one bad
// record never blocks the rest of a batch.
type QuarantineStore interface {
	Quarantine(ctx context.Context, tenantID, source string, externalIDs []string, detail domain.ErrorDetail) error
}

// CursorStore persists sync watermarks with optimistic concurrency.
type CursorStore interface {
	Get(ctx context.Context, tenantID, source, resource string) (domain.Cursor, bool, error)
	// Advance moves the watermark forward only if the stored version still
	// equals c.Version. It must never move the watermark backwards.
	Advance(ctx context.Context, c domain.Cursor, newWatermark time.Time) (domain.Cursor, error)
}

// Message is a job received from the queue.
type Message struct {
	Job           domain.SyncJob
	ReceiptHandle string
	ReceiveCount  int
}

// Queue carries sync jobs between the scheduler and workers (at-least-once).
type Queue interface {
	Enqueue(ctx context.Context, jobs ...domain.SyncJob) error
	Receive(ctx context.Context, maxMsgs int, wait time.Duration) ([]Message, error)
	Ack(ctx context.Context, m Message) error
	// ExtendLease pushes the visibility timeout of a long-running job.
	ExtendLease(ctx context.Context, m Message, d time.Duration) error
}

// QuotaLimiter shares a source's rate limit across all workers and tenants
// that use the same credential.
type QuotaLimiter interface {
	// Wait blocks until one request on credentialID is allowed or ctx ends.
	Wait(ctx context.Context, credentialID string) error
}
