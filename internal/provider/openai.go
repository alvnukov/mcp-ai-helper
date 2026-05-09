// Package provider implements third-party model provider clients.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/config"
)

// ChatClient is the minimal interface required by pipeline model steps.
type ChatClient interface {
	Complete(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// ChatRequest is a normalized chat completion request.
type ChatRequest struct {
	ProviderID      string
	ModelID         string
	Model           string
	SystemPrompt    string
	UserPrompt      string
	MaxOutputTokens int
	Temperature     float64
}

// ChatResponse is normalized provider completion output.
type ChatResponse struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
	Model      string `json:"model"`
	Text       string `json:"text"`
}

// OpenAICompatibleClient calls OpenAI-compatible chat completion endpoints.
type OpenAICompatibleClient struct {
	httpClient *http.Client
	providers  map[string]config.ProviderConfig
}

// NewOpenAICompatibleClient creates a client over configured providers.
func NewOpenAICompatibleClient(providers map[string]config.ProviderConfig) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		httpClient: &http.Client{Transport: sharedTransport},
		providers:  providers,
	}
}

// Client routes chat completions to the correct provider backend.
type Client struct {
	openAI    *OpenAICompatibleClient
	anthropic *anthropicClient
}

// NewClient creates a multi-provider client that routes by provider type.
func NewClient(providers map[string]config.ProviderConfig) *Client {
	return &Client{
		openAI:    NewOpenAICompatibleClient(providers),
		anthropic: &anthropicClient{httpClient: &http.Client{Transport: sharedTransport}, providers: providers},
	}
}

// Complete routes the request to the matching provider backend.
func (c *Client) Complete(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	cfg, ok := c.openAI.providers[req.ProviderID]
	if !ok {
		cfg, ok = c.anthropic.providers[req.ProviderID]
		if !ok {
			return ChatResponse{}, fmt.Errorf("provider %q is not configured", req.ProviderID)
		}
	}
	switch cfg.Type {
	case "anthropic":
		return c.anthropic.Complete(ctx, req)
	default:
		return c.openAI.Complete(ctx, req)
	}
}

// Complete sends a chat completion request to the selected provider.
func (c *OpenAICompatibleClient) Complete(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	cfg, ok := c.providers[req.ProviderID]
	if !ok {
		return ChatResponse{}, fmt.Errorf("provider %q is not configured", req.ProviderID)
	}
	apiKey := cfg.ResolvedAPIKey()
	if apiKey == "" {
		return ChatResponse{}, fmt.Errorf("provider %q: api key is empty", req.ProviderID)
	}

	body := map[string]any{
		"model": req.Model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"temperature": req.Temperature,
	}
	if req.MaxOutputTokens > 0 {
		body["max_tokens"] = req.MaxOutputTokens
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}

	urls := completionURLs(cfg)
	var lastErr error
	text, err := retryLoop(cfg, func() (string, error) {
		for _, url := range urls {
			text, err := c.post(ctx, cfg, apiKey, url, payload)
			if err == nil {
				return text, nil
			}
			if !isRetryable(err) {
				return "", err
			}
			lastErr = err
		}
		return "", lastErr
	})
	if err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{ProviderID: req.ProviderID, ModelID: req.ModelID, Model: req.Model, Text: text}, nil
}

func (c *OpenAICompatibleClient) post(ctx context.Context, cfg config.ProviderConfig, apiKey string, url string, payload []byte) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout())
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.AppName != "" {
		httpReq.Header.Set("HTTP-Referer", cfg.AppName)
		httpReq.Header.Set("X-Title", cfg.AppName)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", retryableError{err}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		return "", readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("provider HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", retryableError{err}
		}
		return "", err
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", fmt.Errorf("provider returned invalid JSON: %w", err)
	}
	if len(decoded.Choices) == 0 || decoded.Choices[0].Message.Content == "" {
		return "", errors.New("provider returned no message content")
	}
	return decoded.Choices[0].Message.Content, nil
}

type retryableError struct {
	err error
}

func (e retryableError) Error() string {
	return e.err.Error()
}

func (e retryableError) Unwrap() error {
	return e.err
}

func isRetryable(err error) bool {
	var retry retryableError
	return errors.As(err, &retry)
}

func completionURLs(cfg config.ProviderConfig) []string {
	if cfg.CompletionsURL != "" {
		return []string{strings.TrimRight(cfg.CompletionsURL, "/")}
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return []string{base}
	}
	urls := []string{base + "/chat/completions"}
	if strings.Contains(base, "/api/openai/v1") {
		urls = append(urls,
			strings.Replace(base, "/api/openai/v1", "/api/v1", 1)+"/chat/completions",
			strings.Replace(base, "/api/openai/v1", "/api/openai", 1)+"/chat/completions",
		)
	}
	return dedupe(urls)
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
