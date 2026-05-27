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
	"github.com/zol/mcp-ai-helper/internal/websearch"
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

func TestFetchedDocReadAndFindToolsReturnBoundedFragments(t *testing.T) {
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("alpha needle beta needle gamma needle delta"))
	}))
	defer srvHTTP.Close()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance(), WebPolicy: config.WebPolicy{CacheDir: t.TempDir(), MaxSourceBytes: 1024, TimeoutSeconds: 2, MaxRedirects: 3, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, AcceptedContentTypes: []string{"text/plain"}}}
	srv := New(cfg)
	fetch := srv.ListTools()["web_fetch"].Handler
	fetchReq := basemcp.CallToolRequest{}
	fetchReq.Params.Arguments = map[string]any{"url": srvHTTP.URL}
	fetchRes, err := fetch(context.Background(), fetchReq)
	if err != nil {
		t.Fatal(err)
	}
	docID := resultMap(t, fetchRes)["doc_id"].(string)
	readReq := basemcp.CallToolRequest{}
	readReq.Params.Arguments = map[string]any{"doc_id": docID, "offset": 0, "limit": 12}
	readRes, err := srv.ListTools()["fetched_doc_read"].Handler(context.Background(), readReq)
	if err != nil {
		t.Fatal(err)
	}
	readMap := resultMap(t, readRes)
	if readMap["content"] != "alpha needle" || readMap["truncated"] != true {
		t.Fatalf("read = %#v", readMap)
	}
	findReq := basemcp.CallToolRequest{}
	findReq.Params.Arguments = map[string]any{"doc_id": docID, "query": "needle", "max_results": 1, "context_chars": 2}
	findRes, err := srv.ListTools()["fetched_doc_find"].Handler(context.Background(), findReq)
	if err != nil {
		t.Fatal(err)
	}
	findMap := resultMap(t, findRes)
	if findMap["truncated"] != true || len(findMap["matches"].([]any)) != 1 {
		t.Fatalf("find = %#v", findMap)
	}
}

func TestWebSearchReturnsCompactHits(t *testing.T) {
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "bounded fetch" {
			t.Fatalf("query = %q", got)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div class="result"><a class="result__a" href="/l/?uddg=https%3A%2F%2Fexample.com%2Falpha">Alpha result</a><a class="result__snippet">compact snippet</a></div><script>raw search markup must not leak</script></body></html>`))
	}))
	defer srvHTTP.Close()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance(), WebPolicy: config.WebPolicy{SearchProvider: websearch.ProviderDuckDuckGoHTML, SearchURL: srvHTTP.URL, TimeoutSeconds: 2, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, UserAgent: "test-agent", MaxSearchResults: 5}}
	srv := New(cfg)
	st, ok := srv.ListTools()["web_search"]
	if !ok {
		t.Fatal("web_search tool is not registered")
	}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "bounded fetch"}
	res, err := st.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := resultMap(t, res)
	if m["status"] != "complete" || len(m["hits"].([]any)) != 1 {
		t.Fatalf("result = %#v", m)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "raw search markup") || !strings.Contains(text, "compact snippet") {
		t.Fatalf("unexpected search response: %s", text)
	}
}

func TestWebSearchGoogleProviderThroughMCP(t *testing.T) {
	srvHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cx") != "engine-id" || r.URL.Query().Get("key") != "secret-key" {
			t.Fatalf("unexpected google query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"searchInformation":{"totalResults":"1"},"items":[{"title":"Google result","link":"https://example.com/google","displayLink":"example.com","snippet":"google snippet"}]}`))
	}))
	defer srvHTTP.Close()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance(), WebPolicy: config.WebPolicy{GoogleCSEURL: srvHTTP.URL, GoogleCSEID: "engine-id", GoogleAPIKey: "secret-key", TimeoutSeconds: 2, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, MaxSearchResults: 5}}
	srv := New(cfg)
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "bounded fetch", "provider": websearch.ProviderGoogleCSE}
	res, err := srv.ListTools()["web_search"].Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := resultMap(t, res)
	if m["status"] != "complete" || len(m["hits"].([]any)) != 1 {
		t.Fatalf("result = %#v", m)
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-key") || !strings.Contains(string(data), "google snippet") {
		t.Fatalf("unexpected google response: %s", data)
	}
}

func TestWebSearchFailsClosedWithoutProvider(t *testing.T) {
	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	st, ok := srv.ListTools()["web_search"]
	if !ok {
		t.Fatal("web_search tool is not registered")
	}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "bounded fetch"}
	res, err := st.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := resultMap(t, res)
	if m["status"] != "blocked" || len(m["diagnostics"].([]any)) == 0 {
		t.Fatalf("result = %#v", m)
	}
}
