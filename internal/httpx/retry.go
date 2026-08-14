// Package httpx provides HTTP transports shared by the integration clients.
package httpx

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Transport retries the responses a rate-limited or briefly broken API
// returns: 429 for any method, because the server refused the request
// without executing it, and 502/503/504 for idempotent methods only,
// because a 5xx leaves whether the request ran ambiguous. A request whose
// body cannot be replayed is returned as received instead of retried.
type Transport struct {
	// Base is the wrapped transport; nil means http.DefaultTransport.
	Base http.RoundTripper
	// MaxAttempts bounds the total attempts including the first. Below 1 means 1.
	MaxAttempts int
	// Backoff computes the wait before a retry from the 1-based attempt and
	// the refused response. nil means Retry-After when present, else one
	// second per attempt.
	Backoff func(attempt int, resp *http.Response) time.Duration
	// MaxBackoff caps one wait so a hostile Retry-After cannot pin the call.
	// Zero or below means 30s.
	MaxBackoff time.Duration
}

// NewTransport wraps base with the shared retry policy.
func NewTransport(base http.RoundTripper) *Transport {
	return &Transport{Base: base, MaxAttempts: 3, MaxBackoff: 30 * time.Second}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	maxAttempts := max(t.MaxAttempts, 1)
	backoff := t.Backoff
	if backoff == nil {
		backoff = retryAfter
	}
	maxBackoff := t.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}

	send := req
	for attempt := 1; ; attempt++ {
		resp, err := base.RoundTrip(send)
		if err != nil {
			return nil, err
		}
		if !retryable(resp.StatusCode, send.Method) {
			return resp, nil
		}
		if attempt >= maxAttempts {
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			// Exhausted attempts surface as an error that names them, rather
			// than the raw status the caller's library would paraphrase.
			return nil, fmt.Errorf("httpx: giving up after %d attempt(s): last response %s", maxAttempts, resp.Status)
		}
		if req.Body != nil && req.GetBody == nil {
			// The body cannot be replayed; a retry would send it empty.
			return resp, nil
		}
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		wait := backoff(attempt, resp)
		if wait > maxBackoff {
			wait = maxBackoff
		}
		if wait < 0 {
			wait = 0
		}
		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}
		}
		send = req.Clone(req.Context())
		if req.Body != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return nil, fmt.Errorf("httpx: replay request body: %w", bodyErr)
			}
			send.Body = body
		}
	}
}

func retryable(status int, method string) bool {
	switch status {
	case http.StatusTooManyRequests:
		return true
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return method == http.MethodGet || method == http.MethodHead
	}
	return false
}

// retryAfter reads the Retry-After a rate-limiting server sent — seconds or
// an HTTP-date — and falls back to one second per attempt when it did not.
func retryAfter(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if v := resp.Header.Get("Retry-After"); v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
				return time.Duration(secs) * time.Second
			}
			if at, err := http.ParseTime(v); err == nil {
				if d := time.Until(at); d > 0 {
					return d
				}
				return 0
			}
		}
	}
	return time.Duration(attempt) * time.Second
}
