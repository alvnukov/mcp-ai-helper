package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/pipeline"
)

func registerPipelineTools(srv *server.MCPServer, deps *Server) {
	// Workflow step schema for discoverability.
	srv.AddTool(basemcp.NewTool("workflow_schema",
		basemcp.WithDescription("Return valid step types and their parameters for run_workflow steps."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(map[string]any{
			"common_step_fields": map[string]string{
				"depends_on": "Optional array of step IDs this step depends on. Engine auto-detects same-file dependencies; use this for cross-file or cross-tool ordering.",
				"if":         "Condition: 'always' (default), &&, ||, !, changed_files_count comparisons, changed_files contains path, steps.<id>.status/exit_code/validation comparisons, steps.<id>.output_contains text, file_exists/file_missing path, or tasks.<id>.status comparisons.",
				"on_failure": "Optional: 'stop' (default) or 'continue'.",
			},
			"step_types": []map[string]any{
				{
					"type":        "command",
					"description": "Run a shell command. Workflow stops on non-zero exit unless on_failure is 'continue'.",
					"fields": map[string]string{
						"command":    "Shell command to run (string, required).",
						"cwd":        "Optional repo-relative working directory (string).",
						"on_failure": "Optional: 'stop' (default) or 'continue'.",
					},
				},
				{
					"type":        "guarded_replace",
					"description": "Replace one unique text span only if the file hash still matches. Use read_file first, then snapshot_file, then this.",
					"fields": map[string]string{
						"path":          "Repo-relative file path (string, required).",
						"expected_hash": "SHA-256 hash from snapshot_file before edit (string, required).",
						"old":           "Text to replace. Use old_b64 for strings with backslashes (string, either old or old_b64 required).",
						"old_b64":       "Base64-encoded old text. Safer for strings with backslashes (string).",
						"new":           "Replacement text. Use new_b64 for strings with backslashes (string).",
						"new_b64":       "Base64-encoded new text (string).",
					},
				},
				{
					"type":        "task_batch_upsert",
					"description": "Synchronize per-repository task state.",
					"fields": map[string]string{
						"tasks":         "Array of task objects with id, title, status, priority, model_level, tags, body (required).",
						"close_missing": "Close active tasks not in this batch (boolean).",
					},
				},
				{
					"type":        "task_transition",
					"description": "Guardedly transition task statuses inside a workflow.",
					"fields": map[string]string{
						"task_ids": "Task IDs to transition (array of strings, required).",
						"from":     "Optional required current status for every task.",
						"to":       "Target status (string, required).",
					},
				},
				{
					"type":        "git_commit_owned",
					"description": "Commit only explicit owned files. Never stages all files.",
					"fields": map[string]string{
						"files":   "Repo-relative files to commit (array of strings, required).",
						"message": "Commit message (string, required).",
					},
				},
				{
					"type":        "git_prepare_task_worktree",
					"description": "Create or reuse .worktrees/<task_id> on branch <task_type>/<task_id>.",
					"fields": map[string]string{
						"task_id":   "Task id, e.g. task-057 (string, required).",
						"task_type": "Branch type, e.g. feature, bug, hotfix, chore, docs, refactor, test, ci (string, required).",
					},
				},
			},
		})
	})

	srv.AddTool(basemcp.NewTool("run_pipeline",
		basemcp.WithDescription("Run command -> evidence extraction -> optional model analysis -> evidence validation."),
		basemcp.WithString("command", basemcp.Required()),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("cwd", basemcp.Description("Optional repo-relative working directory.")),
		basemcp.WithNumber("timeout_seconds", basemcp.Description("Optional command timeout in seconds.")),
		basemcp.WithString("current_task_id", basemcp.Description("Optional task id to update during pipeline execution.")),
		basemcp.WithString("task_on_start", basemcp.Description("Optional status for current_task_id before executing command; defaults to in_progress.")),
		basemcp.WithString("task_on_success", basemcp.Description("Optional status for current_task_id after command exit 0 and valid evidence; defaults to done.")),
		basemcp.WithString("task_on_failure", basemcp.Description("Optional status for current_task_id after command failure, invalid evidence, or pipeline error; defaults to blocked.")),
		basemcp.WithString("task"),
		basemcp.WithBoolean("compact_output", basemcp.Description("Collapse successful command output. Defaults to true.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args pipeline.Request
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		pipes, err := deps.pipelineRunnerForRepo(args.RepoPath, "run_pipeline")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := pipes.Run(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("run_workflow",
		basemcp.WithDescription("Run one repo workflow: guarded edits, checks, and optional owned-files commit. Set preview=true for dry-run."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("current_task_id", basemcp.Description("Optional task id to update during workflow execution.")),
		basemcp.WithString("task_on_start", basemcp.Description("Optional status for current_task_id before executing steps; defaults to in_progress.")),
		basemcp.WithString("task_on_success", basemcp.Description("Optional status for current_task_id after successful workflow; defaults to done.")),
		basemcp.WithString("task_on_failure", basemcp.Description("Optional status for current_task_id after failed workflow; defaults to blocked.")),
		basemcp.WithArray("steps",
			basemcp.Description("Workflow steps: command, guarded_replace, task_batch_upsert, task_transition, git_commit_owned, git_prepare_task_worktree."),
			basemcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":         map[string]any{"type": "string"},
					"tool":       map[string]any{"type": "string"},
					"args":       map[string]any{"type": "object", "additionalProperties": true},
					"depends_on": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"if":         map[string]any{"type": "string"},
					"on_failure": map[string]any{"type": "string"},
				},
				"required": []string{"tool"},
			}),
		),
		basemcp.WithArray("owned_files", basemcp.Description("Repo-relative files the workflow is allowed to modify or commit.")),
		basemcp.WithString("commit_message", basemcp.Description("Optional commit message used by git workflow steps.")),
		basemcp.WithBoolean("preview", basemcp.Description("Set to true for dry-run: returns steps that would execute without running them.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			pipeline.WorkflowRequest
			OwnedFiles    []string `json:"owned_files"`
			CommitMessage string   `json:"commit_message"`
			Preview       bool     `json:"preview"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		if len(args.OwnedFiles) > 0 && len(args.WorkflowRequest.Commit.Files) == 0 {
			args.WorkflowRequest.Commit.Files = args.OwnedFiles
			args.WorkflowRequest.Commit.Enabled = true
		}
		if args.CommitMessage != "" && args.WorkflowRequest.Commit.Message == "" {
			args.WorkflowRequest.Commit.Message = args.CommitMessage
			args.WorkflowRequest.Commit.Enabled = true
		}
		if args.Preview {
			preview := map[string]any{"preview": true, "repo_path": args.RepoPath}
			if len(args.Steps) > 0 {
				stepsPreview := make([]map[string]any, 0, len(args.Steps))
				for _, s := range args.Steps {
					stepsPreview = append(stepsPreview, map[string]any{
						"id":   s.ID,
						"tool": s.Tool,
						"args": s.Args,
					})
				}
				preview["steps"] = stepsPreview
			}
			if len(args.Edits) > 0 {
				editsPreview := make([]map[string]any, 0, len(args.Edits))
				for _, e := range args.Edits {
					editsPreview = append(editsPreview, map[string]any{"path": e.Path})
				}
				preview["edits"] = editsPreview
			}
			if len(args.Checks) > 0 {
				checksPreview := make([]map[string]any, 0, len(args.Checks))
				for _, c := range args.Checks {
					checksPreview = append(checksPreview, map[string]any{"command": c.Command})
				}
				preview["checks"] = checksPreview
			}
			if args.Commit.Enabled {
				preview["commit"] = map[string]any{"files": args.Commit.Files, "message": args.Commit.Message}
			}
			return structured(preview)
		}
		pipes, err := deps.pipelineRunnerForRepo(args.RepoPath, "run_workflow")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := pipes.RunWorkflow(ctx, args.WorkflowRequest)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
}
