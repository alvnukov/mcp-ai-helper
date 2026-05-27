// Package webfetch implements bounded, lossless web fetching for MCP tools.
package webfetch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
)

// FetchRequest is one bounded fetch operation.
type FetchRequest struct {
	URL            string `json:"url"`
	MaxSourceBytes int64  `json:"max_source_bytes,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// CacheInfo describes whether an artifact already existed.
type CacheInfo struct {
	Status     string `json:"status"`
	TTLSeconds int    `json:"ttl_seconds,omitempty"`
}

// Diagnostic is a compact structured fetch diagnostic.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result is the bounded model-facing fetch response.
type Result struct {
	Status           string       `json:"status"`
	DocID            string       `json:"doc_id,omitempty"`
	RequestedURL     string       `json:"requested_url"`
	FinalURL         string       `json:"final_url,omitempty"`
	Redirects        []string     `json:"redirects"`
	ContentType      string       `json:"content_type,omitempty"`
	Encoding         string       `json:"encoding,omitempty"`
	SourceSHA256     string       `json:"source_sha256,omitempty"`
	SourceBytes      int64        `json:"source_bytes,omitempty"`
	NormalizedSHA256 string       `json:"normalized_sha256,omitempty"`
	NormalizedBytes  int64        `json:"normalized_bytes,omitempty"`
	FetchedAt        string       `json:"fetched_at,omitempty"`
	Cache            CacheInfo    `json:"cache"`
	Diagnostics      []Diagnostic `json:"diagnostics"`
}

// Client fetches URLs under policy and stores accepted source artifacts.
type Client struct {
	policy config.WebPolicy
	http   *http.Client
}

// NewClient creates a policy-bounded client.
func NewClient(policy config.WebPolicy) *Client {
	policy = normalizePolicy(policy)
	return &Client{policy: policy, http: httpClient(policy)}
}

func httpClient(policy config.WebPolicy) *http.Client {
	client := &http.Client{Timeout: time.Duration(policy.TimeoutSeconds) * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= policy.MaxRedirects {
			return fmt.Errorf("too many redirects: max %d", policy.MaxRedirects)
		}
		return validateURL(req.URL, policy)
	}
	return client
}

func normalizePolicy(policy config.WebPolicy) config.WebPolicy {
	if policy.MaxSourceBytes <= 0 {
		policy.MaxSourceBytes = 1048576
	}
	if policy.TimeoutSeconds <= 0 {
		policy.TimeoutSeconds = 20
	}
	if policy.MaxRedirects <= 0 {
		policy.MaxRedirects = 5
	}
	if strings.TrimSpace(policy.CacheDir) == "" {
		policy.CacheDir = "~/.mcp-ai-helper/web"
	}
	if len(policy.AllowedSchemes) == 0 {
		policy.AllowedSchemes = []string{"https", "http"}
	}
	if len(policy.AcceptedContentTypes) == 0 {
		policy.AcceptedContentTypes = []string{"text/html", "text/plain", "application/json", "application/xml", "text/"}
	}
	if strings.TrimSpace(policy.UserAgent) == "" {
		policy.UserAgent = "mcp-ai-helper/0.1"
	}
	return policy
}

// Fetch downloads and stores a complete accepted source body, returning only metadata.
func (c *Client) Fetch(ctx context.Context, req FetchRequest) (Result, error) {
	requested := strings.TrimSpace(req.URL)
	result := Result{Status: "blocked", RequestedURL: requested, Redirects: []string{}, Cache: CacheInfo{Status: "none"}}
	if !c.policy.IsEnabled() {
		result.Diagnostics = append(result.Diagnostics, diag("web_disabled", "web fetch is disabled by web_policy.enabled"))
		return result, nil
	}
	u, err := url.Parse(requested)
	if err != nil || u.Scheme == "" || u.Host == "" {
		result.Diagnostics = append(result.Diagnostics, diag("invalid_url", "url must be absolute with scheme and host"))
		return result, nil
	}
	if err := validateURL(u, c.policy); err != nil {
		result.Diagnostics = append(result.Diagnostics, diag("policy_denied", err.Error()))
		return result, nil
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("User-Agent", c.policy.UserAgent)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diag("fetch_failed", err.Error()))
		return result, nil
	}
	defer resp.Body.Close()
	result.FinalURL = resp.Request.URL.String()
	result.ContentType = resp.Header.Get("Content-Type")
	result.Encoding = contentEncoding(result.ContentType)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Diagnostics = append(result.Diagnostics, diag("http_status", fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)))
		return result, nil
	}
	if !contentTypeAllowed(result.ContentType, c.policy.AcceptedContentTypes) {
		result.Diagnostics = append(result.Diagnostics, diag("content_type_denied", fmt.Sprintf("content type %q is not accepted", result.ContentType)))
		return result, nil
	}
	maxBytes := c.policy.MaxSourceBytes
	if req.MaxSourceBytes > 0 && req.MaxSourceBytes < maxBytes {
		maxBytes = req.MaxSourceBytes
	}
	body, complete, err := readBounded(resp.Body, maxBytes)
	if err != nil {
		return Result{}, err
	}
	if !complete {
		result.Status = "incomplete"
		result.SourceBytes = int64(len(body))
		result.Diagnostics = append(result.Diagnostics, diag("size_limit", fmt.Sprintf("response exceeded max_source_bytes %d", maxBytes)))
		return result, nil
	}
	return c.persist(result, body)
}

func (c *Client) persist(result Result, source []byte) (Result, error) {
	normalized := normalizeContent(source, result.ContentType)
	sourceSum := sha256.Sum256(source)
	normalizedSum := sha256.Sum256([]byte(normalized))
	idSum := sha256.Sum256([]byte(result.FinalURL + "\n" + hex.EncodeToString(sourceSum[:])))
	docID := "web_" + hex.EncodeToString(idSum[:8])
	dir := filepath.Join(expandHome(c.policy.CacheDir), docID)
	metaPath := filepath.Join(dir, "metadata.json")
	cacheStatus := "miss"
	if _, err := os.Stat(metaPath); err == nil {
		cacheStatus = "hit"
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "source.bin"), source, 0o600); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "normalized.txt"), []byte(normalized), 0o600); err != nil {
		return Result{}, err
	}
	result.Status = "complete"
	result.DocID = docID
	result.SourceSHA256 = hex.EncodeToString(sourceSum[:])
	result.SourceBytes = int64(len(source))
	result.NormalizedSHA256 = hex.EncodeToString(normalizedSum[:])
	result.NormalizedBytes = int64(len(normalized))
	result.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	result.Cache = CacheInfo{Status: cacheStatus}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(metaPath, data, 0o600); err != nil {
		return Result{}, err
	}
	return result, nil
}

func readBounded(r io.Reader, maxBytes int64) ([]byte, bool, error) {
	limited := &io.LimitedReader{R: r, N: maxBytes + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > maxBytes {
		return body[:maxBytes], false, nil
	}
	return body, true, nil
}

func validateURL(u *url.URL, policy config.WebPolicy) error {
	if !containsFold(policy.AllowedSchemes, u.Scheme) {
		return fmt.Errorf("scheme %q is not allowed", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return errors.New("host is required")
	}
	if containsHost(policy.DeniedHosts, host) {
		return fmt.Errorf("host %q is denied", host)
	}
	if len(policy.AllowedHosts) > 0 {
		if !containsHost(policy.AllowedHosts, host) {
			return fmt.Errorf("host %q is not in allowed_hosts", host)
		}
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("host %q is denied", host)
	}
	if ip := net.ParseIP(host); ip != nil && !publicIP(ip) {
		return fmt.Errorf("ip host %q is not public", host)
	}
	return nil
}

func publicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsUnspecified()
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func containsHost(values []string, host string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), host) {
			return true
		}
	}
	return false
}

func contentTypeAllowed(contentType string, accepted []string) bool {
	base := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	for _, value := range accepted {
		candidate := strings.ToLower(strings.TrimSpace(value))
		if candidate == "" {
			continue
		}
		if strings.HasSuffix(candidate, "/") && strings.HasPrefix(base, candidate) {
			return true
		}
		if base == candidate {
			return true
		}
	}
	return false
}

func contentEncoding(contentType string) string {
	for _, part := range strings.Split(contentType, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "charset=") {
			return strings.Trim(strings.TrimPrefix(part, "charset="), "\"")
		}
	}
	return ""
}

var (
	scriptStylePattern = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	spacePattern       = regexp.MustCompile(`[ \t\r\n]+`)
)

func normalizeContent(source []byte, contentType string) string {
	text := string(source)
	base := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if base == "text/html" {
		text = scriptStylePattern.ReplaceAllString(text, " ")
		text = tagPattern.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	return strings.TrimSpace(spacePattern.ReplaceAllString(text, " "))
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func diag(code string, message string) Diagnostic {
	return Diagnostic{Code: code, Message: message}
}
