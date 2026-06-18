package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/gitops"
)

func registerGitTools(srv *server.MCPServer) {
	srv.AddTool(basemcp.NewTool("git_status",
		basemcp.WithDescription("Structured git status: branch, staged, modified, untracked, ahead/behind."),
		basemcp.WithString("repo_path", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.StatusRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.Status(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_diff",
		basemcp.WithDescription("Structured git diff: files, hunks, lines. Set cached=true for staged changes."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithBoolean("cached", basemcp.Description("Show staged changes instead of working tree.")),
		basemcp.WithString("path", basemcp.Description("Optional single file to diff.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.DiffRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.Diff(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_log",
		basemcp.WithDescription("Structured git log: hash, author, date, message per commit."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithNumber("limit", basemcp.Description("Max commits to return (default 20).")),
		basemcp.WithString("path", basemcp.Description("Optional file path filter.")),
		basemcp.WithString("author", basemcp.Description("Optional author filter.")),
		basemcp.WithString("since", basemcp.Description("Optional since date filter.")),
		basemcp.WithString("until", basemcp.Description("Optional until date filter.")),
		basemcp.WithString("grep", basemcp.Description("Optional message grep filter.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.LogRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.Log(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_log_diff",
		basemcp.WithDescription("Structured git show: commit details with files, hunks, stats."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("hash", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.LogDiffRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.LogDiff(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_stash_list",
		basemcp.WithDescription("Structured git stash list: index, hash, message."),
		basemcp.WithString("repo_path", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.StashRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.StashList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_branch_list",
		basemcp.WithDescription("Structured git branch list: name, hash, current, remote."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithBoolean("all", basemcp.Description("Include remote branches.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.BranchRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.BranchList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_remote_list",
		basemcp.WithDescription("Structured git remote list: name, url."),
		basemcp.WithString("repo_path", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.RemoteRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.RemoteList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_tag_list",
		basemcp.WithDescription("Structured git tag list: name, hash, annotated, message."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("pattern", basemcp.Description("Optional glob pattern filter.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.TagRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.TagList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("git_blame",
		basemcp.WithDescription("Structured git blame: hash, author, date, line, content per line."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("file", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args gitops.BlameRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.Blame(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
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
