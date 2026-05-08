package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestCompletionURLsIncludeGenericProviderFallbacks(t *testing.T) {
	urls := completionURLs(config.ProviderConfig{BaseURL: "https://api.example.com/api/openai/v1"})
	want := []string{
		"https://api.example.com/api/openai/v1/chat/completions",
		"https://api.example.com/api/v1/chat/completions",
		"https://api.example.com/api/openai/chat/completions",
	}
	if len(urls) != len(want) {
		t.Fatalf("urls = %#v, want %#v", urls, want)
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Fatalf("urls[%d] = %q, want %q", i, urls[i], want[i])
		}
	}
}

func TestOpenAICompatibleClientComplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Fatalf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok [E1]"}},
			},
		})
	}))
	defer server.Close()

	client := NewOpenAICompatibleClient(map[string]config.ProviderConfig{
		"test": {BaseURL: server.URL + "/api/v1", APIKey: "key"},
	})
	resp, err := client.Complete(context.Background(), ChatRequest{
		ProviderID:   "test",
		ModelID:      "m",
		Model:        "model",
		SystemPrompt: "system",
		UserPrompt:   "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "ok [E1]" {
		t.Fatalf("text = %q", resp.Text)
	}
}
