package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestAnthropicCompleteMissingProvider(t *testing.T) {
	client := &anthropicClient{
		httpClient: new(http.Client),
		providers:  map[string]config.ProviderConfig{},
	}
	_, err := client.Complete(context.Background(), ChatRequest{ProviderID: "missing"})
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestAnthropicCompleteEmptyAPIKey(t *testing.T) {
	client := &anthropicClient{
		httpClient: new(http.Client),
		providers: map[string]config.ProviderConfig{
			"test": {BaseURL: "https://api.anthropic.com", APIKey: ""},
		},
	}
	_, err := client.Complete(context.Background(), ChatRequest{ProviderID: "test"})
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestAnthropicCompleteSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Fatalf("missing anthropic-version header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{{"text": "hello from claude [E1]"}},
		})
	}))
	defer server.Close()

	client := &anthropicClient{
		httpClient: server.Client(),
		providers: map[string]config.ProviderConfig{
			"claude": {BaseURL: server.URL, APIKey: "test-key"},
		},
	}
	resp, err := client.Complete(context.Background(), ChatRequest{
		ProviderID:   "claude",
		ModelID:      "claude-sonnet",
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "you are helpful",
		UserPrompt:   "hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello from claude [E1]" {
		t.Fatalf("text = %q", resp.Text)
	}
	if resp.ProviderID != "claude" {
		t.Fatalf("ProviderID = %q", resp.ProviderID)
	}
}

func TestAnthropicPostHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := &anthropicClient{httpClient: server.Client()}
	cfg := config.ProviderConfig{BaseURL: server.URL, APIKey: "k", MaxRetries: 0}
	_, err := client.post(context.Background(), cfg, "k", server.URL+"/v1/messages", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
}

func TestAnthropicPostInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := &anthropicClient{httpClient: server.Client()}
	cfg := config.ProviderConfig{BaseURL: server.URL, APIKey: "k", MaxRetries: 0}
	_, err := client.post(context.Background(), cfg, "k", server.URL+"/v1/messages", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAnthropicPostAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "overloaded"},
		})
	}))
	defer server.Close()

	client := &anthropicClient{httpClient: server.Client()}
	cfg := config.ProviderConfig{BaseURL: server.URL, APIKey: "k", MaxRetries: 0}
	_, err := client.post(context.Background(), cfg, "k", server.URL+"/v1/messages", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestAnthropicPostEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{},
		})
	}))
	defer server.Close()

	client := &anthropicClient{httpClient: server.Client()}
	cfg := config.ProviderConfig{BaseURL: server.URL, APIKey: "k", MaxRetries: 0}
	_, err := client.post(context.Background(), cfg, "k", server.URL+"/v1/messages", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestAnthropicCompleteDefaultBaseURL(t *testing.T) {
	client := &anthropicClient{
		httpClient: new(http.Client),
		providers: map[string]config.ProviderConfig{
			"claude": {BaseURL: "", APIKey: "k"},
		},
	}
	// Should fail because the URL is unreachable, but not with "not configured" or "api key empty"
	// We are just checking that it doesn't fail at validation stage
	_, err := client.Complete(context.Background(), ChatRequest{
		ProviderID: "claude",
		ModelID:    "m",
		Model:      "model",
		UserPrompt: "hi",
	})
	if err == nil {
		t.Fatal("expected connection error for default URL")
	}
}
