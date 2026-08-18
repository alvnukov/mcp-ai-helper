package mcp

import (
	"context"
	"time"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/pipeline"
)

func registerPipelineTools(srv *server.MCPServer, deps *Server) {
	runActions := actions{
		"pipeline":        withDeps(runActionPipeline, deps),
		"workflow":        withDeps(runActionWorkflow, deps),
		"workflow_status": withDeps(runActionWorkflowStatus, deps),
		"schema": func(context.Context, basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			return runActionSchema()
		},
	}
	srv.AddTool(basemcp.NewTool("run",
		runsCommands,
		basemcp.WithDescription("Run pipelines and workflows. Required: action, repo_path (except schema). Actions: pipeline (command, repo_path, cwd?, timeout_seconds?, mcp_wait_seconds?, current_task_id?, task_on_start?, task_on_success?, task_on_failure?, compact_output?, secret_handles?) — run command with evidence extraction; workflow (repo_path, steps[], owned_files?, commit_message?, mcp_wait_seconds?, current_task_id?, task_on_start?, task_on_success?, task_on_failure?, secret_handles?, preview?) — run guarded edits, checks, and optional commit; execution is detached from this call: it waits up to mcp_wait_seconds (default and max 600), then answers running + workflow_id while the workflow keeps going server-side; workflow_status (workflow_id, wait_seconds?) — durable status and final result of a workflow run, blocking up to wait_seconds (max 600) instead of rerunning it; schema () — return valid workflow step types and parameters."),
		basemcp.WithString("action", basemcp.Required(), actionEnum(runActions)),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("command", basemcp.Description("Shell command (pipeline action).")),
		basemcp.WithString("cwd", basemcp.Description("Optional repo-relative working directory.")),
		basemcp.WithNumber("timeout_seconds", basemcp.Description("Optional command timeout in seconds (pipeline action).")),
		basemcp.WithNumber("mcp_wait_seconds", basemcp.Description("Optional MCP wait budget before returning running + command_id (pipeline) or running + workflow_id (workflow; default and max 600).")),
		basemcp.WithString("current_task_id", basemcp.Description("Optional task id to update during execution.")),
		basemcp.WithString("task_on_start", basemcp.Description("Optional status for current_task_id before executing; defaults to in_progress.")),
		basemcp.WithString("task_on_success", basemcp.Description("Optional status for current_task_id after success; defaults to done.")),
		basemcp.WithString("task_on_failure", basemcp.Description("Optional status for current_task_id after failure; defaults to blocked.")),
		basemcp.WithBoolean("compact_output", basemcp.Description("Collapse successful command output (pipeline action). Defaults to true.")),
		basemcp.WithArray("secret_handles", basemcp.Description("Optional server-config secret handles to inject as HELPER_SECRET_<HANDLE>."), basemcp.WithStringItems()),
		basemcp.WithString("workflow_id", basemcp.Description("Workflow run id from a workflow reply (workflow_status action).")),
		basemcp.WithNumber("wait_seconds", basemcp.Description("Optional block-until-finished budget for workflow_status, 0 to 600; defaults to an immediate snapshot.")),
		basemcp.WithArray("steps",
			basemcp.Description("Workflow steps: command, guarded_replace, write_file, task_batch_upsert, task_transition, git_commit_owned, git_prepare_task_worktree."),
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
	), dispatch(deps, "run", runActions))
}

func runActionPipeline(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args pipeline.Request
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.Command == "" {
		return basemcp.NewToolResultError("run action=pipeline requires command"), nil
	}
	pipes, err := deps.pipelineRunnerForRepo(args.RepoPath, "run action=pipeline")
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := pipes.Run(ctx, args)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func runActionWorkflow(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args struct {
		pipeline.WorkflowRequest
		OwnedFiles     []string `json:"owned_files"`
		CommitMessage  string   `json:"commit_message"`
		Preview        bool     `json:"preview"`
		MCPWaitSeconds int      `json:"mcp_wait_seconds"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if len(args.OwnedFiles) > 0 && len(args.Commit.Files) == 0 {
		args.Commit.Files = args.OwnedFiles
		args.Commit.Enabled = true
	}
	if args.CommitMessage != "" && args.Commit.Message == "" {
		args.Commit.Message = args.CommitMessage
		args.Commit.Enabled = true
	}
	pipes, err := deps.pipelineRunnerForRepo(args.RepoPath, "run action=workflow")
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
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

	// The workflow must survive this MCP request: a client timeout cancels the
	// request context, and with it every step. Runs are detached onto a
	// background context and stay addressable by workflow_id afterwards.
	registry := pipeline.DefaultWorkflowRegistry()
	run := registry.Start(pipes, args.WorkflowRequest)
	final, _ := registry.WaitFor(run.WorkflowID, pipeline.WorkflowWaitBudget(args.MCPWaitSeconds))
	if final.FinishedAt == nil {
		return structured(map[string]any{
			"status":      "running",
			"workflow_id": final.WorkflowID,
			"repo_path":   final.RepoPath,
			"started_at":  final.StartedAt,
			"note":        "workflow still running server-side; poll run action=workflow_status with workflow_id",
		})
	}
	if final.Error != "" {
		return basemcp.NewToolResultError(final.Error), nil
	}
	if final.Result == nil {
		return basemcp.NewToolResultError("workflow finished without a result"), nil
	}
	return structured(*final.Result)
}

func runActionWorkflowStatus(_ context.Context, req basemcp.CallToolRequest, _ *Server) (*basemcp.CallToolResult, error) {
	var args struct {
		WorkflowID  string `json:"workflow_id"`
		WaitSeconds int    `json:"wait_seconds"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.WorkflowID == "" {
		return basemcp.NewToolResultError("run action=workflow_status requires workflow_id"), nil
	}
	if args.WaitSeconds < 0 {
		args.WaitSeconds = 0
	}
	if args.WaitSeconds > 600 {
		args.WaitSeconds = 600
	}
	run, ok := pipeline.DefaultWorkflowRegistry().WaitFor(args.WorkflowID, time.Duration(args.WaitSeconds)*time.Second)
	if !ok {
		return basemcp.NewToolResultError("unknown workflow_id: " + args.WorkflowID), nil
	}
	return structured(run)
}

func runActionSchema() (*basemcp.CallToolResult, error) {
	return structured(map[string]any{
		"common_step_fields": map[string]string{
			"depends_on": "Optional array of step IDs this step depends on. Engine auto-detects same-file dependencies; use this for cross-file or cross-tool ordering.",
			"if":         "Condition: 'always' (default), &&, ||, !, changed_files_count comparisons, changed_files contains path, steps.<id>.status/exit_code/validation comparisons, steps.<id>.output_contains text, file_exists/file_missing path, or tasks.<id>.status comparisons.",
			"on_failure": "Optional: 'stop' (default) or 'continue'.",
		},
		// Named for the key a step actually carries. "step_types" invited the
		// "type" key that the engine does not read, which is the mistake this
		// list exists to prevent.
		"step_tools": []map[string]any{
			{
				"tool":        "command",
				"description": "Run a shell command. Workflow stops on non-zero exit unless on_failure is 'continue'.",
				"fields": map[string]string{
					"command":          "Shell command to run (string, required).",
					"cwd":              "Optional repo-relative working directory (string).",
					"web_doc_id":       "Optional fetched document id. When set, command receives HELPER_WEB_DOC_PATH pointing to the selected helper-managed artifact.",
					"web_doc_source":   "Optional artifact source for web_doc_id: normalized (default) or raw.",
					"mcp_wait_seconds": "Optional MCP wait budget before returning running + command_id.",
					"on_failure":       "Optional: 'stop' (default) or 'continue'.",
				},
			},
			{
				"tool":        "guarded_replace",
				"description": "Replace one unique text span only if the file hash still matches. Use file action=read first, then file action=snapshot, then this. Takes the same text arguments as edit action=replace, base64 included; a malformed encoding fails the workflow before its first step runs.",
				"fields": map[string]string{
					"path":          "Repo-relative file path (string, required).",
					"expected_hash": "SHA-256 hash from file action=snapshot before edit (string, required).",
					"old":           "Text to replace (string, required unless old_b64 is set).",
					"new":           "Replacement text (string, required unless new_b64 is set).",
					"old_b64":       "Base64-encoded old text (string). Use instead of old for backslash-heavy spans; it wins when both are set.",
					"new_b64":       "Base64-encoded replacement text (string). Use instead of new for backslash-heavy spans; it wins when both are set.",
				},
			},
			{
				"tool":        "write_file",
				"description": "Whole-file create/replace using edit.write semantics. Malformed base64 fails before the first workflow step.",
				"fields": map[string]string{
					"path":          "Repo-relative file path (string, required).",
					"content":       "Whole-file text content (string, required unless content_b64 is set).",
					"content_b64":   "Base64-encoded whole-file content (string). It wins when both are set.",
					"expected_hash": "Optional SHA-256 overwrite guard from file action=snapshot.",
					"mode":          "Optional file mode (number, default 0644).",
				},
			},
			{
				"tool":        "task_batch_upsert",
				"description": "Synchronize per-repository task state.",
				"fields": map[string]string{
					"tasks":           "Array of task objects with id, title, status, priority, model_level, tags, body (required).",
					"close_missing":   "Close active tasks not in this batch (boolean).",
					"missing_status":  "Status written to every task close_missing closes: todo, in_progress, blocked or done (string, default done).",
					"active_statuses": "Statuses that make a task eligible for close_missing, each todo, in_progress, blocked or done (array of strings, default todo/in_progress/blocked).",
				},
			},
			{
				"tool":        "task_transition",
				"description": "Guardedly transition task statuses inside a workflow.",
				"fields": map[string]string{
					"task_ids": "Task IDs to transition (array of strings, required).",
					"from":     "Optional required current status for every task.",
					"to":       "Target status (string, required).",
				},
			},
			{
				"tool":        "git_commit_owned",
				"description": "Commit only explicit owned files. Never stages all files.",
				"fields": map[string]string{
					"files":   "Repo-relative files to commit (array of strings, required).",
					"message": "Commit message (string, required).",
				},
			},
			{
				"tool":        "git_prepare_task_worktree",
				"description": "Create or reuse .worktrees/<task_id> on branch <task_type>/<task_id>.",
				"fields": map[string]string{
					"task_id":   "Task id, e.g. task-057 (string, required).",
					"task_type": "Branch type, e.g. feature, bug, hotfix, chore, docs, refactor, test, ci (string, required).",
				},
			},
		},
	})
}
