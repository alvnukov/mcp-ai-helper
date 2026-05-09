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
}
