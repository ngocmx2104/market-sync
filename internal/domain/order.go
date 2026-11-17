// Package domain holds the core types of market-sync. It has no dependencies
// on transport, storage or vendor SDKs.
package domain

import "time"

// OrderStatus is the lifecycle state reported by the source marketplace.
type OrderStatus string

const (
	StatusNew       OrderStatus = "new"
	StatusConfirmed OrderStatus = "confirmed"
	StatusShipping  OrderStatus = "shipping"
	StatusCompleted OrderStatus = "completed"
	StatusCancelled OrderStatus = "cancelled"
	StatusReturned  OrderStatus = "returned"
)

// IsTerminal reports whether an order in this status can still change at the
// source. Non-terminal orders are the ones the reconciler keeps re-checking.
func (s OrderStatus) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusCancelled, StatusReturned:
		return true
	default:
		return false
	}
}

// Order is one marketplace order as stored by market-sync.
type Order struct {
	TenantID        string
	Source          string // e.g. "fake-market"
	ExternalID      string // id assigned by the source
	Status          OrderStatus
	TotalAmount     int64 // minor units
	Currency        string
	SourceCreatedAt time.Time
	SourceUpdatedAt time.Time
	// DeletedAt is set when the order disappeared at the source (tombstone).
	DeletedAt *time.Time
	Raw       []byte // original payload, kept for replay and debugging
}

// Key uniquely identifies an order across tenants and sources.
type Key struct {
	TenantID   string
	Source     string
	ExternalID string
}

func (o Order) Key() Key {
	return Key{TenantID: o.TenantID, Source: o.Source, ExternalID: o.ExternalID}
}
