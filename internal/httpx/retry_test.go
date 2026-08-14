package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type recordingTransport struct {
	statuses []int
	headers  http.Header
	bodies   []string
	calls    int
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	attempt := r.calls
	r.calls++
	status := http.StatusOK
	if len(r.statuses) > 0 {
		status = r.statuses[min(attempt, len(r.statuses)-1)]
	}
	body := ""
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body)
		body = string(raw)
	}
	r.bodies = append(r.bodies, body)
	header := r.headers.Clone()
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func zeroBackoff(int, *http.Response) time.Duration { return 0 }

func newTestRequest(method, url, body string) (*http.Request, error) {
	if body == "" {
		return http.NewRequest(method, url, nil)
	}
	return http.NewRequest(method, url, strings.NewReader(body))
}

// 429 means the server refused the request without executing it, so even a
// POST is safe to send again — with its body replayed intact.
func TestTransportRetriesRateLimitOnAnyMethod(t *testing.T) {
	stub := &recordingTransport{statuses: []int{http.StatusTooManyRequests, http.StatusOK}}
	transport := &Transport{Base: stub, MaxAttempts: 3, Backoff: zeroBackoff}
	req, err := newTestRequest(http.MethodPost, "https://api.example.com/x", `{"a":1}`)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		t.Fatalf("429 then 200 must succeed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if stub.calls != 2 {
		t.Fatalf("calls = %d, want 2", stub.calls)
	}
	if stub.bodies[1] != stub.bodies[0] || stub.bodies[0] == "" {
		t.Fatalf("retry must replay the body: %q then %q", stub.bodies[0], stub.bodies[1])
	}
}

func TestTransportGivesUpWithNamedStatus(t *testing.T) {
	stub := &recordingTransport{statuses: []int{http.StatusTooManyRequests}}
	transport := &Transport{Base: stub, MaxAttempts: 2, Backoff: zeroBackoff}
	req, _ := newTestRequest(http.MethodGet, "https://api.example.com/x", "")
	resp, err := transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err == nil || !strings.Contains(err.Error(), "2 attempt") || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err = %v, want attempts and status named", err)
	}
	if stub.calls != 2 {
		t.Fatalf("calls = %d, want 2", stub.calls)
	}
}

func TestTransportRetriesServerErrorsOnlyForIdempotentMethods(t *testing.T) {
	post := &recordingTransport{statuses: []int{http.StatusServiceUnavailable}}
	transport := &Transport{Base: post, MaxAttempts: 3, Backoff: zeroBackoff}
	req, _ := newTestRequest(http.MethodPost, "https://api.example.com/x", "")
	resp, err := transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	if post.calls != 1 || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("POST 503 must not retry: calls = %d, status = %d", post.calls, resp.StatusCode)
	}

	get := &recordingTransport{statuses: []int{http.StatusServiceUnavailable, http.StatusOK}}
	transport = &Transport{Base: get, MaxAttempts: 3, Backoff: zeroBackoff}
	req, _ = newTestRequest(http.MethodGet, "https://api.example.com/x", "")
	resp, err = transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	if get.calls != 2 || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET 503 must retry: calls = %d, status = %d", get.calls, resp.StatusCode)
	}
}

func TestTransportDoesNotRetryUnreplayableBody(t *testing.T) {
	stub := &recordingTransport{statuses: []int{http.StatusTooManyRequests, http.StatusOK}}
	transport := &Transport{Base: stub, MaxAttempts: 3, Backoff: zeroBackoff}
	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/x", io.NopCloser(strings.NewReader("body")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("a body without GetBody must not retry: calls = %d, status = %d", stub.calls, resp.StatusCode)
	}
}

func TestRetryAfterParsesHeader(t *testing.T) {
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"7"}}}
	if got := retryAfter(1, resp); got != 7*time.Second {
		t.Fatalf("seconds form: got %v", got)
	}
	resp.Header.Set("Retry-After", time.Now().UTC().Add(20*time.Second).Format(http.TimeFormat))
	if got := retryAfter(1, resp); got <= 15*time.Second || got > 20*time.Second {
		t.Fatalf("http-date form: got %v", got)
	}
	resp.Header.Del("Retry-After")
	if got := retryAfter(2, resp); got != 2*time.Second {
		t.Fatalf("fallback: got %v", got)
	}
}

func TestTransportCapsBackoff(t *testing.T) {
	stub := &recordingTransport{
		statuses: []int{http.StatusTooManyRequests, http.StatusOK},
		headers:  http.Header{"Retry-After": []string{"60"}},
	}
	transport := &Transport{Base: stub, MaxAttempts: 3, MaxBackoff: 20 * time.Millisecond}
	req, _ := newTestRequest(http.MethodGet, "https://api.example.com/x", "")
	start := time.Now()
	resp, err := transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if waited := time.Since(start); waited > time.Second {
		t.Fatalf("MaxBackoff ignored: waited %v", waited)
	}
}

func TestTransportCancelsWaitWithContext(t *testing.T) {
	stub := &recordingTransport{statuses: []int{http.StatusTooManyRequests, http.StatusOK}}
	transport := &Transport{Base: stub, MaxAttempts: 3, Backoff: func(int, *http.Response) time.Duration { return time.Minute }}
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	resp, err := transport.RoundTrip(req)
	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()
	if err == nil {
		t.Fatal("canceled context must end the wait")
	}
}
