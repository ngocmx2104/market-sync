package domain

import (
	"fmt"
	"time"
)

// Window is a half-open time range [From, To) on the source's updated_at.
type Window struct {
	From time.Time
	To   time.Time
}

func (w Window) String() string {
	return fmt.Sprintf("[%s, %s)", w.From.Format(time.RFC3339), w.To.Format(time.RFC3339))
}

// SyncJob is the unit of work put on the queue: one tenant, one source,
// one resource, one window, optionally restricted to a page range.
type SyncJob struct {
	JobID     string
	TenantID  string
	Source    string
	Resource  string // "orders" for now
	Window    Window
	PageFrom  int // inclusive, 0 = start
	PageTo    int // exclusive, 0 = until the last page
	Attempt   int
	CreatedAt time.Time
}

// Cursor is the persisted sync progress of one (tenant, source, resource).
type Cursor struct {
	TenantID  string
	Source    string
	Resource  string
	Watermark time.Time // everything with updated_at < Watermark has been synced
	UpdatedAt time.Time
	Version   int64 // optimistic concurrency token
}

// RunStatus is the outcome of one sync run.
type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunPartial   RunStatus = "partial" // finished, but some records were quarantined or rejected
	RunFailed    RunStatus = "failed"
)
