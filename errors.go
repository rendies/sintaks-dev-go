package sintaksdev

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIError is an error returned by the gateway.
type APIError struct {
	// StatusCode is the HTTP status, or 0 for an error delivered mid-stream.
	StatusCode int
	Type       string
	Code       string
	Message    string
	// RequestID is the gateway's X-Request-ID; quote it when reporting a
	// failed request, e.g. for a refund.
	RequestID string
}

func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("sintaks: ")
	if e.StatusCode != 0 {
		fmt.Fprintf(&b, "%d ", e.StatusCode)
	}
	if e.Type != "" {
		b.WriteString(e.Type + ": ")
	}
	b.WriteString(e.Message)
	if e.RequestID != "" {
		b.WriteString(" (request_id " + e.RequestID + ")")
	}
	return b.String()
}

type errorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Code    *string `json:"code"`
}

func (b *errorBody) toAPIError(status int, requestID string) *APIError {
	e := &APIError{StatusCode: status, Type: b.Type, Message: b.Message, RequestID: requestID}
	if b.Code != nil {
		e.Code = *b.Code
	}
	return e
}

func parseAPIError(resp *http.Response) *APIError {
	id := resp.Header.Get("X-Request-ID")
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var env struct {
		Error *errorBody `json:"error"`
	}
	if json.Unmarshal(raw, &env) == nil && env.Error != nil {
		return env.Error.toAPIError(resp.StatusCode, id)
	}
	msg := strings.TrimSpace(string(raw))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return &APIError{StatusCode: resp.StatusCode, Message: msg, RequestID: id}
}

func statusOf(err error) int {
	var e *APIError
	if errors.As(err, &e) {
		return e.StatusCode
	}
	return 0
}

// IsUnauthorized reports a missing, invalid or revoked API key (401).
func IsUnauthorized(err error) bool { return statusOf(err) == http.StatusUnauthorized }

// IsInsufficientMicrocredits reports that the account balance can't cover the
// request (402). Add balance before retrying.
func IsInsufficientMicrocredits(err error) bool { return statusOf(err) == http.StatusPaymentRequired }

// IsInsufficientCredits is kept as a compatibility alias for existing SDK
// consumers. New code should use IsInsufficientMicrocredits.
func IsInsufficientCredits(err error) bool { return IsInsufficientMicrocredits(err) }

// IsInvalidRequest reports a request rejected as invalid (400).
func IsInvalidRequest(err error) bool { return statusOf(err) == http.StatusBadRequest }

// IsRateLimited reports upstream throttling (429).
func IsRateLimited(err error) bool { return statusOf(err) == http.StatusTooManyRequests }

// IsRetryable reports errors worth retrying with backoff: throttling and
// temporary upstream unavailability or timeouts (429, 503, 504).
func IsRetryable(err error) bool {
	switch statusOf(err) {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}
