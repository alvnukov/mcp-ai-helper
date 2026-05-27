package webfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func testPolicy(t *testing.T) config.WebPolicy {
	t.Helper()
	return config.WebPolicy{CacheDir: t.TempDir(), MaxSourceBytes: 1024, TimeoutSeconds: 2, MaxRedirects: 3, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, AcceptedContentTypes: []string{"text/html", "text/plain"}}
}

func TestFetchStoresSourceLosslesslyAndReturnsMetadataOnly(t *testing.T) {
	raw := []byte("<html><head><title>ok</title></head><body>secret page body</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	policy := testPolicy(t)
	result, err := NewClient(policy).Fetch(context.Background(), FetchRequest{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.DocID == "" {
		t.Fatalf("result = %#v", result)
	}
	stored, err := os.ReadFile(filepath.Join(policy.CacheDir, result.DocID, "source.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(raw) {
		t.Fatalf("stored source = %q, want %q", stored, raw)
	}
	sum := sha256.Sum256(raw)
	if result.SourceSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("source hash = %s", result.SourceSHA256)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret page body") {
		t.Fatalf("metadata response leaked page body: %s", data)
	}
}

func TestFetchReportsCacheHitOnRepeat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("same"))
	}))
	defer srv.Close()
	client := NewClient(testPolicy(t))
	first, err := client.Fetch(context.Background(), FetchRequest{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Fetch(context.Background(), FetchRequest{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if first.DocID != second.DocID || second.Cache.Status != "hit" {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}

func TestFetchRedirectPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("final"))
	}))
	defer srv.Close()
	result, err := NewClient(testPolicy(t)).Fetch(context.Background(), FetchRequest{URL: srv.URL + "/redirect"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || !strings.HasSuffix(result.FinalURL, "/final") {
		t.Fatalf("result = %#v", result)
	}
}

func TestFetchDeniedProtocolFailsClosed(t *testing.T) {
	result, err := NewClient(testPolicy(t)).Fetch(context.Background(), FetchRequest{URL: "file:///etc/passwd"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "blocked" || len(result.Diagnostics) == 0 {
		t.Fatalf("result = %#v", result)
	}
}

func TestFetchSizeLimitIsIncomplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer srv.Close()
	policy := testPolicy(t)
	policy.MaxSourceBytes = 4
	result, err := NewClient(policy).Fetch(context.Background(), FetchRequest{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "incomplete" || result.DocID != "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestFetchContentTypeDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer srv.Close()
	result, err := NewClient(testPolicy(t)).Fetch(context.Background(), FetchRequest{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "blocked" || len(result.Diagnostics) == 0 {
		t.Fatalf("result = %#v", result)
	}
}
