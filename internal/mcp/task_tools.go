package mcp

import (
	"context"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
)

func registerTaskTools(srv *server.MCPServer, deps *Server) {
	taskActions := actions{
		"current":      withDeps(taskActionCurrent, deps),
		"get":          withDeps(taskActionGet, deps),
		"list":         withDeps(taskActionList, deps),
		"search":       withDeps(taskActionSearch, deps),
		"upsert":       withDeps(taskActionUpsert, deps),
		"set_status":   withDeps(taskActionSetStatus, deps),
		"batch_upsert": withDeps(taskActionBatchUpsert, deps),
		"delete":       withDeps(taskActionDelete, deps),
	}
	srv.AddTool(basemcp.NewTool("task",
		basemcp.WithDescription("Per-repository task registry. Required: repo_path, action. Actions: current (no extra args, returns active tasks); get (id); list (status?, query?); search (query, status?); upsert (id?, title, status?, task_type?, priority?, model_level?, body?, tags?, acceptance_criteria?, verification_plan?, parent_id?); set_status (id, status); batch_upsert (tasks[], close_missing?, missing_status?, active_statuses?); delete (id)."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("action", basemcp.Required(), actionEnum(taskActions)),
		basemcp.WithString("id", basemcp.Description("Task id. Required for get, set_status, delete. Optional for upsert (creates new if absent).")),
		basemcp.WithString("title", basemcp.Description("Task title. Required for upsert.")),
		basemcp.WithString("status", basemcp.Description("Filter by status for list/search; target status for set_status (todo, in_progress, blocked, done).")),
		basemcp.WithString("query", basemcp.Description("Search query. Required for search, optional for list.")),
		basemcp.WithString("task_type", basemcp.Description("Branch type for task worktree: feature, bug, hotfix, chore, docs, refactor, test, ci.")),
		basemcp.WithString("priority", basemcp.Description("Task priority: low, medium, high, critical.")),
		basemcp.WithString("model_level", basemcp.Description("Minimum model level: low, medium, high, very_high.")),
		basemcp.WithString("body", basemcp.Description("Task description.")),
		basemcp.WithString("parent_id", basemcp.Description("Optional parent task id for hierarchy.")),
		basemcp.WithArray("tags", basemcp.Description("Optional tags.")),
		basemcp.WithArray("acceptance_criteria", basemcp.Description("Structured completion criteria.")),
		basemcp.WithArray("verification_plan", basemcp.Description("Structured verification steps.")),
		basemcp.WithArray("tasks", basemcp.Items(taskUpsertItemSchema()), basemcp.Description("Batch upsert task array. Each item requires id and title.")),
		basemcp.WithBoolean("close_missing", basemcp.Description("Batch: close active tasks omitted from the batch.")),
		basemcp.WithString("missing_status", basemcp.Description("Batch: status for omitted tasks.")),
		basemcp.WithArray("active_statuses", basemcp.Description("Batch: statuses considered active for close_missing.")),
	), dispatch("task", taskActions))
}

func taskActionCurrent(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
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
}

func taskActionGet(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args tasks.GetRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.ID) == "" {
		return basemcp.NewToolResultError("task action=get requires id"), nil
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
}

func taskActionList(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
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
}

func taskActionSearch(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args tasks.ListRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.Query) == "" {
		return basemcp.NewToolResultError("task action=search requires query"), nil
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
}

func taskActionUpsert(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args tasks.AddRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.Title) == "" {
		return basemcp.NewToolResultError("task action=upsert requires title"), nil
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
}

func taskActionSetStatus(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args tasks.StatusRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.ID) == "" {
		return basemcp.NewToolResultError("task action=set_status requires id"), nil
	}
	if strings.TrimSpace(args.Status) == "" {
		return basemcp.NewToolResultError("task action=set_status requires status"), nil
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
}

func taskActionBatchUpsert(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
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
}

func taskActionDelete(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args tasks.DeleteRequest
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if strings.TrimSpace(args.ID) == "" {
		return basemcp.NewToolResultError("task action=delete requires id"), nil
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
}

func registerTaskAdvancedTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("task_graph",
		basemcp.WithDescription("Bounded task graph after task action=current. focus_task_id=task-123 centers one task. Edges: kind=parent_child, provenance=explicit. Reports truncated data; next_call: task action=current or retry focused."),
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
		basemcp.WithDescription("Compact execution context for task_id=task-123 after task action=current. Includes goals, boundaries, criteria, verification, warnings, usage_contract, truncated metadata. Use task_graph for dependency overview. next_call: task action=current on missing ids."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root.")),
		basemcp.WithString("task_id", basemcp.Required(), basemcp.Description("Task id; discover with task action=current.")),
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

type taskListItem struct {
	ID                      string   `json:"id"`
	TaskType                string   `json:"task_type,omitempty"`
	Branch                  string   `json:"branch,omitempty"`
	WorktreePath            string   `json:"worktree_path,omitempty"`
	CodePath                string   `json:"code_path,omitempty"`
	ParentID                string   `json:"parent_id,omitempty"`
	Status                  string   `json:"status"`
	Title                   string   `json:"title"`
	Priority                string   `json:"priority,omitempty"`
	ModelLevel              string   `json:"model_level"`
	Tags                    []string `json:"tags,omitempty"`
	AcceptanceCriteriaCount int      `json:"acceptance_criteria_count,omitempty"`
	VerificationPlanCount   int      `json:"verification_plan_count,omitempty"`
	ProjectionSource        string   `json:"projection_source,omitempty"`
}

func taskListResponse(backend taskBackend, visible []tasks.Task, counted []tasks.Task, source string) map[string]any {
	out := map[string]any{
		"tasks":            summarizeTaskList(visible),
		"source":           source,
		"counts_by_status": countTasksByStatus(counted),
		"tasks_returned":   len(visible),
		"tasks_total":      len(counted),
		"details_omitted":  []string{"body", "acceptance_criteria", "verification_plan", "created_at", "updated_at"},
		"next_call":        "Use task action=get for one full task or task_context for execution context after choosing an id.",
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

func summarizeTaskList(list []tasks.Task) []taskListItem {
	out := make([]taskListItem, 0, len(list))
	for _, task := range list {
		item := taskListItem{
			ID:                      task.ID,
			TaskType:                task.TaskType,
			Branch:                  task.Branch,
			WorktreePath:            task.WorktreePath,
			CodePath:                task.CodePath,
			ParentID:                task.ParentID,
			Status:                  task.Status,
			Title:                   task.Title,
			Priority:                task.Priority,
			ModelLevel:              task.ModelLevel,
			AcceptanceCriteriaCount: len(task.AcceptanceCriteria),
			VerificationPlanCount:   len(task.VerificationPlan),
			ProjectionSource:        task.ProjectionSource,
		}
		if len(task.Tags) > 0 {
			item.Tags = append([]string(nil), task.Tags...)
		}
		out = append(out, item)
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
