package mcp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
)

// projectHealthResult holds structured project health check output.
type projectHealthResult struct {
	Status   string   `json:"status"` // ok, fail
	Build    string   `json:"build"`  // ok, fail
	Vet      string   `json:"vet"`    // ok, fail
	Test     string   `json:"test"`   // ok, fail
	Duration string   `json:"duration"`
	Errors   []string `json:"errors,omitempty"`
}

// checkProjectHealth runs build, vet, and test for a Go project using command.Runner.
func checkProjectHealth(ctx context.Context, runner *command.Runner, repoPath string) (projectHealthResult, error) {
	if strings.TrimSpace(repoPath) == "" {
		return projectHealthResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(repoPath)
	if err != nil {
		return projectHealthResult{}, err
	}

	start := time.Now()
	result := projectHealthResult{
		Build:  "ok",
		Vet:    "ok",
		Test:   "ok",
		Status: "ok",
	}

	// Build
	buildRes, buildErr := runner.RunFilteredInRepoWithWait(ctx, "go build ./...", repo, "", 60, 0, command.Filter{})
	if buildErr != nil {
		return projectHealthResult{}, buildErr
	}
	if buildRes.ExitCode != 0 {
		result.Build = "fail"
		result.Status = "fail"
		result.Errors = append(result.Errors, "build: "+strings.Join(buildRes.StderrTail, "\n"))
	}

	// Vet
	vetRes, vetErr := runner.RunFilteredInRepoWithWait(ctx, "go vet ./...", repo, "", 60, 0, command.Filter{})
	if vetErr != nil {
		return projectHealthResult{}, vetErr
	}
	if vetRes.ExitCode != 0 {
		result.Vet = "fail"
		result.Status = "fail"
		result.Errors = append(result.Errors, "vet: "+strings.Join(vetRes.StderrTail, "\n"))
	}

	// Test
	testRes, testErr := runner.RunFilteredInRepoWithWait(ctx, "go test -count=1 -timeout=60s ./...", repo, "", 120, 0, command.Filter{})
	if testErr != nil {
		return projectHealthResult{}, testErr
	}
	if testRes.ExitCode != 0 {
		result.Test = "fail"
		result.Status = "fail"
		result.Errors = append(result.Errors, "test: tests failed")
	}

	result.Duration = time.Since(start).Round(time.Millisecond).String()
	return result, nil
}

func registerCommandTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("collect_command_output",
		basemcp.WithDescription("Run a command under policy limits and extract compact evidence lines."),
		basemcp.WithString("command", basemcp.Required(), basemcp.Description("Shell command.")),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root from the calling LLM.")),
		basemcp.WithString("cwd", basemcp.Description("Optional repo-relative working directory.")),
		basemcp.WithNumber("timeout_seconds", basemcp.Description("Optional execution timeout in seconds.")),
		basemcp.WithNumber("mcp_wait_seconds", basemcp.Description("Optional MCP wait budget before returning running + command_id.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			Command        string         `json:"command"`
			RepoPath       string         `json:"repo_path"`
			CWD            string         `json:"cwd"`
			TimeoutSeconds int            `json:"timeout_seconds"`
			MCPWaitSeconds int            `json:"mcp_wait_seconds"`
			Filter         command.Filter `json:"filter"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cmds, err := deps.commandRunnerForRepo(args.RepoPath, "collect_command_output")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := cmds.RunFilteredInRepoWithWait(ctx, args.Command, args.RepoPath, args.CWD, args.TimeoutSeconds, args.MCPWaitSeconds, args.Filter)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("cleanup_command_history",
		basemcp.WithDescription("Remove command log records that exceed retention policy (age, max records)."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		_, _, cmds, _, _ := deps.loadDeps()
		if err := cmds.CleanupHistory(); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return basemcp.NewToolResultText("cleanup complete"), nil
	})

	srv.AddTool(basemcp.NewTool("command_abort",
		basemcp.WithDescription("Abort a running command by command_id. Kills the process group."),
		basemcp.WithString("command_id", basemcp.Required(), basemcp.Description("Command ID to abort.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			CommandID string `json:"command_id"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, cmds, _, _ := deps.loadDeps()
		result, err := cmds.Abort(args.CommandID)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("command_list",
		basemcp.WithDescription("List recent command history entries, optionally filtered by status and repo."),
		basemcp.WithString("repo_path", basemcp.Description("Optional repo_path filter.")),
		basemcp.WithString("status", basemcp.Description("Optional status filter: running, ok, error.")),
		basemcp.WithNumber("limit", basemcp.Description("Max entries to return (default 50, max 200).")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			RepoPath string `json:"repo_path"`
			Status   string `json:"status"`
			Limit    int    `json:"limit"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, cmds, _, _ := deps.loadDeps()
		result, err := cmds.ListCommands(command.ListRequest{
			Status:  args.Status,
			RepoPath: args.RepoPath,
			Limit:   args.Limit,
		})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("command_get",
		basemcp.WithDescription("Return durable command status/result by command_id without rerunning the command."),
		basemcp.WithString("command_id", basemcp.Required()),
		basemcp.WithString("mode", basemcp.Description("Optional mode: status, result, tail, or evidence. Output remains bounded.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			CommandID string         `json:"command_id"`
			Mode      string         `json:"mode"`
			Filter    command.Filter `json:"filter"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, cmds, _, _ := deps.loadDeps()
		result, err := cmds.FilterHistory(args.CommandID, args.Filter)
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
		_, _, cmds, _, _ := deps.loadDeps()
		result, err := cmds.FilterHistory(args.CommandID, args.Filter)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("project_health",
		basemcp.WithDescription("Quick project health check: build, vet, test. Returns structured pass/fail per step."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			RepoPath string `json:"repo_path"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cmds, err := deps.commandRunnerForRepo(args.RepoPath, "project_health")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := checkProjectHealth(ctx, cmds, args.RepoPath)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
}
