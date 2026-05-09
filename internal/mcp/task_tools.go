package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func registerTaskTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("task_add",
		basemcp.WithDescription("Create or replace a per-repository task file in Lean format."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("title", basemcp.Required()),
		basemcp.WithString("body"),
		basemcp.WithString("status"),
		basemcp.WithString("id"),
		basemcp.WithString("priority"),
		basemcp.WithArray("tags"),
		basemcp.WithString("parent_id"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.AddRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		task, err := store.Add(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(task)
	})
	srv.AddTool(basemcp.NewTool("task_list",
		basemcp.WithDescription("List per-repository tasks, optionally filtered by exact status and query."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("status"),
		basemcp.WithString("query"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		list, err := store.List(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"tasks": list})
	})
	srv.AddTool(basemcp.NewTool("task_search",
		basemcp.WithDescription("Search per-repository tasks by id, status, title, body, priority, or tag."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("query", basemcp.Required()),
		basemcp.WithString("status"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		list, err := store.List(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"tasks": list})
	})
	srv.AddTool(basemcp.NewTool("task_update",
		basemcp.WithDescription("Partially update one per-repository task without replacing unspecified fields."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
		basemcp.WithString("title"),
		basemcp.WithString("body"),
		basemcp.WithString("status"),
		basemcp.WithString("priority"),
		basemcp.WithArray("tags"),
		basemcp.WithString("parent_id"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.UpdateRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		task, err := store.Update(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(task)
	})
	srv.AddTool(basemcp.NewTool("task_set_status",
		basemcp.WithDescription("Set one per-repository task status."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
		basemcp.WithString("status", basemcp.Required()),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.StatusRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		task, err := store.SetStatus(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(task)
	})
	srv.AddTool(basemcp.NewTool("task_batch_upsert",
		basemcp.WithDescription("Synchronize many per-repository tasks in one call and optionally close active tasks omitted from the batch."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithArray("tasks", basemcp.Required()),
		basemcp.WithBoolean("close_missing"),
		basemcp.WithString("missing_status"),
		basemcp.WithArray("active_statuses"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.BatchUpsertRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		result, err := store.BatchUpsert(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
	srv.AddTool(basemcp.NewTool("task_upsert",
		basemcp.WithDescription("Create or update a single task. Preferred over task_add/task_update/task_set_status for single-task changes."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Description("Task id. Creates new if not found, updates if exists.")),
		basemcp.WithString("title", basemcp.Required()),
		basemcp.WithString("status", basemcp.Description("Task status: todo, in_progress, blocked, done.")),
		basemcp.WithString("priority", basemcp.Description("Task priority: low, normal, high, critical.")),
		basemcp.WithString("body", basemcp.Description("Task description.")),
		basemcp.WithArray("tags", basemcp.Description("Optional tags.")),
		basemcp.WithString("parent_id", basemcp.Description("Optional parent task id for hierarchy.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.AddRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		task, err := store.Add(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(task)
	})
	srv.AddTool(basemcp.NewTool("task_current",
		basemcp.WithDescription("Return active per-repository tasks with todo or in_progress status."),
		basemcp.WithString("repo_path", basemcp.Required()),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		list, err := store.List(tasks.ListRequest{RepoPath: args.RepoPath})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"tasks": currentTasks(list)})
	})
	srv.AddTool(basemcp.NewTool("task_tree",
		basemcp.WithDescription("Return task tree from the goal root. Goal = task with tag 'goal' and no parent_id."),
		basemcp.WithString("repo_path", basemcp.Required()),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.ListRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		list, err := store.List(tasks.ListRequest{RepoPath: args.RepoPath})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(buildTaskTree(list))
	})
	srv.AddTool(basemcp.NewTool("task_get",
		basemcp.WithDescription("Read one per-repository task by id."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.GetRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		task, err := store.Get(args)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(task)
	})
	srv.AddTool(basemcp.NewTool("task_delete",
		basemcp.WithDescription("Delete one per-repository task by id."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args tasks.DeleteRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		if err := store.Delete(args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]bool{"deleted": true})
	})
}
