// Package fakemarket is a fake marketplace API used for local runs and
// integration tests. It reproduces the failure modes market-sync must
// survive: rate limits, transient errors, poison ids, orders deleted at the
// source, updates the updated_at filter misses, and schema drift.
package fakemarket

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sort"
	"sync"
	"time"
)

// Order is the source-side representation (wire format, not domain).
type Order struct {
	ID          string    `json:"id"`
	ShopID      string    `json:"shop_id"`
	Status      string    `json:"status"`
	TotalAmount int64     `json:"total_amount"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// hiddenFromList simulates a source whose updated_at filter misses some
	// updates (bulk updates, late writes): the order is only visible by id.
	hiddenFromList bool
}

var statuses = []string{"new", "confirmed", "shipping", "completed", "cancelled", "returned"}

func isTerminal(s string) bool { return s == "completed" || s == "cancelled" || s == "returned" }

// store keeps orders per shop in memory. Shops are seeded lazily and
// deterministically, so the same shop always starts with the same data.
type store struct {
	mu        sync.RWMutex
	shops     map[string]map[string]*Order
	seedCount int
	now       func() time.Time
}

func newStore(seedCount int, now func() time.Time) *store {
	return &store{shops: map[string]map[string]*Order{}, seedCount: seedCount, now: now}
}

func (s *store) shop(id string) map[string]*Order {
	s.mu.RLock()
	orders, ok := s.shops[id]
	s.mu.RUnlock()
	if ok {
		return orders
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if orders, ok := s.shops[id]; ok {
		return orders
	}
	orders = seed(id, s.seedCount, s.now())
	s.shops[id] = orders
	return orders
}

func seed(shopID string, n int, now time.Time) map[string]*Order {
	h := fnv.New64a()
	_, _ = h.Write([]byte(shopID))
	rng := rand.New(rand.NewPCG(h.Sum64(), 42))

	orders := make(map[string]*Order, n)
	for i := 0; i < n; i++ {
		created := now.Add(-time.Duration(rng.Int64N(int64(90 * 24 * time.Hour))))
		updated := created.Add(time.Duration(rng.Int64N(int64(72 * time.Hour))))
		if updated.After(now) {
			updated = now
		}
		id := fmt.Sprintf("%s-%07d", shopID, i+1)
		orders[id] = &Order{
			ID:          id,
			ShopID:      shopID,
			Status:      statuses[rng.IntN(len(statuses))],
			TotalAmount: 10_000 + rng.Int64N(5_000_000),
			Currency:    "VND",
			CreatedAt:   created.UTC().Truncate(time.Millisecond),
			UpdatedAt:   updated.UTC().Truncate(time.Millisecond),
		}
	}
	return orders
}

// list returns orders with updated_at in [from, to), sorted by (updated_at, id).
func (s *store) list(shopID string, from, to time.Time) []Order {
	orders := s.shop(shopID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Order, 0, 64)
	for _, o := range orders {
		if o.hiddenFromList {
			continue
		}
		if !o.UpdatedAt.Before(from) && o.UpdatedAt.Before(to) {
			out = append(out, *o)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].UpdatedAt.Before(out[j].UpdatedAt)
	})
	return out
}

func (s *store) get(shopID string, ids []string) []Order {
	orders := s.shop(shopID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Order, 0, len(ids))
	for _, id := range ids {
		if o, ok := orders[id]; ok {
			out = append(out, *o)
		}
	}
	return out
}

// deleteOpen removes up to n non-terminal orders and returns their ids.
func (s *store) deleteOpen(shopID string, n int) []string {
	orders := s.shop(shopID)
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted []string
	for _, id := range sortedKeys(orders) {
		if len(deleted) == n {
			break
		}
		if !isTerminal(orders[id].Status) {
			delete(orders, id)
			deleted = append(deleted, id)
		}
	}
	return deleted
}

// lateUpdate moves up to n non-terminal orders to "completed" with a fresh
// updated_at, but hides them from list results: only a lookup by id sees it.
func (s *store) lateUpdate(shopID string, n int) []string {
	orders := s.shop(shopID)
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC().Truncate(time.Millisecond)
	var updated []string
	for _, id := range sortedKeys(orders) {
		if len(updated) == n {
			break
		}
		o := orders[id]
		if !isTerminal(o.Status) {
			o.Status = "completed"
			o.UpdatedAt = now
			o.hiddenFromList = true
			updated = append(updated, id)
		}
	}
	return updated
}

func sortedKeys(m map[string]*Order) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
