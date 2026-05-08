// Package mcp wires mcp-ai-helper capabilities into an MCP stdio server.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/fileops"
	"github.com/zol/mcp-ai-helper/internal/gitops"
	"github.com/zol/mcp-ai-helper/internal/pipeline"
	"github.com/zol/mcp-ai-helper/internal/project"
	"github.com/zol/mcp-ai-helper/internal/provider"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// New constructs an MCP server with all configured helper tools.
func New(cfg *config.Config) *server.MCPServer {

	srv := server.NewMCPServer(
		"mcp-ai-helper",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
	)
	chat := provider.NewOpenAICompatibleClient(cfg.Providers)
	commands := command.NewRunner(cfg.CommandPolicy)
	pipelines := pipeline.NewRunner(cfg, chat)
	projectStore, err := project.NewStore(cfg.CommandPolicy.LogDir)
	if err != nil {
		projectStore, _ = project.NewStore(".mcp-ai-helper")
	}
	taskStore := tasks.NewStore(projectStore)
	reloadConfig := func(path string) (*config.Config, error) {
		if strings.TrimSpace(path) == "" {
			path = cfg.SourcePath
		}
		next, err := config.Load(path)
		if err != nil {
			return nil, err
		}
		*cfg = *next
		chat = provider.NewOpenAICompatibleClient(cfg.Providers)
		commands = command.NewRunner(cfg.CommandPolicy)
		pipelines = pipeline.NewRunner(cfg, chat)
		projectStore, err = project.NewStore(cfg.CommandPolicy.LogDir)
		if err != nil {
			projectStore, _ = project.NewStore(".mcp-ai-helper")
		}
		taskStore = tasks.NewStore(projectStore)
		return cfg, nil
	}
	registerConfigTools(srv, cfg, reloadConfig)

	if cfg.LayerEnabled("models") {
		registerGuidance(srv, cfg)

		srv.AddTool(basemcp.NewTool("list_models", basemcp.WithDescription("List configured model profiles and routing policy.")),
			func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
				return structured(map[string]any{"models": cfg.Models, "routing": cfg.Routing})
			})

		srv.AddTool(basemcp.NewTool("query_model",
			basemcp.WithDescription("Send a bounded prompt to a configured OpenAI-compatible model."),
			basemcp.WithString("model_id", basemcp.Description("Configured model id.")),
			basemcp.WithString("prompt", basemcp.Description("User prompt.")),
		), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args struct {
				ModelID string `json:"model_id"`
				Prompt  string `json:"prompt"`
			}
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			model, ok := cfg.Models[args.ModelID]
			if !ok {
				return basemcp.NewToolResultError("unknown model_id"), nil
			}
			resp, err := chat.Complete(ctx, provider.ChatRequest{
				ProviderID:      model.Provider,
				ModelID:         args.ModelID,
				Model:           model.Model,
				SystemPrompt:    model.Prompt(),
				UserPrompt:      args.Prompt,
				MaxOutputTokens: model.MaxOutputTokens,
				Temperature:     model.Temperature,
			})
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(resp)
		})

		srv.AddTool(basemcp.NewTool("collect_command_output",
			basemcp.WithDescription("Run a command under policy limits and extract compact evidence lines."),
			basemcp.WithString("command", basemcp.Required(), basemcp.Description("Shell command.")),
			basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root from the calling LLM.")),
			basemcp.WithString("cwd", basemcp.Description("Optional repo-relative working directory.")),
		), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args struct {
				Command        string         `json:"command"`
				RepoPath       string         `json:"repo_path"`
				CWD            string         `json:"cwd"`
				TimeoutSeconds int            `json:"timeout_seconds"`
				Filter         command.Filter `json:"filter"`
			}
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			result, err := commands.RunFilteredInRepo(ctx, args.Command, args.RepoPath, args.CWD, args.TimeoutSeconds, args.Filter)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(result)
		})

		srv.AddTool(basemcp.NewTool("filter_command_history",
			basemcp.WithDescription("Re-filter retained command output by command_id without rerunning the command."),
			basemcp.WithString("command_id", basemcp.Required()),
		), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args struct {
				CommandID string         `json:"command_id"`
				Filter    command.Filter `json:"filter"`
			}
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			result, err := commands.FilterHistory(args.CommandID, args.Filter)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(result)
		})

		srv.AddTool(basemcp.NewTool("run_pipeline",
			basemcp.WithDescription("Run command -> evidence extraction -> optional model analysis -> evidence validation."),
			basemcp.WithString("command", basemcp.Required()),
			basemcp.WithString("repo_path", basemcp.Required()),
			basemcp.WithString("cwd", basemcp.Description("Optional repo-relative working directory.")),
			basemcp.WithString("task"),
			basemcp.WithBoolean("compact_output", basemcp.Description("Collapse successful command output. Defaults to true.")),
		), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args pipeline.Request
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			result, err := pipelines.Run(ctx, args)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(result)
		})

		srv.AddTool(basemcp.NewTool("run_workflow",
			basemcp.WithDescription("Run one repo workflow: guarded edits, checks, and optional owned-files commit."),
			basemcp.WithString("repo_path", basemcp.Required()),
			basemcp.WithString("current_task_id", basemcp.Description("Optional task id to update during workflow execution.")),
			basemcp.WithString("task_on_start", basemcp.Description("Optional status for current_task_id before executing steps; defaults to in_progress.")),
			basemcp.WithString("task_on_success", basemcp.Description("Optional status for current_task_id after successful workflow; defaults to done.")),
			basemcp.WithString("task_on_failure", basemcp.Description("Optional status for current_task_id after failed workflow; defaults to blocked.")),
			basemcp.WithArray("steps", basemcp.Description("Workflow steps: command, guarded_replace, task_batch_upsert, git_commit_owned.")),
			basemcp.WithArray("owned_files", basemcp.Description("Repo-relative files the workflow is allowed to modify or commit.")),
			basemcp.WithString("commit_message", basemcp.Description("Optional commit message used by git workflow steps.")),
		), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args pipeline.WorkflowRequest
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			result, err := pipelines.RunWorkflow(ctx, args)
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

		srv.AddTool(basemcp.NewTool("apply_guarded_replace",
			basemcp.WithDescription("Replace one unique text span only if the file hash still matches."),
			basemcp.WithString("repo_path", basemcp.Required()),
			basemcp.WithString("path", basemcp.Required()),
			basemcp.WithString("expected_hash", basemcp.Required()),
			basemcp.WithString("old", basemcp.Required()),
			basemcp.WithString("new", basemcp.Required()),
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
		if cfg.LayerEnabled("issues") {
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
				result, err := addIssue(taskStore, args)
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
				result, err := listIssues(taskStore, args)
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
				result, err := acceptIssue(taskStore, args)
				if err != nil {
					return nil, err
				}
				return structured(result)
			})
		}

		srv.AddTool(basemcp.NewTool("task_add",
			basemcp.WithDescription("Create or replace a per-repository task file in Lean format."),
			basemcp.WithString("repo_path", basemcp.Required()),
			basemcp.WithString("title", basemcp.Required()),
			basemcp.WithString("body"),
			basemcp.WithString("status"),
			basemcp.WithString("id"),
			basemcp.WithString("priority"),
			basemcp.WithArray("tags"),
		), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args tasks.AddRequest
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			task, err := taskStore.Add(args)
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
			list, err := taskStore.List(args)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(list)
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
			list, err := taskStore.List(args)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(list)
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
		), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args tasks.UpdateRequest
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			task, err := taskStore.Update(args)
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
			task, err := taskStore.SetStatus(args)
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
			result, err := taskStore.BatchUpsert(args)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(result)
		})
		srv.AddTool(basemcp.NewTool("task_current",
			basemcp.WithDescription("Return active per-repository tasks with todo or in_progress status."),
			basemcp.WithString("repo_path", basemcp.Required()),
		), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			var args tasks.ListRequest
			if err := bind(req, &args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			list, err := taskStore.List(tasks.ListRequest{RepoPath: args.RepoPath})
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(currentTasks(list))
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
			task, err := taskStore.Get(args)
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
			if err := taskStore.Delete(args); err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
			return structured(map[string]bool{"deleted": true})
		})

	}

	return srv
}

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
		if hasString(task.Tags, "issue") || hasString(task.Tags, "feedback") {
			issues = append(issues, task)
		}
	}
	return issues, nil
}

func acceptIssue(store *tasks.Store, req issueAcceptRequest) (tasks.Task, error) {
	return store.SetStatus(tasks.StatusRequest{RepoPath: req.RepoPath, ID: req.ID, Status: "in_progress"})
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func currentTasks(list []tasks.Task) []tasks.Task {
	current := make([]tasks.Task, 0, len(list))
	for _, task := range list {
		switch task.Status {
		case "todo", "in_progress", "blocked":
			current = append(current, task)
		}
	}
	return current
}

func bind(req basemcp.CallToolRequest, target any) error {
	data, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func structured(value any) (*basemcp.CallToolResult, error) {
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return basemcp.NewToolResultStructured(value, string(text)), nil
}
