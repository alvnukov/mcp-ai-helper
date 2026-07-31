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

type commandFilterArgs struct {
	Filter        command.Filter `json:"filter"`
	Include       string         `json:"include"`
	Exclude       string         `json:"exclude"`
	Preset        string         `json:"preset"`
	MaxLines      int            `json:"max_lines"`
	ContextBefore int            `json:"context_before"`
	ContextAfter  int            `json:"context_after"`
}

func (a commandFilterArgs) filter() command.Filter {
	filter := a.Filter
	if a.Include != "" {
		filter.Include = a.Include
	}
	if a.Exclude != "" {
		filter.Exclude = a.Exclude
	}
	if a.Preset != "" {
		filter.Preset = a.Preset
	}
	if a.MaxLines != 0 {
		filter.MaxLines = a.MaxLines
	}
	if a.ContextBefore != 0 {
		filter.ContextBefore = a.ContextBefore
	}
	if a.ContextAfter != 0 {
		filter.ContextAfter = a.ContextAfter
	}
	return filter
}

func registerCommandTools(srv *server.MCPServer, deps *Server) {
	commandActions := actions{
		"run":     withDeps(commandActionRun, deps),
		"cleanup": withDeps(commandActionCleanup, deps),
		"abort":   withDeps(commandActionAbort, deps),
		"list":    withDeps(commandActionList, deps),
		"get":     withDeps(commandActionGet, deps),
		"filter":  withDeps(commandActionFilter, deps),
		"health":  withDeps(commandActionHealth, deps),
	}
	srv.AddTool(basemcp.NewTool("command",
		basemcp.WithDescription("Command execution and history management. Required: action. Actions: run (command, repo_path, cwd?, timeout_seconds?, mcp_wait_seconds?) — run a command under policy limits; cleanup () — remove old command log records; abort (command_id) — abort a running command; list (repo_path?, status?, limit?) — list recent command history; get (command_id, mode?, include?, exclude?, preset?, max_lines?, context_before?, context_after?) — get durable command status/result; filter (command_id, include?, exclude?, preset?, max_lines?, context_before?, context_after?) — grep retained command output; health (repo_path) — quick build/vet/test check."),
		basemcp.WithString("action", basemcp.Required(), actionEnum(commandActions)),
		basemcp.WithString("command", basemcp.Description("Shell command. Required for run.")),
		basemcp.WithString("repo_path", basemcp.Description("Repository root. Required for run and health; optional for list.")),
		basemcp.WithString("cwd", basemcp.Description("Optional repo-relative working directory (run action).")),
		basemcp.WithString("command_id", basemcp.Description("Command ID. Required for abort, get, filter.")),
		basemcp.WithString("mode", basemcp.Description("Mode for get: status, result, tail, or evidence.")),
		basemcp.WithString("status", basemcp.Description("Status filter for list: running, ok, error.")),
		basemcp.WithNumber("limit", basemcp.Description("Max entries for list (default 50, max 200).")),
		basemcp.WithNumber("timeout_seconds", basemcp.Description("Execution timeout in seconds (run action).")),
		basemcp.WithNumber("mcp_wait_seconds", basemcp.Description("MCP wait budget before returning running + command_id (run action).")),
		basemcp.WithString("include", basemcp.Description("Regex include pattern (get/filter).")),
		basemcp.WithString("exclude", basemcp.Description("Regex exclude pattern applied after include (get/filter).")),
		basemcp.WithString("preset", basemcp.Description("Filter preset: errors-only, test-failures, compile-errors, git-status, changed-files, summary-with-context (get/filter).")),
		basemcp.WithNumber("max_lines", basemcp.Description("Maximum filtered lines to return; default 80 (get/filter).")),
		basemcp.WithNumber("context_before", basemcp.Description("Lines of context before each match (get/filter).")),
		basemcp.WithNumber("context_after", basemcp.Description("Lines of context after each match (get/filter).")),
		basemcp.WithObject("filter", basemcp.Description("Structured filter object (run action).")),
	), dispatch("command", commandActions))
}

func commandActionRun(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
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
	if args.Command == "" {
		return basemcp.NewToolResultError("command action=run requires command"), nil
	}
	if args.RepoPath == "" {
		return basemcp.NewToolResultError("command action=run requires repo_path"), nil
	}
	cmds, err := deps.commandRunnerForRepo(args.RepoPath, "command action=run")
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := cmds.RunFilteredInRepoWithWait(ctx, args.Command, args.RepoPath, args.CWD, args.TimeoutSeconds, args.MCPWaitSeconds, args.Filter)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func commandActionCleanup(_ context.Context, _ basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	_, _, cmds, _, _ := deps.loadDeps()
	if err := cmds.CleanupHistory(); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return basemcp.NewToolResultText("cleanup complete"), nil
}

func commandActionAbort(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args struct {
		CommandID string `json:"command_id"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.CommandID == "" {
		return basemcp.NewToolResultError("command action=abort requires command_id"), nil
	}
	_, _, cmds, _, _ := deps.loadDeps()
	result, err := cmds.Abort(args.CommandID)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func commandActionList(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
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
		Status:   args.Status,
		RepoPath: args.RepoPath,
		Limit:    args.Limit,
	})
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func commandActionGet(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args struct {
		CommandID string `json:"command_id"`
		Mode      string `json:"mode"`
		commandFilterArgs
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.CommandID == "" {
		return basemcp.NewToolResultError("command action=get requires command_id"), nil
	}
	_, _, cmds, _, _ := deps.loadDeps()
	result, err := cmds.FilterHistory(args.CommandID, args.filter())
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func commandActionFilter(_ context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args struct {
		CommandID string `json:"command_id"`
		commandFilterArgs
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.CommandID == "" {
		return basemcp.NewToolResultError("command action=filter requires command_id"), nil
	}
	_, _, cmds, _, _ := deps.loadDeps()
	result, err := cmds.FilterHistory(args.CommandID, args.filter())
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}

func commandActionHealth(ctx context.Context, req basemcp.CallToolRequest, deps *Server) (*basemcp.CallToolResult, error) {
	var args struct {
		RepoPath string `json:"repo_path"`
	}
	if err := bind(req, &args); err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	if args.RepoPath == "" {
		return basemcp.NewToolResultError("command action=health requires repo_path"), nil
	}
	cmds, err := deps.commandRunnerForRepo(args.RepoPath, "command action=health")
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	result, err := checkProjectHealth(ctx, cmds, args.RepoPath)
	if err != nil {
		return basemcp.NewToolResultError(err.Error()), nil
	}
	return structured(result)
}
