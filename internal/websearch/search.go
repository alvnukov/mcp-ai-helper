// Package websearch implements compact provider-backed search results for MCP tools.
package websearch

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/webfetch"
)

const (
	// ProviderDuckDuckGoHTML is the explicit provider id for DuckDuckGo's HTML endpoint.
	ProviderDuckDuckGoHTML = "duckduckgo_html"

	defaultSearchURL          = "https://html.duckduckgo.com/html/"
	defaultMaxSearchResults   = 10
	hardMaxSearchResults      = 20
	maxSearchResponseBodySize = int64(512 * 1024)
)

var (
	resultLinkRe = regexp.MustCompile(`(?is)<a\b[^>]*class=["'][^"']*\bresult__a\b[^"']*["'][^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	snippetRe    = regexp.MustCompile(`(?is)<(?:a|div|span)\b[^>]*class=["'][^"']*\bresult__snippet\b[^"']*["'][^>]*>(.*?)</(?:a|div|span)>`)
	tagRe        = regexp.MustCompile(`(?is)<[^>]+>`)
)

// Request is one compact search request.
type Request struct {
	Query      string `json:"query"`
	Provider   string `json:"provider,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// Hit is one compact search result. It intentionally contains no page body.
type Hit struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	DisplayURL  string `json:"display_url,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
	Rank        int    `json:"rank"`
	Provider    string `json:"provider"`
	FetchedHint string `json:"fetched_hint,omitempty"`
}

// Result is the bounded model-facing search response.
type Result struct {
	Status      string                `json:"status"`
	Query       string                `json:"query"`
	Provider    string                `json:"provider,omitempty"`
	Total       int                   `json:"total"`
	Hits        []Hit                 `json:"hits"`
	Truncated   bool                  `json:"truncated"`
	Diagnostics []webfetch.Diagnostic `json:"diagnostics"`
}

// Search returns compact hits from an explicitly selected search provider.
func Search(ctx context.Context, policy config.WebPolicy, req Request) Result {
	policy = normalizePolicy(policy)
	query := strings.TrimSpace(req.Query)
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = policy.SearchProvider
	}
	result := Result{Status: "blocked", Query: query, Provider: provider, Hits: []Hit{}}
	if !policy.IsEnabled() {
		result.Diagnostics = append(result.Diagnostics, diag("web_disabled", "web search is disabled by web_policy.enabled"))
		return result
	}
	if query == "" {
		result.Diagnostics = append(result.Diagnostics, diag("empty_query", "query is required"))
		return result
	}
	if provider == "" {
		result.Diagnostics = append(result.Diagnostics, diag("search_provider_not_configured", "set web_policy.search_provider or pass an explicit provider argument"))
		return result
	}
	if provider != ProviderDuckDuckGoHTML {
		result.Diagnostics = append(result.Diagnostics, diag("unsupported_search_provider", fmt.Sprintf("unsupported web_search provider %q", provider)))
		return result
	}
	endpoint, err := searchEndpoint(policy)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diag("search_endpoint_invalid", err.Error()))
		return result
	}
	searchURL := *endpoint
	values := searchURL.Query()
	values.Set("q", query)
	searchURL.RawQuery = values.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL.String(), nil)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diag("search_request_invalid", err.Error()))
		return result
	}
	httpReq.Header.Set("User-Agent", policy.UserAgent)
	client := &http.Client{Timeout: time.Duration(policy.TimeoutSeconds) * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diag("search_failed", err.Error()))
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Diagnostics = append(result.Diagnostics, diag("search_http_status", fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)))
		return result
	}
	body, bodyTruncated, err := readSearchBody(resp.Body)
	if err != nil {
		result.Diagnostics = append(result.Diagnostics, diag("search_read_failed", err.Error()))
		return result
	}
	hits, total, hitsTruncated := parseDuckDuckGoHTML(string(body), endpoint, boundedMaxResults(policy, req.MaxResults), provider)
	result.Status = "complete"
	result.Hits = hits
	result.Total = total
	result.Truncated = bodyTruncated || hitsTruncated
	if bodyTruncated {
		result.Diagnostics = append(result.Diagnostics, diag("search_response_truncated", fmt.Sprintf("search response exceeded %d bytes", maxSearchResponseBodySize)))
	}
	return result
}

