// Package mysqlstore implements ports.OrderStore and ports.QuarantineStore on
// MySQL 8 / Amazon Aurora MySQL. Schema: migrations/.
package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // driver

	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/ports"
)

var errNotImplemented = errors.New("not implemented")

type Store struct {
	db *sql.DB
}

var (
	_ ports.OrderStore      = (*Store)(nil)
	_ ports.QuarantineStore = (*Store)(nil)
)

// Open connects and verifies the connection. dsn must include parseTime=true.
func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Ping is used by health checks.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) UpsertOrders(ctx context.Context, orders []domain.Order) (ports.UpsertResult, error) {
	// TODO(core): one multi-row INSERT ... ON DUPLICATE KEY UPDATE per batch.
	// Only overwrite when VALUES(source_updated_at) > source_updated_at, so
	// at-least-once and out-of-order delivery never move a row backwards.
	// Note what affected-rows means for ODKU (1 = insert, 2 = update, 0 = no-op).
	return ports.UpsertResult{}, errNotImplemented
}

func (s *Store) ListOpenOrderIDs(ctx context.Context, tenantID, source, afterID string, limit int) ([]string, error) {
	// TODO(core): keyset pagination (external_id > ?) on idx_orders_open;
	// check the plan with EXPLAIN and record it in docs/queries.md.
	return nil, errNotImplemented
}

func (s *Store) MarkDeleted(ctx context.Context, keys []domain.Key, at time.Time) (int, error) {
	// TODO(core): set deleted_at for the keys that are still open; never
	// hard-delete (see docs/adr/0003-tombstones.md).
	return 0, errNotImplemented
}

func (s *Store) Quarantine(ctx context.Context, tenantID, source string, externalIDs []string, detail domain.ErrorDetail) error {
	// TODO(core): upsert into quarantine, bump attempts and last_seen_at.
	return errNotImplemented
}
