package mcp

import (
	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/fileops"
)

func registerFileTools(srv *server.MCPServer, deps *Server) {
	fileActions := actions{
		"read":      ignoringContext(fileActionRead),
		"read_many": ignoringContext(fileActionReadMany),
		"list":      ignoringContext(fileActionList),
		"search":    ignoringContext(fileActionSearch),
		"snapshot":  ignoringContext(fileActionSnapshot),
	}
	srv.AddTool(basemcp.NewTool("file",
		readsLocal,
		basemcp.WithDescription("Repo file reading and inspection. Required: repo_path, action. Actions: read (path, offset?, limit?) — read single file with line numbers; read_many (paths[]) — read up to 8 files in one call; list (path?) — structured directory listing; search (path?, pattern, max_matches?, regex?, ignore_case?, smart_case?, glob?[], glob_exclude?[], type?[], type_not?[], context_before?, context_after?, files_only?, invert?, no_ignore?, word_match?, line_regexp?, count_only?, only_matching?, replace?) — rg-like text search honoring .gitignore/.ignore/.rgignore (literal substring by default, regexp with regex=true; type names like go/py fold into globs; count_only reports path:count; only_matching plus replace extracts capture groups); snapshot (path) — read file hash/size before guarded edits."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Any readable directory that roots the operation; a git repository root is NOT required. Other path arguments are relative to it.")),
		basemcp.WithString("action", basemcp.Required(), actionEnum(fileActions)),
		basemcp.WithString("path", basemcp.Description("Repo-relative file or directory path. Required for read/snapshot; optional dir for list/search (defaults to repo root).")),
		basemcp.WithArray("paths", basemcp.Description("Repo-relative file paths to read (max 8). Required for read_many."), basemcp.WithStringItems(), basemcp.MinItems(1), basemcp.MaxItems(8)),
		basemcp.WithString("pattern", basemcp.Description("Search pattern. A literal substring by default; a regular expression when regex is true.")),
		basemcp.WithNumber("offset", basemcp.Description("1-based line number to start reading from (read action).")),
		basemcp.WithNumber("limit", basemcp.Description("Maximum lines to return (read action).")),
		basemcp.WithNumber("max_matches", basemcp.Description("Maximum total matches (search action). Defaults to 100.")),
		basemcp.WithBoolean("regex", basemcp.Description("Treat pattern as a regular expression (search action).")),
		basemcp.WithBoolean("ignore_case", basemcp.Description("Case-insensitive matching whatever the pattern's case (search action).")),
		basemcp.WithBoolean("smart_case", basemcp.Description("Case-insensitive while pattern has no uppercase letter, like rg -S (search action).")),
		basemcp.WithArray("glob", basemcp.Description("Include file globs, e.g. *.go or pkg/*.go; without a separator a glob matches the base name (search action)."), basemcp.WithStringItems()),
		basemcp.WithArray("glob_exclude", basemcp.Description("Exclude file globs; files matching any of them are skipped (search action)."), basemcp.WithStringItems()),
		basemcp.WithNumber("context_before", basemcp.Description("Non-matching lines before each match, marked with '-' (search action).")),
		basemcp.WithNumber("context_after", basemcp.Description("Non-matching lines after each match, marked with '-' (search action).")),
		basemcp.WithBoolean("files_only", basemcp.Description("Return only the paths of files with matches, like rg -l (search action).")),
		basemcp.WithBoolean("invert", basemcp.Description("Report lines the pattern does not match, like rg -v (search action).")),
		basemcp.WithBoolean("no_ignore", basemcp.Description("Search files the ignore cascade would drop: .gitignore/.ignore/.rgignore stop applying (search action).")),
		basemcp.WithArray("type", basemcp.Description("Include file types by name, e.g. go, py, md; folds into glob (search action)."), basemcp.WithStringItems()),
		basemcp.WithArray("type_not", basemcp.Description("Exclude file types by name; folds into glob_exclude (search action)."), basemcp.WithStringItems()),
		basemcp.WithBoolean("word_match", basemcp.Description("Match only at word boundaries, like rg -w (search action).")),
		basemcp.WithBoolean("line_regexp", basemcp.Description("Match whole lines only, like rg -x (search action).")),
		basemcp.WithBoolean("count_only", basemcp.Description("Per-file match counts as path:count, like rg -c (search action).")),
		basemcp.WithBoolean("only_matching", basemcp.Description("Report each match's own text, not its whole line, like rg -o (search action).")),
		basemcp.WithString("replace", basemcp.Description("Rewrite matched text with this template, $1 for capture groups, like rg -r (search action).")),
	), dispatch(deps, "file", fileActions))

	editActions := actions{
		"replace": ignoringContext(editActionReplace),
		"write":   ignoringContext(editActionWrite),
	}
	srv.AddTool(basemcp.NewTool("edit",
		rewritesLocal,
		basemcp.WithDescription("Repo file writing and guarded replacement. Required: repo_path, action. Actions: replace (path, expected_hash, old|old_b64, new|new_b64) — replace one unique text span only if file hash matches; write (path, content|content_b64, expected_hash?, mode?) — write content to a file, creating parent dirs if needed."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Any readable directory that roots the operation; a git repository root is NOT required. Other path arguments are relative to it.")),
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
	), dispatch(deps, "edit", editActions))
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
		RepoPath      string   `json:"repo_path"`
		Path          string   `json:"path"`
		Pattern       string   `json:"pattern"`
		MaxMatches    int      `json:"max_matches"`
		Regex         bool     `json:"regex"`
		IgnoreCase    bool     `json:"ignore_case"`
		SmartCase     bool     `json:"smart_case"`
		Glob          []string `json:"glob"`
		GlobExclude   []string `json:"glob_exclude"`
		ContextBefore int      `json:"context_before"`
		ContextAfter  int      `json:"context_after"`
		FilesOnly     bool     `json:"files_only"`
		Invert        bool     `json:"invert"`
		NoIgnore      bool     `json:"no_ignore"`
		Type          []string `json:"type"`
		TypeNot       []string `json:"type_not"`
		WordMatch     bool     `json:"word_match"`
		LineRegexp    bool     `json:"line_regexp"`
		CountOnly     bool     `json:"count_only"`
		OnlyMatching  bool     `json:"only_matching"`
		Replace       string   `json:"replace"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Pattern == "" {
		return basemcp.NewToolResultError("file action=search requires pattern"), nil
	}
	result, err := fileops.SearchFilesInRepoWithOptions(args.RepoPath, args.Path, fileops.SearchOptions{
		Pattern:       args.Pattern,
		Regex:         args.Regex,
		IgnoreCase:    args.IgnoreCase,
		SmartCase:     args.SmartCase,
		Glob:          args.Glob,
		GlobExclude:   args.GlobExclude,
		ContextBefore: args.ContextBefore,
		ContextAfter:  args.ContextAfter,
		FilesOnly:     args.FilesOnly,
		Invert:        args.Invert,
		NoIgnore:      args.NoIgnore,
		Type:          args.Type,
		TypeNot:       args.TypeNot,
		WordMatch:     args.WordMatch,
		LineRegexp:    args.LineRegexp,
		CountOnly:     args.CountOnly,
		OnlyMatching:  args.OnlyMatching,
		Replace:       args.Replace,
		MaxMatches:    args.MaxMatches,
	})
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
