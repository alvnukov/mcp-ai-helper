package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/language"
)

func registerLanguageTools(srv *server.MCPServer) {
	languageRegistry := language.DefaultRegistry()
	srv.AddTool(basemcp.NewTool("language_profiles",
		basemcp.WithDescription("List built-in language profiles with formatter, test, static-check, and guardrail recommendations."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(map[string]any{"profiles": languageRegistry.List()})
	})
	srv.AddTool(basemcp.NewTool("language_detect",
		basemcp.WithDescription("Detect language profiles for repo-relative paths before building an edit/test workflow."),
		basemcp.WithArray("paths", basemcp.Description("Repo-relative file paths.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			Paths []string `json:"paths"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"profiles": languageRegistry.Detect(args.Paths)})
	})
}
