// Package confluence wraps the virtomize/confluence-go-api client for mcp-ai-helper tools.
package confluence

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	goconfluence "github.com/virtomize/confluence-go-api"

	"github.com/alvnukov/mcp-ai-helper/internal/fileops"
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

// UpdateRequest describes a guarded edit of one page that already exists.
// Either Body carries the whole new storage-format body, or Old and New name a
// span to replace inside the body the page already has.
type UpdateRequest struct {
	PageID          string
	ExpectedVersion int
	Title           string
	Body            string
	Old             string
	New             string
	VersionMessage  string
	MinorEdit       bool
}

// CreateRequest describes a page that does not exist yet.
type CreateRequest struct {
	SpaceKey string
	Title    string
	Body     string
	ParentID string
}

// EditResult reports what an edit did. Status is "ok" or "conflict" — the same
// two words a guarded file replace answers with — and a conflict means nothing
// was sent to Confluence.
type EditResult struct {
	Status  string `json:"status"`
	Changed bool   `json:"changed"`
	PageID  string `json:"page_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Space   string `json:"space,omitempty"`
	Version int    `json:"version,omitempty"`
	URL     string `json:"url,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// requireGuard rejects an edit that cannot be guarded at all: one with no page
// read behind it, or none naming the version that read reported.
func requireGuard(current *PageInfo, expectedVersion int, op string) error {
	if current == nil {
		return fmt.Errorf("confluence %s: the page must be read before it is edited", op)
	}
	if expectedVersion <= 0 {
		return fmt.Errorf("confluence %s: expected_version is required; pass the version the page read reported", op)
	}
	return nil
}

// staleRead reports the conflict an edit must return when the page has moved on
// from the version the caller last saw. Update and delete share it, so neither
// can drift into writing on a stale read.
func staleRead(current *PageInfo, expectedVersion int) (EditResult, bool) {
	if current.Version == expectedVersion {
		return EditResult{}, false
	}
	return EditResult{
		Status:  "conflict",
		PageID:  current.ID,
		Title:   current.Title,
		Space:   current.Space,
		Version: current.Version,
		Reason: fmt.Sprintf("page is at version %d, not the expected %d; read it again and edit from what it says now",
			current.Version, expectedVersion),
	}, true
}

// UpdatePageContext writes a new body, a new title, or both, to the page the
// caller already read.
//
// current is that read. It supplies the fields Confluence demands back on every
// update — type, title, space — and rejects the request without; and it
// supplies the version the guard is checked against. Taking it as an argument
// rather than reading the page again here keeps the page whose space was
// checked against the allowlist and the page being written the same page.
//
// That version is the guard. Confluence accepts an update only at current+1, so
// a page someone else edited since the caller read it is refused: here, with
// both versions named, and again at the server if it changes between this check
// and the write.
func (c *Client) UpdatePageContext(ctx context.Context, current *PageInfo, req UpdateRequest) (EditResult, error) {
	if err := requireGuard(current, req.ExpectedVersion, "update"); err != nil {
		return EditResult{}, err
	}
	if req.Body != "" && req.Old != "" {
		return EditResult{}, fmt.Errorf("confluence update: set body or old/new, not both")
	}
	if req.Body == "" && req.Old == "" && req.Title == "" {
		return EditResult{}, fmt.Errorf("confluence update: nothing to change; set body, old and new, or title")
	}
	if conflict, stale := staleRead(current, req.ExpectedVersion); stale {
		return conflict, nil
	}

	body, reason := current.Body, ""
	switch {
	case req.Body != "":
		body = req.Body
	case req.Old != "":
		edit := fileops.ReplaceUniqueSpan(current.Body, req.Old, req.New)
		if edit.Status != "ok" {
			return EditResult{
				Status: edit.Status, PageID: current.ID, Title: current.Title,
				Space: current.Space, Version: current.Version, Reason: edit.Reason,
			}, nil
		}
		body, reason = edit.Text, edit.Reason
	}
	title := current.Title
	if req.Title != "" {
		title = req.Title
	}
	// Confluence records a version for every update it accepts, including one
	// that changes nothing. Sending that is a lie in the page history, so an
	// edit already applied stops here instead.
	if body == current.Body && title == current.Title {
		if reason == "" {
			reason = "page already says this"
		}
		return EditResult{
			Status: "ok", Changed: false, PageID: current.ID, Title: title,
			Space: current.Space, Version: current.Version, URL: current.URL, Reason: reason,
		}, nil
	}

	pageType := current.Type
	if pageType == "" {
		pageType = "page"
	}
	updated, err := c.apiWithContext(ctx).UpdateContent(&goconfluence.Content{
		ID:    current.ID,
		Type:  pageType,
		Title: title,
		Space: &goconfluence.Space{Key: current.Space},
		Body: goconfluence.Body{Storage: goconfluence.Storage{
			Value:          body,
			Representation: "storage",
		}},
		Version: &goconfluence.Version{
			Number:    req.ExpectedVersion + 1,
			MinorEdit: req.MinorEdit,
			Message:   req.VersionMessage,
		},
	})
	if err != nil {
		return EditResult{}, fmt.Errorf("confluence update %s: %w", current.ID, err)
	}
	result := EditResult{
		Status: "ok", Changed: true, PageID: current.ID, Title: title,
		Space: current.Space, Version: req.ExpectedVersion + 1, URL: current.URL,
	}
	applyContentEcho(&result, updated)
	return result, nil
}

// CreatePageContext adds a page to a space.
func (c *Client) CreatePageContext(ctx context.Context, req CreateRequest) (EditResult, error) {
	if strings.TrimSpace(req.SpaceKey) == "" {
		return EditResult{}, fmt.Errorf("confluence create: space_key is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		return EditResult{}, fmt.Errorf("confluence create: title is required")
	}
	content := &goconfluence.Content{
		Type:  "page",
		Title: req.Title,
		Space: &goconfluence.Space{Key: req.SpaceKey},
		Body: goconfluence.Body{Storage: goconfluence.Storage{
			Value:          req.Body,
			Representation: "storage",
		}},
	}
	if req.ParentID != "" {
		content.Ancestors = []goconfluence.Ancestor{{ID: req.ParentID}}
	}
	created, err := c.apiWithContext(ctx).CreateContent(content)
	if err != nil {
		return EditResult{}, fmt.Errorf("confluence create in %s: %w", req.SpaceKey, err)
	}
	result := EditResult{Status: "ok", Changed: true, Title: req.Title, Space: req.SpaceKey, Version: 1}
	applyContentEcho(&result, created)
	return result, nil
}

// DeletePageContext removes the page the caller already read. It takes the
// version guard an update takes, so a page that moved on since the caller
// looked at it is not deleted on the strength of a stale read.
func (c *Client) DeletePageContext(ctx context.Context, current *PageInfo, expectedVersion int) (EditResult, error) {
	if err := requireGuard(current, expectedVersion, "delete"); err != nil {
		return EditResult{}, err
	}
	if conflict, stale := staleRead(current, expectedVersion); stale {
		return conflict, nil
	}
	if _, err := c.apiWithContext(ctx).DelContent(current.ID); err != nil {
		return EditResult{}, fmt.Errorf("confluence delete %s: %w", current.ID, err)
	}
	return EditResult{
		Status: "ok", Changed: true, PageID: current.ID, Title: current.Title,
		Space: current.Space, Version: current.Version,
	}, nil
}

// applyContentEcho takes the id, version and link out of what Confluence echoed
// back, where it said anything: the server assigns the id of a new page, and it
// is the authority on the version an accepted edit landed on.
func applyContentEcho(result *EditResult, content *goconfluence.Content) {
	if content == nil {
		return
	}
	if content.ID != "" {
		result.PageID = content.ID
	}
	if content.Version != nil && content.Version.Number > 0 {
		result.Version = content.Version.Number
	}
	if content.Links != nil && content.Links.WebUI != "" {
		result.URL = content.Links.WebUI
	}
}
