package marketclient

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/fakemarket"
)

func newClient(t *testing.T, chaos fakemarket.Chaos) *Client {
	t.Helper()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	srv := fakemarket.NewServer(20, func() time.Time { return now })
	srv.SetChaos(chaos)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return New(ts.URL, ts.Client())
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		chaos     fakemarket.Chaos
		ids       []string
		wantCode  domain.ErrorCode
		wantRetry bool
		wantIDs   []string
	}{
		{
			name:     "poison id names the bad ids",
			chaos:    fakemarket.Chaos{PoisonIDs: []string{"s1-0000002"}},
			ids:      []string{"s1-0000001", "s1-0000002"},
			wantCode: domain.CodePoisonRecord,
			wantIDs:  []string{"s1-0000002"},
		},
		{
			name:      "5xx is transient",
			chaos:     fakemarket.Chaos{TransientErrorRate: 1},
			ids:       []string{"s1-0000001"},
			wantCode:  domain.CodeTransient,
			wantRetry: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t, tt.chaos)
			_, _, err := c.GetOrders(context.Background(), "s1", tt.ids)
			d, ok := domain.AsErrorDetail(err)
			if !ok {
				t.Fatalf("err = %v, want *domain.ErrorDetail", err)
			}
			if d.Code != tt.wantCode || d.Retryable != tt.wantRetry || !slices.Equal(d.ObjectIDs, tt.wantIDs) {
				t.Fatalf("got %+v", d)
			}
		})
	}
}

func TestSchemaDriftIsRejectedPerRecord(t *testing.T) {
	c := newClient(t, fakemarket.Chaos{SchemaDriftRate: 1})
	orders, rejected, err := c.GetOrders(context.Background(), "s1", []string{"s1-0000001", "s1-0000002"})
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 0 || len(rejected) != 2 {
		t.Fatalf("orders=%d rejected=%d, want 0/2", len(orders), len(rejected))
	}
	if rejected[0].Detail.Code != domain.CodeInvalidRecord || rejected[0].ExternalID == "" {
		t.Fatalf("rejected[0] = %+v", rejected[0])
	}
}
