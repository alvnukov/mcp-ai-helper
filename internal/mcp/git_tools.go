package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/gitops"
)

func registerGitTools(srv *server.MCPServer) {
	srv.AddTool(basemcp.NewTool("git_commit_owned",
		basemcp.WithDescription("Commit only explicit owned files. Never stages all files."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithArray("files", basemcp.Required()),
		basemcp.WithString("message", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.CommitRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.CommitOwned(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_prepare_task_worktree",
		basemcp.WithDescription("Create or reuse .worktrees/<task_id> on branch <task_type>/<task_id>."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("task_id", basemcp.Required()),
		basemcp.WithString("task_type", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.PrepareTaskWorktreeRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.PrepareTaskWorktree(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
}
