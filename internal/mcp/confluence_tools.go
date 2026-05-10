package mcp

import (
	"context"
	"fmt"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/confluence"
)

type confSearchRequest struct {
	CQL        string `json:"cql"`
	MaxResults int    `json:"max_results"`
}

type confReadRequest struct {
	PageID string `json:"page_id"`
}

func checkConfSpace(deps *Server, spaceKey string) bool {
	cfg, _, _, _, _ := deps.loadDeps()
	if cfg.Integrations.Confluence == nil { return false }
	return cfg.Integrations.Confluence.IsSpaceAllowed(spaceKey)
}

func registerConfluenceTools(srv *server.MCPServer, deps *Server) {
	getClient := func() (*confluence.Client, error) {
		return deps.getConfluenceClient()
	}

	srv.AddTool(basemcp.NewTool("conf_search",
		basemcp.WithDescription("Search Confluence pages by CQL (Confluence Query Language)."),
		basemcp.WithString("cql", basemcp.Required(), basemcp.Description("CQL query string, e.g. 'title ~ kubernetes'.")),
		basemcp.WithNumber("max_results", basemcp.Description("Maximum results. Defaults to 20.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args confSearchRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if args.MaxResults <= 0 {
			args.MaxResults = 20
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		results, err := jc.Search(args.CQL, args.MaxResults)
		if err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"total": len(results), "results": results})
	})

	srv.AddTool(basemcp.NewTool("conf_read",
		basemcp.WithDescription("Read a Confluence page by ID, including content body and version."),
		basemcp.WithString("page_id", basemcp.Required(), basemcp.Description("Confluence page ID (numeric string).")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args confReadRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		page, err := jc.GetContentByID(args.PageID)
		if err != nil {
			return safeError(deps, err), nil
		}
		if !checkConfSpace(deps, page.Space) { return safeError(deps, fmt.Errorf("confluence: space %q not in allowed_spaces", page.Space)), nil }
		return structured(map[string]any{"page": page})
	})

	srv.AddTool(basemcp.NewTool("conf_spaces",
		basemcp.WithDescription("List all Confluence spaces."),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		spaces, err := jc.GetSpaces()
		if err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"total": len(spaces), "spaces": spaces})
	})
}
