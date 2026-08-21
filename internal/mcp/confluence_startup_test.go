package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// The confluence tool is registered from the startup config, so the client
// behind it has to exist from process start too. It used to be built only by
// the config-reload path, so every call on a fresh server answered
// "confluence: not configured" until something reloaded the config.
func TestConfluenceToolsAnswerFromProcessStart(t *testing.T) {
	wiki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"results":[],"start":0,"limit":0,"size":0}`)); err != nil {
			t.Errorf("write test response: %v", err)
		}
	}))
	defer wiki.Close()

	enabled := true
	cfg := &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{
			URL:     wiki.URL,
			APIKey:  "test",
			Enabled: &enabled,
		},
	}}

	srv := New(cfg)
	tool, ok := srv.ListTools()["confluence"]
	if !ok {
		t.Fatal("confluence is not registered")
	}
	var req basemcp.CallToolRequest
	req.Params.Name = "confluence"
	req.Params.Arguments = map[string]any{"action": "spaces"}
	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("confluence spaces refused on a fresh server without a config reload: %s", resultText(t, result))
	}
	if got := resultText(t, result); !strings.Contains(got, `"total"`) {
		t.Errorf("confluence spaces returned %s", got)
	}
}
