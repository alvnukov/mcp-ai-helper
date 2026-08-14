package confluence

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// A rate-limited Confluence answers 429 before it answers content; the
// client must retry instead of surfacing the library's paraphrase of the
// status as a dead end.
func TestGetSpacesRetriesRateLimit(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results": [], "start": 0, "limit": 50, "size": 0}`))
	}))
	defer server.Close()

	client, err := NewClientWithHTTP(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	spaces, err := client.GetSpaces()
	if err != nil {
		t.Fatalf("429 then 200 must succeed: %v", err)
	}
	if len(spaces) != 0 {
		t.Fatalf("spaces = %v, want none", spaces)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("server calls = %d, want 2", got)
	}
}
