package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestWebFetchToolReturnsBoundedMetadata(t *testing.T) {
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>full page body must stay cached</body></html>"))
	}))
	defer srvHTTP.Close()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance(), WebPolicy: config.WebPolicy{CacheDir: t.TempDir(), MaxSourceBytes: 1024, TimeoutSeconds: 2, MaxRedirects: 3, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, AcceptedContentTypes: []string{"text/html"}}}
	srv := New(cfg)
	st, ok := srv.ListTools()["web_fetch"]
	if !ok {
		t.Fatal("web_fetch tool is not registered")
	}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"url": srvHTTP.URL}
	res, err := st.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error")
	}
	m := resultMap(t, res)
	if m["status"] != "complete" || m["doc_id"] == "" {
		t.Fatalf("result = %#v", m)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "full page body") {
		t.Fatalf("tool response leaked page body: %s", data)
	}
}

func TestFetchURLAliasRegistered(t *testing.T) {
	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	if _, ok := srv.ListTools()["fetch_url"]; !ok {
		t.Fatal("fetch_url alias is not registered")
	}
}