func normalizePolicy(policy config.WebPolicy) config.WebPolicy {
	policy.SearchProvider = strings.TrimSpace(policy.SearchProvider)
	if strings.TrimSpace(policy.SearchURL) == "" {
		policy.SearchURL = defaultSearchURL
	}
	if policy.MaxSearchResults <= 0 {
		policy.MaxSearchResults = defaultMaxSearchResults
	}
	if policy.MaxSearchResults > hardMaxSearchResults {
		policy.MaxSearchResults = hardMaxSearchResults
	}
	if policy.TimeoutSeconds <= 0 {
		policy.TimeoutSeconds = 20
	}
	if strings.TrimSpace(policy.UserAgent) == "" {
		policy.UserAgent = "mcp-ai-helper/0.1"
	}
	if len(policy.AllowedSchemes) == 0 {
		policy.AllowedSchemes = []string{"https", "http"}
	}
	return policy
}

func boundedMaxResults(policy config.WebPolicy, requested int) int {
	limit := policy.MaxSearchResults
	if limit <= 0 {
		limit = defaultMaxSearchResults
	}
	if limit > hardMaxSearchResults {
		limit = hardMaxSearchResults
	}
	if requested > 0 && requested < limit {
		limit = requested
	}
	if limit <= 0 {
		return defaultMaxSearchResults
	}
	return limit
}

func searchEndpoint(policy config.WebPolicy) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimSpace(policy.SearchURL))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, errors.New("web_policy.search_url must be an absolute http/https URL")
	}
	if err := validateEndpoint(endpoint, policy); err != nil {
		return nil, err
	}
	return endpoint, nil
}

func validateEndpoint(endpoint *url.URL, policy config.WebPolicy) error {
	if !containsFold(policy.AllowedSchemes, endpoint.Scheme) {
		return fmt.Errorf("scheme %q is not allowed", endpoint.Scheme)
	}
	host := strings.ToLower(endpoint.Hostname())
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

func readSearchBody(r io.Reader) ([]byte, bool, error) {
	limited := &io.LimitedReader{R: r, N: maxSearchResponseBodySize + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > maxSearchResponseBodySize {
		return body[:maxSearchResponseBodySize], true, nil
	}
	return body, false, nil
}

func parseDuckDuckGoHTML(body string, endpoint *url.URL, maxResults int, provider string) ([]Hit, int, bool) {
	matches := resultLinkRe.FindAllStringSubmatchIndex(body, -1)
	hits := make([]Hit, 0, min(maxResults, len(matches)))
	seen := map[string]bool{}
	total := 0
	truncated := false
	for _, match := range matches {
		if len(match) < 6 || match[2] < 0 || match[4] < 0 {
			continue
		}
		href := body[match[2]:match[3]]
		title := cleanHTML(body[match[4]:match[5]])
		resultURL := resolveResultURL(href, endpoint)
		if title == "" || resultURL == "" || seen[resultURL] {
			continue
		}
		seen[resultURL] = true
		total++
		if len(hits) >= maxResults {
			truncated = true
			continue
		}
		hits = append(hits, Hit{
			Title:       title,
			URL:         resultURL,
			DisplayURL:  displayURL(resultURL),
			Snippet:     snippetAfter(body, match[1]),
			Rank:        len(hits) + 1,
			Provider:    provider,
			FetchedHint: "not_fetched",
		})
	}
	return hits, total, truncated
}

func snippetAfter(body string, offset int) string {
	if offset < 0 || offset >= len(body) {
		return ""
	}
	end := offset + 2500
	if end > len(body) {
		end = len(body)
	}
	match := snippetRe.FindStringSubmatch(body[offset:end])
	if len(match) < 2 {
		return ""
	}
	return cleanHTML(match[1])
}

func resolveResultURL(href string, endpoint *url.URL) string {
	clean := strings.TrimSpace(html.UnescapeString(href))
	if clean == "" {
		return ""
	}
	if strings.HasPrefix(clean, "//") {
		clean = endpoint.Scheme + ":" + clean
	}
	parsed, err := url.Parse(clean)
	if err != nil {
		return ""
	}
	if !parsed.IsAbs() {
		parsed = endpoint.ResolveReference(parsed)
	}
	if uddg := parsed.Query().Get("uddg"); uddg != "" {
		decoded, err := url.Parse(uddg)
		if err == nil && (decoded.Scheme == "http" || decoded.Scheme == "https") && decoded.Host != "" {
			return decoded.String()
		}
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func displayURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func cleanHTML(value string) string {
	withoutTags := tagRe.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(html.UnescapeString(withoutTags)), " ")
}

func diag(code string, message string) webfetch.Diagnostic {
	return webfetch.Diagnostic{Code: code, Message: message}
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
