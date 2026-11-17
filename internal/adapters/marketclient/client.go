// Package marketclient implements ports.Source over the fake-market HTTP API.
package marketclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ngocmx2104/market-sync/internal/domain"
	"github.com/ngocmx2104/market-sync/internal/ports"
)

const sourceName = "fake-market"

// Client calls the fake marketplace. Tenants map 1:1 to shops.
type Client struct {
	baseURL string
	http    *http.Client
}

var _ ports.Source = (*Client)(nil)

func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

func (c *Client) Name() string  { return sourceName }
func (c *Client) MaxBatch() int { return 100 }

type listResponse struct {
	Orders []struct {
		ID        string    `json:"id"`
		UpdatedAt time.Time `json:"updated_at"`
	} `json:"orders"`
	Page    int  `json:"page"`
	HasMore bool `json:"has_more"`
	Total   int  `json:"total"`
}

func (c *Client) ListOrders(ctx context.Context, tenantID string, w domain.Window, page, pageSize int) (ports.Page, error) {
	q := url.Values{}
	q.Set("updated_from", w.From.UTC().Format(time.RFC3339Nano))
	q.Set("updated_to", w.To.UTC().Format(time.RFC3339Nano))
	q.Set("page", fmt.Sprint(page))
	q.Set("page_size", fmt.Sprint(pageSize))

	var lr listResponse
	if err := c.get(ctx, "list_orders", "/v1/shops/"+url.PathEscape(tenantID)+"/orders?"+q.Encode(), &lr); err != nil {
		return ports.Page{}, err
	}
	p := ports.Page{Page: lr.Page, HasMore: lr.HasMore, Total: lr.Total, Refs: make([]ports.OrderRef, 0, len(lr.Orders))}
	for _, o := range lr.Orders {
		p.Refs = append(p.Refs, ports.OrderRef{ExternalID: o.ID, UpdatedAt: o.UpdatedAt})
	}
	return p, nil
}

func (c *Client) GetOrders(ctx context.Context, tenantID string, ids []string) ([]domain.Order, []ports.Rejected, error) {
	if len(ids) == 0 {
		return nil, nil, nil
	}
	if len(ids) > c.MaxBatch() {
		return nil, nil, fmt.Errorf("marketclient: %d ids exceeds max batch %d", len(ids), c.MaxBatch())
	}

	var br struct {
		Orders []json.RawMessage `json:"orders"`
	}
	path := "/v1/shops/" + url.PathEscape(tenantID) + "/orders/batch?ids=" + url.QueryEscape(strings.Join(ids, ","))
	if err := c.get(ctx, "batch_get", path, &br); err != nil {
		return nil, nil, err
	}

	orders := make([]domain.Order, 0, len(br.Orders))
	var rejected []ports.Rejected
	for _, raw := range br.Orders {
		o, err := decodeOrder(tenantID, raw)
		if err != nil {
			rejected = append(rejected, ports.Rejected{
				ExternalID: peekID(raw),
				Raw:        raw,
				Detail:     domain.ErrorDetail{Code: domain.CodeInvalidRecord, Step: "decode", Message: err.Error()},
			})
			continue
		}
		orders = append(orders, o)
	}
	return orders, rejected, nil
}

type wireOrder struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	TotalAmount int64     `json:"total_amount"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// decodeOrder maps the wire format to the domain, strictly.
// TODO(feature 6): add a normalization layer (e.g. numeric strings) driven by
// an explicit contract, and count rejections as a metric.
func decodeOrder(tenantID string, raw json.RawMessage) (domain.Order, error) {
	var w wireOrder
	if err := json.Unmarshal(raw, &w); err != nil {
		return domain.Order{}, err
	}
	if w.ID == "" || w.UpdatedAt.IsZero() {
		return domain.Order{}, fmt.Errorf("missing id or updated_at")
	}
	return domain.Order{
		TenantID:        tenantID,
		Source:          sourceName,
		ExternalID:      w.ID,
		Status:          domain.OrderStatus(w.Status),
		TotalAmount:     w.TotalAmount,
		Currency:        w.Currency,
		SourceCreatedAt: w.CreatedAt,
		SourceUpdatedAt: w.UpdatedAt,
		Raw:             raw,
	}, nil
}

func peekID(raw json.RawMessage) string {
	var v struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &v)
	return v.ID
}

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// poisonIDs extracts ids from messages like "has invalid order ids- a,b;".
var poisonIDs = regexp.MustCompile(`invalid order ids-\s*([^;]+);`)

func (c *Client) get(ctx context.Context, step, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return &domain.ErrorDetail{Code: domain.CodeTransient, Step: step, Retryable: true, Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return json.NewDecoder(resp.Body).Decode(out)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var ae apiError
	_ = json.Unmarshal(body, &ae)
	detail := &domain.ErrorDetail{Step: step, Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, ae.Message)}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		detail.Code, detail.Retryable = domain.CodeRateLimited, true
	case resp.StatusCode >= 500:
		detail.Code, detail.Retryable = domain.CodeTransient, true
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		detail.Code = domain.CodeAuth
	case resp.StatusCode == http.StatusBadRequest && poisonIDs.MatchString(ae.Message):
		detail.Code = domain.CodePoisonRecord
		for _, id := range strings.Split(poisonIDs.FindStringSubmatch(ae.Message)[1], ",") {
			if id = strings.TrimSpace(id); id != "" {
				detail.ObjectIDs = append(detail.ObjectIDs, id)
			}
		}
	default:
		detail.Code = domain.CodeInternal
	}
	return detail
}
