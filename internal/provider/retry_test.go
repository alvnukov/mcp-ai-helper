package provider

import (
	"errors"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestRetryLoopSuccessFirstAttempt(t *testing.T) {
	t.Parallel()
	cfg := config.ProviderConfig{MaxRetries: 2}
	calls := 0
	text, err := retryLoop(cfg, func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "ok" {
		t.Fatalf("text = %q, want ok", text)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRetryLoopRetriesOnRetryableError(t *testing.T) {
	t.Parallel()
	cfg := config.ProviderConfig{MaxRetries: 2}
	calls := 0
	text, err := retryLoop(cfg, func() (string, error) {
		calls++
		if calls < 3 {
			return "", retryableError{errors.New("transient")}
		}
		return "recovered", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "recovered" {
		t.Fatalf("text = %q, want recovered", text)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestRetryLoopStopsOnNonRetryableError(t *testing.T) {
	t.Parallel()
	cfg := config.ProviderConfig{MaxRetries: 3}
	calls := 0
	_, err := retryLoop(cfg, func() (string, error) {
		calls++
		return "", errors.New("permanent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRetryLoopExhaustsRetries(t *testing.T) {
	t.Parallel()
	cfg := config.ProviderConfig{MaxRetries: 1}
	calls := 0
	_, err := retryLoop(cfg, func() (string, error) {
		calls++
		return "", retryableError{errors.New("always failing")}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (initial + 1 retry)", calls)
	}
}

func TestRetryLoopZeroRetries(t *testing.T) {
	t.Parallel()
	cfg := config.ProviderConfig{MaxRetries: 0}
	calls := 0
	_, err := retryLoop(cfg, func() (string, error) {
		calls++
		return "", retryableError{errors.New("fail")}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	if isRetryable(errors.New("plain")) {
		t.Fatal("plain error should not be retryable")
	}
	if !isRetryable(retryableError{errors.New("wrapped")}) {
		t.Fatal("retryableError should be retryable")
	}
	var retryErr retryableError
	if !errors.As(retryableError{errors.New("test")}, &retryErr) {
		t.Fatal("errors.As should unwrap retryableError")
	}
}

func TestRetryableErrorUnwrap(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	retry := retryableError{inner}
	if !errors.Is(retry, inner) {
		t.Fatal("errors.Is should match inner error")
	}
	if retry.Error() != "inner" {
		t.Fatalf("Error() = %q, want inner", retry.Error())
	}
}

func TestSharedTransport(t *testing.T) {
	t.Parallel()
	if sharedTransport == nil {
		t.Fatal("sharedTransport should not be nil")
	}
	if sharedTransport.MaxIdleConns != 100 {
		t.Fatalf("MaxIdleConns = %d, want 100", sharedTransport.MaxIdleConns)
	}
	if sharedTransport.IdleConnTimeout == 0 {
		t.Fatal("IdleConnTimeout should be set")
	}
	if sharedTransport.TLSHandshakeTimeout == 0 {
		t.Fatal("TLSHandshakeTimeout should be set")
	}
}

func TestRetryLoopRetryableHTTPStatuses(t *testing.T) {
	t.Parallel()
	// 429 and 5xx are wrapped as retryableError in post methods
	err429 := retryableError{errors.New("HTTP 429")}
	if !isRetryable(err429) {
		t.Fatal("HTTP 429 should be retryable")
	}
	err500 := retryableError{errors.New("HTTP 500")}
	if !isRetryable(err500) {
		t.Fatal("HTTP 500 should be retryable")
	}
	err400 := errors.New("HTTP 400")
	if isRetryable(err400) {
		t.Fatal("HTTP 400 should not be retryable")
	}
}

func TestNewClientCreatesBothBackends(t *testing.T) {
	t.Parallel()
	providers := map[string]config.ProviderConfig{
		"openai":    {Type: "generic", BaseURL: "https://api.openai.com/v1"},
		"anthropic": {Type: "anthropic", BaseURL: "https://api.anthropic.com"},
	}
	client := NewClient(providers)
	if client.openAI == nil {
		t.Fatal("openAI client should not be nil")
	}
	if client.anthropic == nil {
		t.Fatal("anthropic client should not be nil")
	}
	if client.openAI.httpClient == nil {
		t.Fatal("openAI httpClient should not be nil")
	}
	if client.openAI.httpClient.Transport != sharedTransport {
		t.Fatal("openAI should use sharedTransport")
	}
	if client.anthropic.httpClient == nil {
		t.Fatal("anthropic httpClient should not be nil")
	}
	if client.anthropic.httpClient.Transport != sharedTransport {
		t.Fatal("anthropic should use sharedTransport")
	}
}
