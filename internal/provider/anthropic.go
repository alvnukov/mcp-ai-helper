package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/config"
)

const defaultAnthropicBaseURL = "https://api.anthropic.com"
const anthropicVersion = "2023-06-01"

type anthropicClient struct {
	httpClient *http.Client
	providers  map[string]config.ProviderConfig
}

type anthropicRequest struct {
	Model       string           `json:"model"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature,omitempty"`
	System      anthropicSystem  `json:"system,omitempty"`
	Messages    []anthropicMsg   `json:"messages"`
}

type anthropicSystem struct {
	Text string `json:"text"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *anthropicClient) Complete(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	cfg, ok := c.providers[req.ProviderID]
	if !ok {
		return ChatResponse{}, fmt.Errorf("provider %q is not configured", req.ProviderID)
	}
	apiKey := cfg.ResolvedAPIKey()
	if apiKey == "" {
		return ChatResponse{}, fmt.Errorf("provider %q: api key is empty", req.ProviderID)
	}

	maxTokens := req.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	temperature := req.Temperature
	if temperature <= 0 {
		temperature = 1.0
	}

	messages := []anthropicMsg{
		{Role: "user", Content: req.UserPrompt},
	}
	body := anthropicRequest{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Messages:    messages,
	}
	if req.SystemPrompt != "" {
		body.System = anthropicSystem{Text: req.SystemPrompt}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAnthropicBaseURL
	}
	url := baseURL + "/v1/messages"

	text, err := retryLoop(cfg, func() (string, error) {
		return c.post(ctx, cfg, apiKey, url, payload)
	})
	if err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{ProviderID: req.ProviderID, ModelID: req.ModelID, Model: req.Model, Text: text}, nil
}

func (c *anthropicClient) post(ctx context.Context, cfg config.ProviderConfig, apiKey string, url string, payload []byte) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout())
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.AppName != "" {
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
		err := fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return "", retryableError{err}
		}
		return "", err
	}

	var decoded anthropicResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", fmt.Errorf("anthropic returned invalid JSON: %w", err)
	}
	if decoded.Error != nil {
		return "", fmt.Errorf("anthropic error: %s", decoded.Error.Message)
	}
	if len(decoded.Content) == 0 || decoded.Content[0].Text == "" {
		return "", fmt.Errorf("anthropic returned no message content")
	}
	return decoded.Content[0].Text, nil
}
