// Package confluence wraps the virtomize/confluence-go-api client for mcp-ai-helper tools.
package confluence

import (
	"context"
	"fmt"
	"net/http"
	"os"

	goconfluence "github.com/virtomize/confluence-go-api"
)

// Config holds Confluence connection settings (package-local, converted from config.ConfluenceConfig).
type Config struct {
	URL       string
	Username  string
	APIKey    string
	APIKeyEnv string
}

// ResolvedAPIKey returns the API key: direct value first, then env fallback.
func (c Config) ResolvedAPIKey() string {
	if c.APIKey != "" {
		return c.APIKey
	}
	if c.APIKeyEnv != "" {
		return os.Getenv(c.APIKeyEnv)
	}
	return ""
}

// Client wraps the goconfluence API.
type Client struct {
	api *goconfluence.API
}

type contextRoundTripper struct {
	ctx  context.Context
	base http.RoundTripper
}

func (rt contextRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.base.RoundTrip(req.Clone(rt.ctx))
}

func (c *Client) apiWithContext(ctx context.Context) *goconfluence.API {
	api := *c.api
	httpClient := c.api.Client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	clientCopy := *httpClient
	baseTransport := clientCopy.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	clientCopy.Transport = contextRoundTripper{ctx: ctx, base: baseTransport}
	api.Client = &clientCopy
	return &api
}

// NewClient creates a Confluence client using PAT (Bearer-equivalent via library's Basic Auth with empty username).
func NewClient(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("confluence: url is required")
	}
	apiKey := cfg.ResolvedAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("confluence: api key is required — set api_key or api_key_env")
	}
	api, err := goconfluence.NewAPI(cfg.URL, cfg.Username, apiKey)
	if err != nil {
		return nil, fmt.Errorf("confluence: connect to %s: %w", cfg.URL, err)
	}
	return &Client{api: api}, nil
}

// NewClientWithHTTP creates a client with a custom HTTP client (for testing).
func NewClientWithHTTP(url string, hc *http.Client) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("confluence: url is required")
	}
	api, err := goconfluence.NewAPIWithClient(url, hc)
	if err != nil {
		return nil, err
	}
	return &Client{api: api}, nil
}

// SearchResult is a simplified search hit.
type SearchResult struct {
	ID     string
	Type   string
	Title  string
	Status string
}

// Search performs a CQL search.
func (c *Client) Search(cql string, limit int) ([]SearchResult, error) {
	return c.SearchContext(context.Background(), cql, limit)
}

// SearchContext performs a CQL search and propagates cancellation to Confluence.
func (c *Client) SearchContext(ctx context.Context, cql string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	items, _, err := c.searchPageContext(ctx, cql, limit, "")
	return items, err
}

func (c *Client) searchPageContext(ctx context.Context, cql string, limit int, next string) ([]SearchResult, string, error) {
	query := goconfluence.SearchQuery{CQL: cql, Limit: limit}
	api := c.apiWithContext(ctx)
	var result *goconfluence.Search
	var err error
	if next == "" {
		result, err = api.Search(query)
	} else {
		result, err = api.SearchWithNext(query, next)
	}
	if err != nil {
		return nil, "", fmt.Errorf("confluence search: %w", err)
	}
	items := make([]SearchResult, 0, len(result.Results))
	for _, resultItem := range result.Results {
		id, contentType := resultItem.ID, resultItem.Type
		if id == "" && resultItem.Content.ID != "" {
			id = resultItem.Content.ID
			if contentType == "" {
				contentType = resultItem.Content.Type
			}
		}
		items = append(items, SearchResult{
			ID: id, Type: contentType, Title: resultItem.Title, Status: resultItem.Status,
		})
	}
	return items, result.Links.Next, nil
}

// PageInfo holds content page data.
type PageInfo struct {
	ID      string
	Type    string
	Title   string
	Body    string
	Version int
	Space   string
	URL     string
}

// GetContentByID returns a single content item by ID.
func (c *Client) GetContentByID(id string) (*PageInfo, error) {
	return c.GetContentByIDContext(context.Background(), id)
}

// GetContentByIDContext returns content by ID and propagates cancellation to Confluence.
func (c *Client) GetContentByIDContext(ctx context.Context, id string) (*PageInfo, error) {
	content, err := c.apiWithContext(ctx).GetContentByID(id, goconfluence.ContentQuery{
		Expand: []string{"body.storage", "version", "space"},
	})
	if err != nil {
		return nil, fmt.Errorf("confluence get %s: %w", id, err)
	}
	page := &PageInfo{
		ID:    content.ID,
		Type:  content.Type,
		Title: content.Title,
	}
	if content.Body.Storage.Value != "" {
		page.Body = content.Body.Storage.Value
	}
	if content.Version != nil {
		page.Version = content.Version.Number
	}
	if content.Space != nil {
		page.Space = content.Space.Key
	}
	if content.Links != nil && content.Links.WebUI != "" {
		page.URL = content.Links.WebUI
	}
	return page, nil
}

// SpaceInfo holds space summary data.
type SpaceInfo struct {
	ID   int
	Key  string
	Name string
	Type string
}

// GetSpaces returns all spaces.
func (c *Client) GetSpaces() ([]SpaceInfo, error) {
	return c.GetSpacesContext(context.Background())
}

// GetSpacesContext returns all spaces and propagates cancellation to every page request.
func (c *Client) GetSpacesContext(ctx context.Context) ([]SpaceInfo, error) {
	var spaces []SpaceInfo
	start := 0
	const pageSize = 50
	api := c.apiWithContext(ctx)
	for {
		result, err := api.GetAllSpaces(goconfluence.AllSpacesQuery{
			Start: start,
			Limit: pageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("confluence spaces: %w", err)
		}
		for _, space := range result.Results {
			spaces = append(spaces, SpaceInfo{
				ID:   space.ID,
				Key:  space.Key,
				Name: space.Name,
				Type: space.Type,
			})
		}
		if len(result.Results) < pageSize {
			break
		}
		start += pageSize
	}
	return spaces, nil
}
