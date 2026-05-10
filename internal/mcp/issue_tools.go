package mcp

import (
	"context"
	"slices"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type issueAddRequest struct {
	RepoPath       string   `json:"repo_path"`
	SourceRepoPath string   `json:"source_repo_path"`
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Body           string   `json:"body"`
	Priority       string   `json:"priority"`
	Tags           []string `json:"tags"`
}

type issueListRequest struct {
	RepoPath string `json:"repo_path"`
	Status   string `json:"status"`
	Query    string `json:"query"`
}

type issueAcceptRequest struct {
	RepoPath string `json:"repo_path"`
	ID       string `json:"id"`
}

func registerIssueTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("issue_add",
		basemcp.WithDescription("Record cross-repository feedback as an actionable Lean-backed task issue."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Target repository root that should receive the issue.")),
		basemcp.WithString("source_repo_path", basemcp.Description("Repository root where the feedback originated.")),
		basemcp.WithString("id", basemcp.Description("Canonical task-NNN id for the issue.")),
		basemcp.WithString("title", basemcp.Required(), basemcp.Description("Short issue title.")),
		basemcp.WithString("body", basemcp.Description("Feedback details and expected behavior.")),
		basemcp.WithString("priority", basemcp.Description("Issue priority: low, medium, high, critical.")),
		basemcp.WithArray("tags", basemcp.Description("Optional additional issue tags.")),
	), func(ctx context.Context, request basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args issueAddRequest
		if err := bind(request, &args); err != nil {
			return nil, err
		}
		_, _, commands, _, store := deps.loadDeps()
		result, err := addIssue(ctx, args, commands, store)
		if err != nil {
			return nil, err
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("issue_list",
		basemcp.WithDescription("List open feedback issues recorded in the Lean task registry."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root used for issue lookup.")),
		basemcp.WithString("status", basemcp.Description("Optional task status filter; defaults to todo.")),
		basemcp.WithString("query", basemcp.Description("Optional text query.")),
	), func(ctx context.Context, request basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args issueListRequest
		if err := bind(request, &args); err != nil {
			return nil, err
		}
		_, _, commands, _, store := deps.loadDeps()
		result, err := listIssues(ctx, args, commands, store)
		if err != nil {
			return nil, err
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("issue_accept",
		basemcp.WithDescription("Accept one feedback issue as current work by moving it to in_progress."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root used for issue lookup.")),
		basemcp.WithString("id", basemcp.Required(), basemcp.Description("Canonical task-NNN issue id to accept.")),
	), func(ctx context.Context, request basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args issueAcceptRequest
		if err := bind(request, &args); err != nil {
			return nil, err
		}
		_, _, commands, _, store := deps.loadDeps()
		result, err := acceptIssue(ctx, args, commands, store)
		if err != nil {
			return nil, err
		}
		return structured(result)
	})
}

func addIssue(ctx context.Context, req issueAddRequest, commands *command.Runner, store *tasks.Store) (tasks.Task, error) {
	body := strings.TrimSpace(req.Body)
	if strings.TrimSpace(req.SourceRepoPath) != "" {
		body = strings.TrimSpace(body + "\n\nsource_repo_path: " + req.SourceRepoPath)
	}
	tags := append([]string{"issue", "feedback"}, req.Tags...)
	result, err := upsertTask(ctx, tasks.AddRequest{
		RepoPath: req.RepoPath,
		ID:       req.ID,
		Status:   "todo",
		Title:    req.Title,
		Body:     body,
		Priority: req.Priority,
		Tags:     uniqueStrings(tags),
	}, commands, store)
	return result.Task, err
}

func listIssues(ctx context.Context, req issueListRequest, commands *command.Runner, store *tasks.Store) ([]tasks.Task, error) {
	status := req.Status
	if status == "" {
		status = "todo"
	}
	var listed []tasks.Task
	var err error
	if status == "done" {
		listed, _, err = readAllTasks(ctx, req.RepoPath, commands, store)
	} else {
		listed, _, err = readCurrentTasks(ctx, req.RepoPath, commands, store)
	}
	if err != nil {
		return nil, err
	}
	issues := make([]tasks.Task, 0, len(listed))
	for _, task := range filterTasks(listed, tasks.ListRequest{RepoPath: req.RepoPath, Status: status, Query: req.Query}) {
		if slices.Contains(task.Tags, "issue") || slices.Contains(task.Tags, "feedback") {
			issues = append(issues, task)
		}
	}
	return issues, nil
}

func acceptIssue(ctx context.Context, req issueAcceptRequest, commands *command.Runner, store *tasks.Store) (tasks.Task, error) {
	result, err := setTaskStatus(ctx, tasks.StatusRequest{RepoPath: req.RepoPath, ID: req.ID, Status: "in_progress"}, commands, store)
	return result.Task, err
}
