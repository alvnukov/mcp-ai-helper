package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// A rate-limited Jira answers 429 before it answers content; the client must
// retry instead of failing the tool call on the first refusal.
func TestGetIssuePropertyRetriesRateLimit(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(config.JiraConfig{URL: server.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := client.GetIssuePropertyContext(context.Background(), "PROJ-1", "checklist", &out); err != nil {
		t.Fatalf("429 then 200 must succeed: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("server calls = %d, want 2", got)
	}
}
