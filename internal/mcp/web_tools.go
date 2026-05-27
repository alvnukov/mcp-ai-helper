package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/webfetch"
)

type webFetchRequest struct {
	URL            string `json:"url"`
	RepoPath       string `json:"repo_path"`
	MaxSourceBytes int64  `json:"max_source_bytes"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type fetchedDocReadRequest struct {
	RepoPath string `json:"repo_path"`
	DocID    string `json:"doc_id"`
	Source   string `json:"source"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type fetchedDocFindRequest struct {
	RepoPath     string `json:"repo_path"`
	DocID        string `json:"doc_id"`
	Query        string `json:"query"`
	MaxResults   int    `json:"max_results"`
	ContextChars int    `json:"context_chars"`
}

type webSearchRequest struct {
	RepoPath   string `json:"repo_path"`
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	Provider   string `json:"provider"`
}

type webSearchHit struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	DisplayURL  string `json:"display_url,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
	Rank        int    `json:"rank"`
	Provider    string `json:"provider"`
	FetchedHint string `json:"fetched_hint,omitempty"`
}

type webSearchResult struct {
	Status      string                `json:"status"`
	Query       string                `json:"query"`
	Provider    string                `json:"provider,omitempty"`
	Total       int                   `json:"total"`
	Hits        []webSearchHit        `json:"hits"`
	Truncated   bool                  `json:"truncated"`
	Diagnostics []webfetch.Diagnostic `json:"diagnostics"`
}

func webPolicyForRequest(deps *Server, repoPath string, toolName string) (config.WebPolicy, error) {
	cfg, _, _, _, _ := deps.loadDeps()
	if repoPath == "" {
		return cfg.WebPolicy, nil
	}
	repoCfg, err := config.LoadRepoConfig(repoPath)
	if err != nil {
		return config.WebPolicy{}, err
	}
	if repoCfg != nil && repoCfg.ToolDenied(toolName) {
		return config.WebPolicy{}, configToolDenied(toolName)
	}
	return cfg.WebPolicy, nil
}

func configToolDenied(toolName string) error {
	return &toolDeniedError{toolName: toolName}
}

type toolDeniedError struct{ toolName string }

func (e *toolDeniedError) Error() string {
	return "tool " + e.toolName + " is denied by repo-local config"
}

func registerWebTools(srv *server.MCPServer, deps *Server) {
	registerFetchTool := func(name string) {
		srv.AddTool(basemcp.NewTool(name,
			basemcp.WithDescription("Fetch one allowed web URL into a helper-managed artifact cache and return only doc_id, metadata, hashes, completeness, cache status, and diagnostics. Full page content is not returned."),
			basemcp.WithString("url", basemcp.Required(), basemcp.Description("Absolute http/https URL to fetch.")),
			basemcp.WithString("repo_path", basemcp.Description("Optional repository root used only for repo-local tool deny policy.")),
			basemcp.WithNumber("max_source_bytes", basemcp.Description("Optional per-call source byte cap, bounded by web_policy.max_source_bytes.")),
			basemcp.WithNumber("timeout_seconds", basemcp.Description("Optional per-call timeout, bounded by web_policy.timeout_seconds.")),
		), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args webFetchRequest
			if err := bind(req, &args); err != nil {
				return nil, err
			}
			policy, err := webPolicyForRequest(deps, args.RepoPath, name)
			if err != nil {
				return safeError(deps, err), nil
			}
			client := webfetch.NewClient(policy)
			result, err := client.Fetch(ctx, webfetch.FetchRequest{URL: args.URL, MaxSourceBytes: args.MaxSourceBytes, TimeoutSeconds: args.TimeoutSeconds})
			if err != nil {
				return safeError(deps, err), nil
			}
			return structured(result)
		})
	}
	registerFetchTool("web_fetch")
	registerFetchTool("fetch_url")

	srv.AddTool(basemcp.NewTool("web_search",
		basemcp.WithDescription("Return compact web search results without fetching page bodies. Fails closed until an explicit search provider adapter is configured."),
		basemcp.WithString("query", basemcp.Required()),
		basemcp.WithString("repo_path", basemcp.Description("Optional repository root used only for repo-local tool deny policy.")),
		basemcp.WithString("provider", basemcp.Description("Search provider id. No implicit provider is used.")),
		basemcp.WithNumber("max_results", basemcp.Description("Maximum compact hits requested.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args webSearchRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if _, err := webPolicyForRequest(deps, args.RepoPath, "web_search"); err != nil {
			return safeError(deps, err), nil
		}
		result := webSearchResult{Status: "blocked", Query: args.Query, Provider: args.Provider, Hits: []webSearchHit{}, Diagnostics: []webfetch.Diagnostic{{Code: "search_provider_not_configured", Message: "web_search requires an explicit search provider adapter; no implicit external search service is used"}}}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("fetched_doc_read",
		basemcp.WithDescription("Read a bounded fragment from a fetched web document by doc_id. Returns selected content only with offsets and truncation metadata."),
		basemcp.WithString("doc_id", basemcp.Required()),
		basemcp.WithString("repo_path", basemcp.Description("Optional repository root used only for repo-local tool deny policy.")),
		basemcp.WithString("source", basemcp.Description("Artifact source: normalized (default) or raw.")),
		basemcp.WithNumber("offset", basemcp.Description("Zero-based byte offset.")),
		basemcp.WithNumber("limit", basemcp.Description("Maximum bytes to return. Defaults to 4000, capped at 20000.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args fetchedDocReadRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		policy, err := webPolicyForRequest(deps, args.RepoPath, "fetched_doc_read")
		if err != nil {
			return safeError(deps, err), nil
		}
		return structured(webfetch.Read(policy, webfetch.ReadRequest{DocID: args.DocID, Source: args.Source, Offset: args.Offset, Limit: args.Limit}))
	})

	srv.AddTool(basemcp.NewTool("fetched_doc_find",
		basemcp.WithDescription("Search the complete normalized text of a fetched web document and return bounded snippets with stable offsets."),
		basemcp.WithString("doc_id", basemcp.Required()),
		basemcp.WithString("query", basemcp.Required()),
		basemcp.WithString("repo_path", basemcp.Description("Optional repository root used only for repo-local tool deny policy.")),
		basemcp.WithNumber("max_results", basemcp.Description("Maximum matches. Defaults to 10, capped at 50.")),
		basemcp.WithNumber("context_chars", basemcp.Description("Snippet context around each match. Defaults to 80, capped at 500.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args fetchedDocFindRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		policy, err := webPolicyForRequest(deps, args.RepoPath, "fetched_doc_find")
		if err != nil {
			return safeError(deps, err), nil
		}
		return structured(webfetch.Find(policy, webfetch.FindRequest{DocID: args.DocID, Query: args.Query, MaxResults: args.MaxResults, ContextChars: args.ContextChars}))
	})
}
