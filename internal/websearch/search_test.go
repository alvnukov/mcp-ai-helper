package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestSearchParsesCompactDuckDuckGoHTMLHits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "bounded fetch" {
			t.Fatalf("query = %q", got)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
<div class="result"><a class="result__a" href="/l/?uddg=https%3A%2F%2Fexample.com%2Falpha%3Fx%3D1">Alpha <b>Result</b></a><a class="result__snippet">First compact snippet.</a></div>
<div class="result"><a class="result__a" href="https://example.org/beta">Beta result</a><a class="result__snippet">Second compact snippet.</a></div>
</body></html>`))
	}))
	defer srv.Close()

	policy := config.WebPolicy{SearchProvider: ProviderDuckDuckGoHTML, SearchURL: srv.URL, TimeoutSeconds: 2, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, UserAgent: "test-agent", MaxSearchResults: 5}
	result := Search(context.Background(), policy, Request{Query: "bounded fetch", MaxResults: 1})
	if result.Status != "complete" || result.Total != 2 || len(result.Hits) != 1 || !result.Truncated {
		t.Fatalf("result = %#v", result)
	}
	hit := result.Hits[0]
	if hit.Title != "Alpha Result" || hit.URL != "https://example.com/alpha?x=1" || hit.Snippet != "First compact snippet." || hit.Rank != 1 || hit.FetchedHint != "not_fetched" {
		t.Fatalf("hit = %#v", hit)
	}
}

func TestSearchAcceptsExplicitProviderArgument(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<a class="result__a" href="https://example.com/">Example</a>`))
	}))
	defer srv.Close()

	policy := config.WebPolicy{SearchURL: srv.URL, TimeoutSeconds: 2, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, MaxSearchResults: 5}
	result := Search(context.Background(), policy, Request{Query: "example", Provider: ProviderDuckDuckGoHTML})
	if result.Status != "complete" || len(result.Hits) != 1 {
		t.Fatalf("result = %#v", result)
	}
}

func TestSearchFailsClosedWithoutProvider(t *testing.T) {
	result := Search(context.Background(), config.WebPolicy{}, Request{Query: "bounded fetch"})
	if result.Status != "blocked" || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != "search_provider_not_configured" {
		t.Fatalf("result = %#v", result)
	}
}

func TestSearchFailsClosedForUnsupportedProvider(t *testing.T) {
	result := Search(context.Background(), config.WebPolicy{SearchProvider: "other"}, Request{Query: "bounded fetch"})
	if result.Status != "blocked" || len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != "unsupported_search_provider" {
		t.Fatalf("result = %#v", result)
	}
}
