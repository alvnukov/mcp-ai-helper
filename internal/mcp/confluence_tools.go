package mcp

import (
	"context"
	"fmt"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/confluence"
)

type confRequest struct {
	PageID          string `json:"page_id"`
	ExpectedVersion int    `json:"expected_version"`
	SpaceKey        string `json:"space_key"`
	Title           string `json:"title"`
	Body            string `json:"body"`
	Old             string `json:"old"`
	New             string `json:"new"`
	ParentID        string `json:"parent_id"`
	VersionMessage  string `json:"version_message"`
	MinorEdit       bool   `json:"minor_edit"`
	CQL             string `json:"cql"`
	MaxResults      int    `json:"max_results"`
}

func checkConfSpace(deps *Server, spaceKey string) bool {
	cfg, _, _, _, _ := deps.loadDeps()
	if cfg.Integrations.Confluence == nil {
		return false
	}
	return cfg.Integrations.Confluence.IsSpaceAllowed(spaceKey)
}

func registerConfluenceTools(srv *server.MCPServer, deps *Server) {
	confluenceActions := actions{
		"search": withDeps(confActionSearch, deps),
		"read":   withDeps(confActionRead, deps),
		"spaces": withDeps(confActionSpaces, deps),
		"update": withDeps(confActionUpdate, deps),
		"create": withDeps(confActionCreate, deps),
		"delete": withDeps(confActionDelete, deps),
	}

	srv.AddTool(basemcp.NewTool("confluence",
		rewritesRemote,
		basemcp.WithDescription("Interact with Confluence. Required: action. Actions: search (cql, max_results?) — search pages by CQL; read (page_id) — the page with its body and current version; spaces () — list all spaces; update (page_id, expected_version, body|old+new, title?, version_message?, minor_edit?) — replace the whole page body or one unique span of it; create (space_key, title, body?, parent_id?) — add a page; delete (page_id, expected_version) — remove one. For updates, read the page first and pass back the version that read reported: every edit is refused unless the page is still at that version, so a page someone else changed meanwhile is never overwritten. Editing is refused entirely when the integration is read_only or the space is outside allowed_spaces; reading is not."),
		basemcp.WithString("action", basemcp.Required(), actionEnum(confluenceActions)),
		basemcp.WithString("page_id", basemcp.Description("Confluence page ID (read, update, delete; required).")),
		basemcp.WithNumber("expected_version", basemcp.Description("Version the read action reported for the page (update, delete; required). The edit is refused if the page has moved past it.")),
		basemcp.WithString("space_key", basemcp.Description("Space key to create the page in (create; required).")),
		basemcp.WithString("title", basemcp.Description("Page title. Required for create; on update it renames the page, and omitting it keeps the current title.")),
		basemcp.WithString("body", basemcp.Description("Whole page body in Confluence storage format, the XHTML the read action returns (create; update when replacing everything). On update, omit when using old and new.")),
		basemcp.WithString("old", basemcp.Description("Span of the current body to replace (update). It must appear exactly once, as in edit action=replace.")),
		basemcp.WithString("new", basemcp.Description("Text to put in place of old (update). Empty deletes the span.")),
		basemcp.WithString("parent_id", basemcp.Description("Page ID to create the new page under (create).")),
		basemcp.WithString("version_message", basemcp.Description("Note recorded beside the new version in page history (update).")),
		basemcp.WithBoolean("minor_edit", basemcp.Description("Record the new version as a minor edit, which does not notify watchers (update).")),
		basemcp.WithString("cql", basemcp.Description("CQL query string, e.g. 'title ~ kubernetes' (search; required).")),
		basemcp.WithNumber("max_results", basemcp.Description("Maximum results. Defaults to 20 (search).")),
	), dispatch(deps, "confluence", confluenceActions))
}

// confWritesAllowed reports why an edit must not happen, or nil when it may.
//
// Naming the reason matters more for a write than for a read: read_only and an
// unlisted space are both single lines of config, and a caller told only that
// the edit was "not allowed" cannot tell its user which line to change.
func confWritesAllowed(deps *Server) error {
	cfg, _, _, _, _ := deps.loadDeps()
	if cfg.Integrations.Confluence == nil {
		return fmt.Errorf("confluence: no integration configured")
	}
	if !cfg.Integrations.Confluence.CanMutate() {
		return fmt.Errorf("confluence: integration is read_only; set integrations.confluence.read_only to false to allow edits")
	}
	return nil
}

