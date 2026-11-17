package fakemarket

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// MaxBatch is the largest number of ids accepted by the batch endpoint.
const MaxBatch = 100

// Chaos configures the failure modes. The zero value means a well-behaved API.
type Chaos struct {
	// RateLimitRPS > 0 enables a token bucket; excess calls get 429 + Retry-After.
	RateLimitRPS float64 `json:"rate_limit_rps"`
	RateBurst    int     `json:"rate_burst"`
	// TransientErrorRate (0..1) answers that share of calls with 503.
	TransientErrorRate float64 `json:"transient_error_rate"`
	// PoisonIDs makes the batch endpoint reject any request containing one of
	// these ids, failing the whole batch (as some marketplace APIs do).
	PoisonIDs []string `json:"poison_ids"`
	// SchemaDriftRate (0..1) serializes that share of orders with
	// total_amount as a string instead of a number.
	SchemaDriftRate float64 `json:"schema_drift_rate"`
}

// Server is the fake marketplace HTTP API.
type Server struct {
	store *store

	mu      sync.RWMutex
	chaos   Chaos
	limiter *rate.Limiter
	rng     *rand.Rand
}

// NewServer creates a server whose shops are seeded with seedCount orders.
func NewServer(seedCount int, now func() time.Time) *Server {
	if now == nil {
		now = time.Now
	}
	return &Server{store: newStore(seedCount, now), rng: rand.New(rand.NewPCG(1, 2))}
}

// SetChaos replaces the chaos configuration.
func (s *Server) SetChaos(c Chaos) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chaos = c
	s.limiter = nil
	if c.RateLimitRPS > 0 {
		burst := max(c.RateBurst, 1)
		s.limiter = rate.NewLimiter(rate.Limit(c.RateLimitRPS), burst)
	}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	mux.HandleFunc("GET /v1/shops/{shop}/orders", s.chaotic(s.listOrders))
	mux.HandleFunc("GET /v1/shops/{shop}/orders/batch", s.chaotic(s.batchOrders))

	mux.HandleFunc("GET /admin/chaos", s.getChaos)
	mux.HandleFunc("PUT /admin/chaos", s.putChaos)
	mux.HandleFunc("POST /admin/shops/{shop}/delete", s.adminDelete)
	mux.HandleFunc("POST /admin/shops/{shop}/late-update", s.adminLateUpdate)
	return mux
}

// chaotic applies rate limiting and transient errors before the handler.
func (s *Server) chaotic(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		limiter, errRate := s.limiter, s.chaos.TransientErrorRate
		fail := errRate > 0 && s.rng.Float64() < errRate
		s.mu.Unlock()

		if limiter != nil && !limiter.Allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		if fail {
			writeError(w, http.StatusServiceUnavailable, "unavailable", "temporary failure, try again")
			return
		}
		next(w, r)
	}
}

type listResponse struct {
	Orders  []orderRef `json:"orders"`
	Page    int        `json:"page"`
	HasMore bool       `json:"has_more"`
	Total   int        `json:"total"`
}

type orderRef struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Server) listOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err1 := time.Parse(time.RFC3339Nano, q.Get("updated_from"))
	to, err2 := time.Parse(time.RFC3339Nano, q.Get("updated_to"))
	if err1 != nil || err2 != nil || !from.Before(to) {
		writeError(w, http.StatusBadRequest, "invalid_param", "updated_from and updated_to must be RFC3339 and from < to")
		return
	}
	page := atoiDefault(q.Get("page"), 1)
	size := atoiDefault(q.Get("page_size"), 50)
	if page < 1 || size < 1 || size > 1000 {
		writeError(w, http.StatusBadRequest, "invalid_param", "page >= 1 and 1 <= page_size <= 1000")
		return
	}

	all := s.store.list(r.PathValue("shop"), from, to)
	start := min((page-1)*size, len(all))
	end := min(start+size, len(all))

	resp := listResponse{Orders: make([]orderRef, 0, end-start), Page: page, HasMore: end < len(all), Total: len(all)}
	for _, o := range all[start:end] {
		resp.Orders = append(resp.Orders, orderRef{ID: o.ID, UpdatedAt: o.UpdatedAt})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) batchOrders(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		writeError(w, http.StatusBadRequest, "invalid_param", "ids is required")
		return
	}
	ids := strings.Split(raw, ",")
	if len(ids) > MaxBatch {
		writeError(w, http.StatusBadRequest, "invalid_param", fmt.Sprintf("at most %d ids per call", MaxBatch))
		return
	}

	s.mu.RLock()
	poison := s.chaos.PoisonIDs
	driftRate := s.chaos.SchemaDriftRate
	s.mu.RUnlock()

	var bad []string
	for _, id := range ids {
		if slices.Contains(poison, id) {
			bad = append(bad, id)
		}
	}
	if len(bad) > 0 {
		// Mirrors real APIs that fail the whole call and only name the bad
		// ids inside a free-text message.
		writeError(w, http.StatusBadRequest, "invalid_param", "has invalid order ids- "+strings.Join(bad, ",")+";")
		return
	}

	orders := s.store.get(r.PathValue("shop"), ids)
	out := make([]any, 0, len(orders))
	for _, o := range orders {
		if driftRate > 0 && s.drift(driftRate) {
			out = append(out, driftedOrder(o))
			continue
		}
		out = append(out, o)
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": out})
}

func (s *Server) drift(p float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < p
}

// driftedOrder renders total_amount as a string to simulate a schema change.
func driftedOrder(o Order) map[string]any {
	return map[string]any{
		"id":           o.ID,
		"shop_id":      o.ShopID,
		"status":       o.Status,
		"total_amount": strconv.FormatInt(o.TotalAmount, 10),
		"currency":     o.Currency,
		"created_at":   o.CreatedAt,
		"updated_at":   o.UpdatedAt,
	}
}

func (s *Server) getChaos(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, http.StatusOK, s.chaos)
}

func (s *Server) putChaos(w http.ResponseWriter, r *http.Request) {
	var c Chaos
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	s.SetChaos(c)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) adminDelete(w http.ResponseWriter, r *http.Request) {
	n := atoiDefault(r.URL.Query().Get("count"), 1)
	ids := s.store.deleteOpen(r.PathValue("shop"), n)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": ids})
}

func (s *Server) adminLateUpdate(w http.ResponseWriter, r *http.Request) {
	n := atoiDefault(r.URL.Query().Get("count"), 1)
	ids := s.store.lateUpdate(r.PathValue("shop"), n)
	writeJSON(w, http.StatusOK, map[string]any{"updated": ids})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": code, "message": msg})
}
