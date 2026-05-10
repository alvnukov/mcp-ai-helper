// Package confluence wraps the virtomize/confluence-go-api client for mcp-ai-helper tools.
package confluence

import (
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

// NewClient creates a Confluence client using PAT (Bearer-equivalent via library's Basic Auth with empty username).
func NewClient(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("confluence: url is required")
	}
	apiKey := cfg.ResolvedAPIKey()
	if apiKey == "" && cfg.APIKeyEnv != "" {
		apiKey = "" // resolved at library level for env
	}
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
	if limit <= 0 {
		limit = 20
	}
	result, err := c.api.Search(goconfluence.SearchQuery{
		CQL:   cql,
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("confluence search: %w", err)
	}
	items := make([]SearchResult, 0, len(result.Results))
	for _, r := range result.Results {
		id, typ := r.ID, r.Type
		if id == "" && r.Content.ID != "" {
			id = r.Content.ID
			if typ == "" { typ = r.Content.Type }
		}
		items = append(items, SearchResult{
			ID:     id,
			Type:   typ,
			Title:  r.Title,
			Status: r.Status,
		})
	}
	return items, nil
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
	content, err := c.api.GetContentByID(id, goconfluence.ContentQuery{
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
	result, err := c.api.GetAllSpaces(goconfluence.AllSpacesQuery{
		Limit: 50,
	})
	if err != nil {
		return nil, fmt.Errorf("confluence spaces: %w", err)
	}
	spaces := make([]SpaceInfo, 0, len(result.Results))
	for _, s := range result.Results {
		spaces = append(spaces, SpaceInfo{
			ID:   s.ID,
			Key:  s.Key,
			Name: s.Name,
			Type: s.Type,
		})
	}
	return spaces, nil
}
