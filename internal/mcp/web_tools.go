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
}
