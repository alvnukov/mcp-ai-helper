package mcp

import (
	"context"
	"fmt"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/notes"
)

type noteRequest struct {
	RepoPath   string   `json:"repo_path"`
	Scope      string   `json:"scope"`
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Tags       []string `json:"tags"`
	Tag        string   `json:"tag"`
	Query      string   `json:"query"`
	MaxResults int      `json:"max_results"`
}

func registerNoteTools(srv *server.MCPServer, deps *Server) {
	noteActions := actions{
		"add":    withDeps(noteActionAdd, deps),
		"list":   withDeps(noteActionList, deps),
		"read":   withDeps(noteActionRead, deps),
		"search": withDeps(noteActionSearch, deps),
		"update": withDeps(noteActionUpdate, deps),
		"delete": withDeps(noteActionDelete, deps),
	}
	srv.AddTool(basemcp.NewTool("note",
		rewritesLocal,
		basemcp.WithDescription("Your long-term memory, persisted between sessions. Write facts, decisions, gotchas and working state, then consult it any time — list or search — instead of re-deriving or guessing what a past session already worked out. Required: action. Actions: add (title, body, tags?, scope?) — create a note, get its id; list (tag?, scope?) — summaries newest first, no bodies; read (id, scope?) — one full note; search (query, max_results?, scope?) — case-insensitive matches with snippets and offsets; update (id, title?, body?, tags?, scope?) — replace the given fields; delete (id, scope?) — remove one note. scope: repo (default) keeps notes under <repo_path>/.mcp-ai-helper/notes, committable with the project; global keeps them in the helper data root, shared across repos. Use global for cross-repo lessons, repo for project state."),
		basemcp.WithString("repo_path", basemcp.Description("Repository root. Required for scope repo; ignored for scope global.")),
		basemcp.WithString("action", basemcp.Required(), actionEnum(noteActions)),
		basemcp.WithString("scope", basemcp.Description("Which notebook: repo (default) or global."), basemcp.Enum("repo", "global")),
		basemcp.WithString("id", basemcp.Description("Note id from add/list/search (read, update, delete).")),
		basemcp.WithString("title", basemcp.Description("Note title (add; update replaces it).")),
		basemcp.WithString("body", basemcp.Description("Note body markdown (add; update replaces it).")),
		basemcp.WithArray("tags", basemcp.Description("Note tags (add; update replaces the whole list, [] clears)."), basemcp.WithStringItems()),
		basemcp.WithString("tag", basemcp.Description("Keep only notes carrying this tag (list).")),
		basemcp.WithString("query", basemcp.Description("Case-insensitive substring to find in title and body (search).")),
		basemcp.WithNumber("max_results", basemcp.Description("Maximum search matches: default 10, cap 50 (search).")),
	), dispatch(deps, "note", noteActions))
}

func noteScope(raw string) (notes.Scope, error) {
	switch strings.TrimSpace(raw) {
	case "", "repo":
		return notes.ScopeRepo, nil
	case "global":
		return notes.ScopeGlobal, nil
	default:
		return "", fmt.Errorf("scope must be repo or global, got %q", raw)
	}
}

func (s *Server) notebook() (*notes.Store, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.notesStore == nil {
		return nil, fmt.Errorf("notes store is not initialised")
	}
	return s.notesStore, nil
}

func noteActionAdd(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args noteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	scope, err := noteScope(args.Scope)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	store, err := deps.notebook()
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	note, err := store.Add(scope, args.RepoPath, args.Title, args.Body, args.Tags)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"status": "ok", "note": note})
}

func noteActionList(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args noteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	scope, err := noteScope(args.Scope)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	store, err := deps.notebook()
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	summaries, skipped, err := store.List(scope, args.RepoPath, args.Tag)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"notes": summaries, "skipped_unparsable": skipped, "scope": string(scope)})
}

func noteActionRead(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args noteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.ID) == "" {
		return basemcp.NewToolResultError("id is required (note action=read)"), nil
	}
	scope, err := noteScope(args.Scope)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	store, err := deps.notebook()
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	note, err := store.Get(scope, args.RepoPath, args.ID)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"note": note})
}

func noteActionSearch(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args noteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	scope, err := noteScope(args.Scope)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	store, err := deps.notebook()
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	matches, err := store.Search(scope, args.RepoPath, args.Query, args.MaxResults)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"matches": matches, "scope": string(scope)})
}

func noteActionUpdate(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args noteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.ID) == "" {
		return basemcp.NewToolResultError("id is required (note action=update)"), nil
	}
	scope, err := noteScope(args.Scope)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	store, err := deps.notebook()
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	var fields notes.UpdateFields
	if args.Title != "" {
		title := args.Title
		fields.Title = &title
	}
	if args.Body != "" {
		body := args.Body
		fields.Body = &body
	}
	if args.Tags != nil {
		fields.Tags = args.Tags
	}
	note, err := store.Update(scope, args.RepoPath, args.ID, fields)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"status": "ok", "note": note})
}

func noteActionDelete(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args noteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.ID) == "" {
		return basemcp.NewToolResultError("id is required (note action=delete)"), nil
	}
	scope, err := noteScope(args.Scope)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	store, err := deps.notebook()
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if err := store.Delete(scope, args.RepoPath, args.ID); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(map[string]any{"status": "ok", "deleted": strings.TrimSpace(args.ID), "scope": string(scope)})
}
