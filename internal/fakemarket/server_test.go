package fakemarket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

func newTestServer(t *testing.T, seed int) (*Server, *httptest.Server) {
	t.Helper()
	s := NewServer(seed, func() time.Time { return fixedNow })
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func get(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func listAll(t *testing.T, base string) listResponse {
	t.Helper()
	q := url.Values{}
	q.Set("updated_from", fixedNow.Add(-365*24*time.Hour).Format(time.RFC3339))
	q.Set("updated_to", fixedNow.Add(time.Hour).Format(time.RFC3339))
	q.Set("page_size", "1000")
	resp, err := get(t, base+"/v1/shops/s1/orders?"+q.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatal(err)
	}
	return lr
}

func TestListIsSortedAndComplete(t *testing.T) {
	_, ts := newTestServer(t, 200)
	lr := listAll(t, ts.URL)

	if lr.Total != 200 || len(lr.Orders) != 200 || lr.HasMore {
		t.Fatalf("total=%d len=%d hasMore=%t, want 200/200/false", lr.Total, len(lr.Orders), lr.HasMore)
	}
	for i := 1; i < len(lr.Orders); i++ {
		if lr.Orders[i].UpdatedAt.Before(lr.Orders[i-1].UpdatedAt) {
			t.Fatalf("not sorted at %d", i)
		}
	}
}

func TestBatchRejectsWholeCallOnPoisonID(t *testing.T) {
	s, ts := newTestServer(t, 10)
	s.SetChaos(Chaos{PoisonIDs: []string{"s1-0000003"}})

	resp, err := get(t, ts.URL+"/v1/shops/s1/orders/batch?ids=s1-0000001,s1-0000003")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !strings.Contains(body["message"], "s1-0000003") {
		t.Fatalf("message %q does not name the poison id", body["message"])
	}
}

func TestLateUpdateIsHiddenFromListButVisibleByID(t *testing.T) {
	s, ts := newTestServer(t, 50)
	ids := s.store.lateUpdate("s1", 1)
	if len(ids) != 1 {
		t.Fatalf("lateUpdate returned %v", ids)
	}

	for _, o := range listAll(t, ts.URL).Orders {
		if o.ID == ids[0] {
			t.Fatalf("late-updated order %s should be hidden from list", ids[0])
		}
	}
	if got := s.store.get("s1", ids); len(got) != 1 || got[0].Status != "completed" {
		t.Fatalf("get by id = %+v, want one completed order", got)
	}
}

func TestRateLimitReturns429(t *testing.T) {
	s, ts := newTestServer(t, 1)
	s.SetChaos(Chaos{RateLimitRPS: 0.001, RateBurst: 1})

	codes := map[int]int{}
	for range 3 {
		resp, err := get(t, ts.URL+"/v1/shops/s1/orders/batch?ids=s1-0000001")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		codes[resp.StatusCode]++
	}
	if codes[http.StatusOK] != 1 || codes[http.StatusTooManyRequests] != 2 {
		t.Fatalf("codes = %v, want 1x200 and 2x429", codes)
	}
}
