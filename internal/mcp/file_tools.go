package mcp

import (
	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/fileops"
)

func registerFileTools(srv *server.MCPServer) {
	fileActions := actions{
		"read":      ignoringContext(fileActionRead),
		"read_many": ignoringContext(fileActionReadMany),
		"list":      ignoringContext(fileActionList),
		"search":    ignoringContext(fileActionSearch),
		"snapshot":  ignoringContext(fileActionSnapshot),
	}
	srv.AddTool(basemcp.NewTool("file",
		basemcp.WithDescription("Repo file reading and inspection. Required: repo_path, action. Actions: read (path, offset?, limit?) — read single file with line numbers; read_many (paths[]) — read up to 8 files in one call; list (path?) — structured directory listing; search (path?, pattern, max_matches?) — search text in files under a directory; snapshot (path) — read file hash/size before guarded edits."),
		basemcp.WithReadOnlyHintAnnotation(true),
		basemcp.WithDestructiveHintAnnotation(false),
		basemcp.WithIdempotentHintAnnotation(true),
		basemcp.WithOpenWorldHintAnnotation(false),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("action", basemcp.Required(), actionEnum(fileActions)),
		basemcp.WithString("path", basemcp.Description("Repo-relative file or directory path. Required for read/snapshot; optional dir for list/search (defaults to repo root).")),
		basemcp.WithArray("paths", basemcp.Description("Repo-relative file paths to read (max 8). Required for read_many."), basemcp.WithStringItems(), basemcp.MinItems(1), basemcp.MaxItems(8)),
		basemcp.WithString("pattern", basemcp.Description("Search pattern. Required for search.")),
		basemcp.WithNumber("offset", basemcp.Description("1-based line number to start reading from (read action).")),
		basemcp.WithNumber("limit", basemcp.Description("Maximum lines to return (read action).")),
		basemcp.WithNumber("max_matches", basemcp.Description("Maximum total matches (search action). Defaults to 100.")),
	), dispatch("file", fileActions))

	editActions := actions{
		"replace": ignoringContext(editActionReplace),
		"write":   ignoringContext(editActionWrite),
	}
	srv.AddTool(basemcp.NewTool("edit",
		basemcp.WithDescription("Repo file writing and guarded replacement. Required: repo_path, action. Actions: replace (path, expected_hash, old|old_b64, new|new_b64) — replace one unique text span only if file hash matches; write (path, content|content_b64, expected_hash?, mode?) — write content to a file, creating parent dirs if needed."),
		basemcp.WithDestructiveHintAnnotation(true),
		basemcp.WithIdempotentHintAnnotation(false),
		basemcp.WithOpenWorldHintAnnotation(false),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("action", basemcp.Required(), actionEnum(editActions)),
		basemcp.WithString("path", basemcp.Required(), basemcp.Description("Repo-relative file path.")),
		basemcp.WithString("expected_hash", basemcp.Description("SHA-256 hash guard. Required for replace; optional overwrite guard for write.")),
		basemcp.WithString("old", basemcp.Description("Text to replace (replace action). Omit when using old_b64.")),
		basemcp.WithString("old_b64", basemcp.Description("Base64-encoded old text (replace action). Use instead of old for safe transport.")),
		basemcp.WithString("new", basemcp.Description("Replacement text (replace action). Omit when using new_b64.")),
		basemcp.WithString("new_b64", basemcp.Description("Base64-encoded new text (replace action). Use instead of new for safe transport.")),
		basemcp.WithString("content", basemcp.Description("File content as string (write action). Omit when using content_b64.")),
		basemcp.WithString("content_b64", basemcp.Description("Base64-encoded content (write action). Use instead of content for safe transport.")),
		basemcp.WithNumber("mode", basemcp.Description("File permission mode (write action, default 0644).")),
	), dispatch("edit", editActions))
}

func fileActionRead(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Path     string `json:"path"`
		Offset   int    `json:"offset"`
		Limit    int    `json:"limit"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Path == "" {
		return basemcp.NewToolResultError("file action=read requires path"), nil
	}
	fc, err := fileops.ReadFileContentInRepo(args.RepoPath, args.Path)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Offset > 0 || args.Limit > 0 {
		start := args.Offset - 1
		if start < 0 {
			start = 0
		}
		if start < len(fc.Lines) {
			end := len(fc.Lines)
			if args.Limit > 0 {
				if start+args.Limit < end {
					end = start + args.Limit
				}
			}
			fc.Lines = fc.Lines[start:end]
		}
	}
	return structured(fc)
}

func fileActionReadMany(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath string   `json:"repo_path"`
		Paths    []string `json:"paths"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if len(args.Paths) == 0 {
		return basemcp.NewToolResultError("file action=read_many requires paths"), nil
	}
	result, err := fileops.ReadFilesInRepo(args.RepoPath, args.Paths)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func fileActionList(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Path     string `json:"path"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := fileops.ListDir(fileops.ListDirRequest{RepoPath: args.RepoPath, Path: args.Path})
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func fileActionSearch(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath   string `json:"repo_path"`
		Path       string `json:"path"`
		Pattern    string `json:"pattern"`
		MaxMatches int    `json:"max_matches"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Pattern == "" {
		return basemcp.NewToolResultError("file action=search requires pattern"), nil
	}
	result, err := fileops.SearchFilesInRepo(args.RepoPath, args.Path, args.Pattern, args.MaxMatches)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func fileActionSnapshot(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
		Path     string `json:"path"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Path == "" {
		return basemcp.NewToolResultError("file action=snapshot requires path"), nil
	}
	snapshot, err := fileops.ReadSnapshotInRepo(args.RepoPath, args.Path)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(snapshot)
}

func editActionReplace(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args fileops.ReplaceRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Path == "" {
		return basemcp.NewToolResultError("edit action=replace requires path"), nil
	}
	if args.ExpectedHash == "" {
		return basemcp.NewToolResultError("edit action=replace requires expected_hash"), nil
	}
	result, err := fileops.ApplyGuardedReplace(args)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func editActionWrite(req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath     string `json:"repo_path"`
		Path         string `json:"path"`
		Content      string `json:"content"`
		ContentB64   string `json:"content_b64"`
		ExpectedHash string `json:"expected_hash"`
		Mode         int    `json:"mode"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Path == "" {
		return basemcp.NewToolResultError("edit action=write requires path"), nil
	}
	result, err := fileops.WriteFile(fileops.WriteFileRequest{
		RepoPath:     args.RepoPath,
		Path:         args.Path,
		Content:      args.Content,
		ContentB64:   args.ContentB64,
		ExpectedHash: args.ExpectedHash,
		Mode:         args.Mode,
	})
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}
