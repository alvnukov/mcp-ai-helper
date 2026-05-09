package mcp

import (
	"context"
	"slices"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

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
		basemcp.WithDescription("Record cross-repository feedback as an actionable issue in the target repository task store."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Target repository root that should receive the issue.")),
		basemcp.WithString("source_repo_path", basemcp.Description("Repository root where the feedback originated.")),
		basemcp.WithString("id", basemcp.Description("Optional stable issue id.")),
		basemcp.WithString("title", basemcp.Required(), basemcp.Description("Short issue title.")),
		basemcp.WithString("body", basemcp.Description("Feedback details and expected behavior.")),
		basemcp.WithString("priority", basemcp.Description("Issue priority: low, normal, high, critical.")),
		basemcp.WithArray("tags", basemcp.Description("Optional additional issue tags.")),
	), func(ctx context.Context, request basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args issueAddRequest
		if err := bind(request, &args); err != nil {
			return nil, err
		}
		_, _, _, _, store := deps.loadDeps()
		result, err := addIssue(store, args)
		if err != nil {
			return nil, err
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("issue_list",
		basemcp.WithDescription("List open feedback issues recorded for a repository."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root used for issue lookup.")),
		basemcp.WithString("status", basemcp.Description("Optional task status filter; defaults to todo.")),
		basemcp.WithString("query", basemcp.Description("Optional text query.")),
	), func(ctx context.Context, request basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args issueListRequest
		if err := bind(request, &args); err != nil {
			return nil, err
		}
		_, _, _, _, store := deps.loadDeps()
		result, err := listIssues(store, args)
		if err != nil {
			return nil, err
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("issue_accept",
		basemcp.WithDescription("Accept one feedback issue as current work by moving it to in_progress."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root used for issue lookup.")),
		basemcp.WithString("id", basemcp.Required(), basemcp.Description("Issue id to accept.")),
	), func(ctx context.Context, request basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args issueAcceptRequest
		if err := bind(request, &args); err != nil {
			return nil, err
		}
		_, _, _, _, store := deps.loadDeps()
		result, err := acceptIssue(store, args)
		if err != nil {
			return nil, err
		}
		return structured(result)
	})
}

func addIssue(store *tasks.Store, req issueAddRequest) (tasks.Task, error) {
	body := strings.TrimSpace(req.Body)
	if strings.TrimSpace(req.SourceRepoPath) != "" {
		body = strings.TrimSpace(body + "\n\nsource_repo_path: " + req.SourceRepoPath)
	}
	tags := append([]string{"issue", "feedback"}, req.Tags...)
	return store.Add(tasks.AddRequest{
		RepoPath: req.RepoPath,
		ID:       req.ID,
		Status:   "todo",
		Title:    req.Title,
		Body:     body,
		Priority: req.Priority,
		Tags:     uniqueStrings(tags),
	})
}

func listIssues(store *tasks.Store, req issueListRequest) ([]tasks.Task, error) {
	status := req.Status
	if status == "" {
		status = "todo"
	}
	listed, err := store.List(tasks.ListRequest{RepoPath: req.RepoPath, Status: status, Query: req.Query})
	if err != nil {
		return nil, err
	}
	issues := make([]tasks.Task, 0, len(listed))
	for _, task := range listed {
		if slices.Contains(task.Tags, "issue") || slices.Contains(task.Tags, "feedback") {
			issues = append(issues, task)
		}
	}
	return issues, nil
}

func acceptIssue(store *tasks.Store, req issueAcceptRequest) (tasks.Task, error) {
	return store.SetStatus(tasks.StatusRequest{RepoPath: req.RepoPath, ID: req.ID, Status: "in_progress"})
}
