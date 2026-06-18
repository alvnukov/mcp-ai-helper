package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/fileops"
)

func registerFileTools(srv *server.MCPServer) {
	srv.AddTool(basemcp.NewTool("read_file",
		basemcp.WithDescription("Read file content with line numbers as structured data. Prefer this over shell cat/head/tail for reading files."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("path", basemcp.Required()),
		basemcp.WithNumber("offset", basemcp.Description("Optional 1-based line number to start reading from.")),
		basemcp.WithNumber("limit", basemcp.Description("Optional maximum number of lines to return.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			RepoPath string `json:"repo_path"`
			Path     string `json:"path"`
			Offset   int    `json:"offset"`
			Limit    int    `json:"limit"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
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
	})
	srv.AddTool(basemcp.NewTool("search_files",
		basemcp.WithDescription("Search for text pattern in files under a directory. Returns structured results with file path, line number, and matched text. Prefer this over raw grep/rg for safety and structured output."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("path", basemcp.Description("Repo-relative directory to search. Defaults to repo root.")),
		basemcp.WithString("pattern", basemcp.Required()),
		basemcp.WithNumber("max_matches", basemcp.Description("Maximum total matches. Defaults to 100.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
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
			return basemcp.NewToolResultError("pattern is required"), nil
		}
		result, err := fileops.SearchFilesInRepo(args.RepoPath, args.Path, args.Pattern, args.MaxMatches)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("snapshot_file",
		basemcp.WithDescription("Read file hash/size before guarded edits."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("path", basemcp.Required()),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			RepoPath string `json:"repo_path"`
			Path     string `json:"path"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		snapshot, err := fileops.ReadSnapshotInRepo(args.RepoPath, args.Path)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(snapshot)
	})

	srv.AddTool(basemcp.NewTool("read_files",
		basemcp.WithDescription("Read multiple small repo-relative files in one call. Bounded: max 8 paths, 64 KiB per file, 128 KiB total. Missing files reported per file and do not fail the whole call."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithArray("paths", basemcp.Required(), basemcp.Description("Repo-relative file paths to read (max 8)."), basemcp.WithStringItems(), basemcp.MinItems(1), basemcp.MaxItems(8)),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			RepoPath string   `json:"repo_path"`
			Paths    []string `json:"paths"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		if len(args.Paths) == 0 {
			return basemcp.NewToolResultError("paths must not be empty"), nil
		}
		result, err := fileops.ReadFilesInRepo(args.RepoPath, args.Paths)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("apply_guarded_replace",
		basemcp.WithDescription("Replace one unique text span only if the file hash still matches. Use old_b64/new_b64 for text with characters that are hard to escape in JSON (e.g. Go raw strings with backslashes)."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("path", basemcp.Required()),
		basemcp.WithString("expected_hash", basemcp.Required()),
		basemcp.WithString("old", basemcp.Description("Text to replace. Omit when using old_b64.")),
		basemcp.WithString("old_b64", basemcp.Description("Base64-encoded old text. Use instead of old for safe transport of strings with backslashes.")),
		basemcp.WithString("new", basemcp.Description("Replacement text. Omit when using new_b64.")),
		basemcp.WithString("new_b64", basemcp.Description("Base64-encoded new text. Use instead of new for safe transport of strings with backslashes.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args fileops.ReplaceRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := fileops.ApplyGuardedReplace(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("list_dir",
		basemcp.WithDescription("Structured directory listing with name, path, type, size, modified_at."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("path", basemcp.Description("Repo-relative directory path. Defaults to repo root.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
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
	})
	srv.AddTool(basemcp.NewTool("write_file",
		basemcp.WithDescription("Write content to a file. Creates parent dirs if needed. Use content_b64 for text with backslashes or non-UTF8. Set expected_hash to guard overwrite of existing files."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("path", basemcp.Required(), basemcp.Description("Repo-relative file path.")),
		basemcp.WithString("content", basemcp.Description("File content as string. Omit when using content_b64.")),
		basemcp.WithString("content_b64", basemcp.Description("Base64-encoded content. Use instead of content for safe transport.")),
		basemcp.WithString("expected_hash", basemcp.Description("SHA-256 hash for overwrite guard. If set and file exists with different hash, returns conflict.")),
		basemcp.WithNumber("mode", basemcp.Description("File permission mode (default 0644).")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
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
	})
}
