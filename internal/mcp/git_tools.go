package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/gitops"
)

// gitAdvancedActions are the ones behind the git_advanced layer. They all reach
// gitActionAdvanced, so the only thing that varies is the gate and the name.
var gitAdvancedActions = []string{"log", "log_diff", "stash_list", "branch_list", "remote_list", "tag_list", "blame", "prepare_task_worktree"}

func registerGitTools(srv *server.MCPServer, deps *Server) {
	gitActions := actions{
		"status": gitActionStatus,
		"diff":   gitActionDiff,
		"commit": gitActionCommit,
	}
	for _, name := range gitAdvancedActions {
		gitActions[name] = gitAdvancedAction(name, deps)
	}
	srv.AddTool(basemcp.NewTool("git",
		basemcp.WithDescription("Git operations. Required: repo_path, action. Actions (always available): status — structured git status; diff (cached?, path?) — structured git diff; commit (files[], message) — commit only explicit owned files. Actions (require git_advanced layer): log (limit?, path?, author?, since?, until?, grep?) — git log; log_diff (hash) — show commit details; stash_list — git stash list; branch_list (all?) — git branch list; remote_list — git remote list; tag_list (pattern?) — git tag list; blame (file) — git blame; prepare_task_worktree (task_id, task_type) — create or reuse .worktrees/<task_id>."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("action", basemcp.Required(), actionEnum(gitActions)),
		basemcp.WithArray("files", basemcp.Description("Files to commit (commit action).")),
		basemcp.WithString("message", basemcp.Description("Commit message (commit action).")),
		basemcp.WithBoolean("cached", basemcp.Description("Show staged changes instead of working tree (diff action).")),
		basemcp.WithString("path", basemcp.Description("Optional file path filter (diff/log actions).")),
		basemcp.WithNumber("limit", basemcp.Description("Max commits to return (log action, default 20).")),
		basemcp.WithString("author", basemcp.Description("Optional author filter (log action).")),
		basemcp.WithString("since", basemcp.Description("Optional since date filter (log action).")),
		basemcp.WithString("until", basemcp.Description("Optional until date filter (log action).")),
		basemcp.WithString("grep", basemcp.Description("Optional message grep filter (log action).")),
		basemcp.WithString("hash", basemcp.Description("Commit hash (log_diff action).")),
		basemcp.WithBoolean("all", basemcp.Description("Include remote branches (branch_list action).")),
		basemcp.WithString("pattern", basemcp.Description("Optional glob pattern filter (tag_list action).")),
		basemcp.WithString("file", basemcp.Description("File path for blame (blame action).")),
		basemcp.WithString("task_id", basemcp.Description("Task ID for worktree (prepare_task_worktree action).")),
		basemcp.WithString("task_type", basemcp.Description("Branch type for worktree (prepare_task_worktree action).")),
	), dispatch("git", gitActions))
}

func gitAdvancedAction(action string, deps *Server) actionHandler {
	return func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		if deps != nil && deps.cfg != nil && !deps.cfg.LayerEnabled("git_advanced") {
			return basemcp.NewToolResultError("git action=" + action + " requires git_advanced layer to be enabled"), nil
		}
		return gitActionAdvanced(ctx, req, action)
	}
}

func gitActionStatus(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args gitops.StatusRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := gitops.Status(ctx, args)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func gitActionDiff(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args gitops.DiffRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := gitops.Diff(ctx, args)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func gitActionCommit(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	var args gitops.CommitRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := gitops.CommitOwned(ctx, args)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func gitActionAdvanced(ctx context.Context, req basemcp.CallToolRequest, action string) (*basemcp.CallToolResult, error) {
	switch action {
	case "log":
		var args gitops.LogRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.Log(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "log_diff":
		var args gitops.LogDiffRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.LogDiff(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "stash_list":
		var args gitops.StashRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.StashList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "branch_list":
		var args gitops.BranchRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.BranchList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "remote_list":
		var args gitops.RemoteRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.RemoteList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "tag_list":
		var args gitops.TagRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.TagList(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "blame":
		var args gitops.BlameRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.Blame(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	case "prepare_task_worktree":
		var args gitops.PrepareTaskWorktreeRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := gitops.PrepareTaskWorktree(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	default:
		return basemcp.NewToolResultError("git: unknown advanced action: " + action), nil
	}
}