// confReadPage reads one page and refuses a space outside the allowlist.
func confReadPage(ctx context.Context, deps *Server, pageID string) (*confluence.PageInfo, *basemcp.CallToolResult) {
	if pageID == "" {
		return nil, basemcp.NewToolResultError("confluence: page_id is required")
	}
	client, err := deps.getConfluenceClient()
	if err != nil {
		return nil, safeError(deps, err)
	}
	page, err := client.GetContentByIDContext(ctx, pageID)
	if err != nil {
		return nil, safeError(deps, err)
	}
	if !checkConfSpace(deps, page.Space) {
		return nil, safeError(deps, fmt.Errorf("confluence: space %q not in allowed_spaces", page.Space))
	}
	return page, nil
}

// confPageForEdit reads the page an edit names and puts it through both gates:
// the integration must allow writes at all, and the page's space must be one a
// read would have answered for.
func confPageForEdit(ctx context.Context, deps *Server, pageID string) (*confluence.Client, *confluence.PageInfo, *basemcp.CallToolResult) {
	if err := confWritesAllowed(deps); err != nil {
		return nil, nil, safeError(deps, err)
	}
	page, refusal := confReadPage(ctx, deps, pageID)
	if refusal != nil {
		return nil, nil, refusal
	}
	client, err := deps.getConfluenceClient()
	if err != nil {
		return nil, nil, safeError(deps, err)
	}
	return client, page, nil
}

func confActionSearch(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args confRequest
	if err := bind(req, &args); err != nil {
		return nil, err
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 20
	}
	client, err := deps.getConfluenceClient()
	if err != nil {
		return safeError(deps, err), nil
	}
	results, err := client.SearchContext(ctx, args.CQL, args.MaxResults)
	if err != nil {
		return safeError(deps, err), nil
	}
	return structured(map[string]any{"total": len(results), "results": results})
}

func confActionRead(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args confRequest
	if err := bind(req, &args); err != nil {
		return nil, err
	}
	page, refusal := confReadPage(ctx, deps, args.PageID)
	if refusal != nil {
		return refusal, nil
	}
	return structured(map[string]any{"page": page})
}

func confActionSpaces(ctx context.Context, _ basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	client, err := deps.getConfluenceClient()
	if err != nil {
		return safeError(deps, err), nil
	}
	spaces, err := client.GetSpacesContext(ctx)
	if err != nil {
		return safeError(deps, err), nil
	}
	return structured(map[string]any{"total": len(spaces), "spaces": spaces})
}

func confActionUpdate(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args confRequest
	if err := bind(req, &args); err != nil {
		return nil, err
	}
	client, page, refusal := confPageForEdit(ctx, deps, args.PageID)
	if refusal != nil {
		return refusal, nil
	}
	result, err := client.UpdatePageContext(ctx, page, confluence.UpdateRequest{
		PageID:          args.PageID,
		ExpectedVersion: args.ExpectedVersion,
		Title:           args.Title,
		Body:            args.Body,
		Old:             args.Old,
		New:             args.New,
		VersionMessage:  args.VersionMessage,
		MinorEdit:       args.MinorEdit,
	})
	if err != nil {
		return safeError(deps, err), nil
	}
	return structured(result)
}

func confActionCreate(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args confRequest
	if err := bind(req, &args); err != nil {
		return nil, err
	}
	if err := confWritesAllowed(deps); err != nil {
		return safeError(deps, err), nil
	}
	if !checkConfSpace(deps, args.SpaceKey) {
		return safeError(deps, fmt.Errorf("confluence: space %q not in allowed_spaces", args.SpaceKey)), nil
	}
	client, err := deps.getConfluenceClient()
	if err != nil {
		return safeError(deps, err), nil
	}
	result, err := client.CreatePageContext(ctx, confluence.CreateRequest{
		SpaceKey: args.SpaceKey,
		Title:    args.Title,
		Body:     args.Body,
		ParentID: args.ParentID,
	})
	if err != nil {
		return safeError(deps, err), nil
	}
	return structured(result)
}

func confActionDelete(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args confRequest
	if err := bind(req, &args); err != nil {
		return nil, err
	}
	client, page, refusal := confPageForEdit(ctx, deps, args.PageID)
	if refusal != nil {
		return refusal, nil
	}
	result, err := client.DeletePageContext(ctx, page, args.ExpectedVersion)
	if err != nil {
		return safeError(deps, err), nil
	}
	return structured(result)
}
