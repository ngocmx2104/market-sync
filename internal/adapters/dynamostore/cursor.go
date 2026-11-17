// Package dynamostore implements ports.CursorStore on DynamoDB.
//
// Table "sync_cursors": partition key "pk" = "<tenant>#<source>#<resource>".
// Attributes: watermark (RFC3339Nano string), updated_at, version (number).
package dynamostore

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/ports"
)

// ErrConflict means another writer advanced the cursor first.
var ErrConflict = errors.New("cursor version conflict")

var errNotImplemented = errors.New("not implemented")

type CursorStore struct {
	client *dynamodb.Client
	table  string
}

var _ ports.CursorStore = (*CursorStore)(nil)

func NewCursorStore(client *dynamodb.Client, table string) *CursorStore {
	return &CursorStore{client: client, table: table}
}

func (s *CursorStore) Get(ctx context.Context, tenantID, source, resource string) (domain.Cursor, bool, error) {
	// TODO(core): GetItem with ConsistentRead, map attributes to domain.Cursor.
	return domain.Cursor{}, false, errNotImplemented
}

func (s *CursorStore) Advance(ctx context.Context, c domain.Cursor, newWatermark time.Time) (domain.Cursor, error) {
	// TODO(core): PutItem/UpdateItem with a ConditionExpression on version
	// (attribute_not_exists(pk) for the first write), version + 1, and reject
	// newWatermark < c.Watermark. Map ConditionalCheckFailedException to
	// ErrConflict. See docs/adr/0001-storage-split.md.
	return domain.Cursor{}, errNotImplemented
}
