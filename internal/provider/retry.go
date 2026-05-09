package provider

import (
	"net"
	"net/http"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
)

var sharedTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

func retryLoop(cfg config.ProviderConfig, fn func() (string, error)) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= cfg.RetryCount(); attempt++ {
		text, err := fn()
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !isRetryable(err) {
			break
		}
		if attempt < cfg.RetryCount() {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}
	}
	return "", lastErr
}
