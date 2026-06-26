package mcp

import (
	"context"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func registerTaskTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("task_list",
		basemcp.WithDescription("List per-repository tasks, optionally filtered by exact status and query."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("status"),
		basemcp.WithString("query"),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		list, source, err := backend.ListAll(ctx, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(taskListResponse(backend, filterTasks(list, args), list, source))
	})
	srv.AddTool(basemcp.NewTool("task_search",
		basemcp.WithDescription("Search per-repository tasks by id, status, title, body, priority, or tag."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("query", basemcp.Required()),
		basemcp.WithString("status"),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		list, source, err := backend.ListAll(ctx, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(taskListResponse(backend, filterTasks(list, args), list, source))
	})
	srv.AddTool(basemcp.NewTool("task_set_status",
		basemcp.WithDescription("Set one per-repository task status."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
		basemcp.WithString("status", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.StatusRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := backend.SetStatus(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("task_batch_upsert",
		basemcp.WithDescription("Synchronize many per-repository tasks in one call and optionally close active tasks omitted from the batch."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithArray("tasks", basemcp.Required(), basemcp.Items(taskUpsertItemSchema())),
		basemcp.WithBoolean("close_missing"),
		basemcp.WithString("missing_status"),
		basemcp.WithArray("active_statuses"),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.BatchUpsertRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := backend.BatchUpsert(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("task_upsert",
		basemcp.WithDescription("Create or update one task. Use task_set_status for status-only changes."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Description("Task id. Creates new if not found, updates if exists.")),
		basemcp.WithString("title", basemcp.Required()),
		basemcp.WithString("status", basemcp.Description("Task status: todo, in_progress, blocked, done.")),
		basemcp.WithString("task_type", basemcp.Description("Branch type for task worktree, e.g. feature, bug, hotfix, chore, docs, refactor, test, ci.")),
		basemcp.WithString("priority", basemcp.Description("Task priority: low, medium, high, critical.")),
		basemcp.WithString("model_level", basemcp.Description("Minimum model level for the task: low, medium, high, very_high.")),
		basemcp.WithString("body", basemcp.Description("Task description.")),
		basemcp.WithArray("tags", basemcp.Description("Optional tags.")),
		basemcp.WithArray("acceptance_criteria", basemcp.Description("Structured completion criteria.")),
		basemcp.WithArray("verification_plan", basemcp.Description("Structured verification steps.")),
		basemcp.WithString("parent_id", basemcp.Description("Optional parent task id for hierarchy.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.AddRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := backend.Upsert(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("task_current",
		basemcp.WithDescription("Return active per-repository tasks with todo or in_progress status."),
		basemcp.WithString("repo_path", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		list, source, err := backend.ListCurrent(ctx, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(taskListResponse(backend, list, list, source))
	})
	srv.AddTool(basemcp.NewTool("task_get",
		basemcp.WithDescription("Read one per-repository task by id."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.GetRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		task, _, err := backend.Get(ctx, args.RepoPath, args.ID)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(task)
	})
	srv.AddTool(basemcp.NewTool("task_delete",
		basemcp.WithDescription("Delete one per-repository task by id."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.DeleteRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := backend.Delete(ctx, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("task_graph",
		basemcp.WithDescription("Bounded task graph after task_current. focus_task_id=task-123 centers one task. Edges: kind=parent_child, provenance=explicit. Reports truncated data; next_call: task_current or retry focused."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root.")),
		basemcp.WithString("focus_task_id", basemcp.Description("Optional task id to center the graph.")),
		basemcp.WithNumber("max_nodes", basemcp.Description("Max nodes; truncation reports omissions.")),
		basemcp.WithNumber("max_bytes", basemcp.Description("Max response bytes.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args TaskGraphRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		if err := validateTaskGraphRequest(args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		list, source, err := backend.ListAll(ctx, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		graph, err := BuildTaskGraph(list, args, source)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(graph)
	})
	srv.AddTool(basemcp.NewTool("task_context",
		basemcp.WithDescription("Compact execution context for task_id=task-123 after task_current. Includes goals, boundaries, criteria, verification, warnings, usage_contract, truncated metadata. Use task_graph for dependency overview. next_call: task_current on missing ids."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root.")),
		basemcp.WithString("task_id", basemcp.Required(), basemcp.Description("Task id; discover with task_current.")),
		basemcp.WithNumber("max_nodes", basemcp.Description("Max items per section.")),
		basemcp.WithNumber("max_bytes", basemcp.Description("Max response bytes.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args TaskContextRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		if err := validateTaskContextRequest(args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		backend, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		list, _, err := backend.ListAll(ctx, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		ctxResult, err := BuildTaskContext(list, args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(ctxResult)
	})
	srv.AddTool(basemcp.NewTool("task_export",
		basemcp.WithDescription("Export tasks from the current backend to an Obsidian Markdown directory."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("target_dir", basemcp.Required()),
		basemcp.WithBoolean("dry_run"),
		basemcp.WithBoolean("overwrite"),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args ExportRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		source, err := deps.loadTaskBackendForRepo(args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		target := newObsidianTaskBackend(args.TargetDir)
		result, err := exportTasks(ctx, source, target, args.RepoPath, ImportExportRequest{DryRun: args.DryRun, Overwrite: args.Overwrite})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
}

func taskUpsertItemSchema() map[string]any {
	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":                  map[string]any{"type": "string", "description": "Stable task id."},
			"title":               map[string]any{"type": "string"},
			"status":              map[string]any{"type": "string", "description": "todo, in_progress, blocked, or done."},
			"body":                map[string]any{"type": "string"},
			"priority":            map[string]any{"type": "string"},
			"model_level":         map[string]any{"type": "string"},
			"task_type":           map[string]any{"type": "string"},
			"branch":              map[string]any{"type": "string"},
			"worktree_path":       map[string]any{"type": "string"},
			"parent_id":           map[string]any{"type": "string"},
			"tags":                stringArray,
			"acceptance_criteria": stringArray,
			"verification_plan":   stringArray,
		},
		"required": []string{"id", "title"},
	}
}

func mergeTaskUpdate(existing tasks.Task, update tasks.UpdateRequest) tasks.AddRequest {
	merged := tasks.AddRequest{RepoPath: update.RepoPath, ID: existing.ID, TaskType: existing.TaskType, Branch: existing.Branch, WorktreePath: existing.WorktreePath, ParentID: existing.ParentID, Status: existing.Status, Title: existing.Title, Body: existing.Body, Priority: existing.Priority, ModelLevel: existing.ModelLevel, Tags: existing.Tags, AcceptanceCriteria: existing.AcceptanceCriteria, VerificationPlan: existing.VerificationPlan}
	if strings.TrimSpace(update.Status) != "" {
		merged.Status = strings.TrimSpace(update.Status)
	}
	if strings.TrimSpace(update.TaskType) != "" {
		merged.TaskType = strings.TrimSpace(update.TaskType)
	}
	if strings.TrimSpace(update.Branch) != "" {
		merged.Branch = strings.TrimSpace(update.Branch)
	}
	if strings.TrimSpace(update.WorktreePath) != "" {
		merged.WorktreePath = strings.TrimSpace(update.WorktreePath)
	}
	if update.ParentID != "" {
		merged.ParentID = update.ParentID
	}
	if strings.TrimSpace(update.Title) != "" {
		merged.Title = update.Title
	}
	if update.Body != "" {
		merged.Body = update.Body
	}
	if strings.TrimSpace(update.Priority) != "" {
		merged.Priority = strings.TrimSpace(update.Priority)
	}
	if strings.TrimSpace(update.ModelLevel) != "" {
		merged.ModelLevel = strings.TrimSpace(update.ModelLevel)
	}
	if update.Tags != nil {
		merged.Tags = update.Tags
	}
	if update.AcceptanceCriteria != nil {
		merged.AcceptanceCriteria = update.AcceptanceCriteria
	}
	if update.VerificationPlan != nil {
		merged.VerificationPlan = update.VerificationPlan
	}
	return merged
}

func taskListResponse(backend taskBackend, visible []tasks.Task, counted []tasks.Task, source string) map[string]any {
	out := map[string]any{
		"tasks":            visible,
		"source":           source,
		"counts_by_status": countTasksByStatus(counted),
	}
	if provider, ok := backend.(taskListMetadataProvider); ok {
		meta := provider.ListMetadata()
		if meta.Validation != "" {
			out["validation"] = meta.Validation
		}
		if len(meta.Diagnostics) > 0 {
			out["diagnostics"] = meta.Diagnostics
		}
		if len(meta.ChangedFiles) > 0 {
			out["changed_files"] = meta.ChangedFiles
		}
	}
	return out
}

func countTasksByStatus(list []tasks.Task) map[string]int {
	counts := make(map[string]int)
	for _, task := range list {
		counts[task.Status]++
	}
	return counts
}

func filterTasks(list []tasks.Task, req tasks.ListRequest) []tasks.Task {
	out := make([]tasks.Task, 0, len(list))
	for _, task := range list {
		if req.Status != "" && task.Status != req.Status {
			continue
		}
		if req.Query != "" && !taskMatchesMCP(task, req.Query) {
			continue
		}
		out = append(out, task)
	}
	return out
}

func taskMatchesMCP(task tasks.Task, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	fields := []string{task.ID, task.TaskType, task.Branch, task.WorktreePath, task.CodePath, task.Status, task.Title, task.Body, task.Priority, task.ModelLevel, task.ParentID}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	for _, tag := range task.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}
