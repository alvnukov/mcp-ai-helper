package mcp

import (
	"context"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/features"
)

type featureReadRequest struct {
	ID       string `json:"id"`
	RepoPath string `json:"repo_path"`
}

type featureWriteRequest struct {
	ID       string `json:"id"`
	Scope    string `json:"scope"`
	RepoPath string `json:"repo_path"`
	Reason   string `json:"reason"`
}

func registerFeatureTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("feature_list",
		basemcp.WithDescription("List known helper feature flags with code default, global override, repo override, effective value, and source."),
		basemcp.WithString("repo_path", basemcp.Description("Optional repository root. When provided, repo-local overrides participate in resolution.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args featureReadRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		mgr := featureManager(deps)
		items, err := mgr.List(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"features": items, "global_state_path": mgr.GlobalStatePath})
	})

	srv.AddTool(basemcp.NewTool("feature_get",
		basemcp.WithDescription("Return one helper feature flag with code default, global override, repo override, effective value, and source."),
		basemcp.WithString("id", basemcp.Required(), basemcp.Description("Known feature id.")),
		basemcp.WithString("repo_path", basemcp.Description("Optional repository root. When provided, repo-local overrides participate in resolution.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args featureReadRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		mgr := featureManager(deps)
		item, err := mgr.Get(args.ID, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"feature": item, "global_state_path": mgr.GlobalStatePath})
	})

	srv.AddTool(basemcp.NewTool("feature_enable",
		basemcp.WithDescription("Enable a known helper feature flag in scope global or repo. Repo scope requires repo_path."),
		basemcp.WithString("id", basemcp.Required(), basemcp.Description("Known feature id.")),
		basemcp.WithString("scope", basemcp.Required(), basemcp.Description("Override scope: global or repo.")),
		basemcp.WithString("repo_path", basemcp.Description("Repository root required when scope is repo.")),
		basemcp.WithString("reason", basemcp.Description("Optional compact reason stored in audit.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return setFeatureOverride(req, deps, true)
	})

	srv.AddTool(basemcp.NewTool("feature_disable",
		basemcp.WithDescription("Disable a known helper feature flag in scope global or repo. Repo scope requires repo_path."),
		basemcp.WithString("id", basemcp.Required(), basemcp.Description("Known feature id.")),
		basemcp.WithString("scope", basemcp.Required(), basemcp.Description("Override scope: global or repo.")),
		basemcp.WithString("repo_path", basemcp.Description("Repository root required when scope is repo.")),
		basemcp.WithString("reason", basemcp.Description("Optional compact reason stored in audit.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return setFeatureOverride(req, deps, false)
	})

	srv.AddTool(basemcp.NewTool("feature_reset",
		basemcp.WithDescription("Remove a global or repo-local helper feature override so lower-priority state applies again."),
		basemcp.WithString("id", basemcp.Required(), basemcp.Description("Known feature id.")),
		basemcp.WithString("scope", basemcp.Required(), basemcp.Description("Override scope: global or repo.")),
		basemcp.WithString("repo_path", basemcp.Description("Repository root required when scope is repo.")),
		basemcp.WithString("reason", basemcp.Description("Optional compact reason stored in audit.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args featureWriteRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		mgr := featureManager(deps)
		item, err := mgr.Set(strings.TrimSpace(args.Scope), args.RepoPath, args.ID, nil, args.Reason)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"status": "ok", "feature": item, "global_state_path": mgr.GlobalStatePath})
	})
}

func setFeatureOverride(req basemcp.CallToolRequest, deps *Server, enabled bool) (*basemcp.CallToolResult, error) {
	var args featureWriteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	mgr := featureManager(deps)
	item, err := mgr.Set(strings.TrimSpace(args.Scope), args.RepoPath, args.ID, &enabled, args.Reason)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"status": "ok", "feature": item, "global_state_path": mgr.GlobalStatePath})
}

func featureManager(deps *Server) *features.Manager {
	cfg, _, _, _, _ := deps.loadDeps()
	return features.NewManager(features.GlobalStatePathForConfig(cfg.SourcePath))
}
