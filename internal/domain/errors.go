package domain

import (
	"errors"
	"fmt"
)

// ErrorCode classifies a failure so operators and retry logic can act on it.
type ErrorCode string

const (
	CodeRateLimited   ErrorCode = "rate_limited"   // retryable after a delay
	CodeTransient     ErrorCode = "transient"      // 5xx, timeouts: retryable
	CodePoisonRecord  ErrorCode = "poison_record"  // the source rejects specific ids
	CodeInvalidRecord ErrorCode = "invalid_record" // payload breaks the contract
	CodeAuth          ErrorCode = "auth"           // credential problem: not retryable
	CodeInternal      ErrorCode = "internal"
)

// ErrorDetail is the structured error stored on sync runs and quarantined
// records, instead of a truncated error string.
type ErrorDetail struct {
	Code      ErrorCode `json:"code"`
	Step      string    `json:"step"`                 // e.g. "list_orders", "batch_get", "upsert"
	ObjectIDs []string  `json:"object_ids,omitempty"` // ids involved, if known
	Retryable bool      `json:"retryable"`
	Attempts  int       `json:"attempts"`
	Message   string    `json:"message"`
}

func (e *ErrorDetail) Error() string {
	return fmt.Sprintf("%s at %s (retryable=%t, attempts=%d): %s", e.Code, e.Step, e.Retryable, e.Attempts, e.Message)
}

// AsErrorDetail extracts an *ErrorDetail from an error chain.
func AsErrorDetail(err error) (*ErrorDetail, bool) {
	var d *ErrorDetail
	ok := errors.As(err, &d)
	return d, ok
}
