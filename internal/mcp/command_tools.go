package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
)

func registerCommandTools(srv *server.MCPServer, deps *Server) {
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
		cmds, err := deps.commandRunnerForRepo(args.RepoPath, "collect_command_output")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := cmds.RunFilteredInRepo(ctx, args.Command, args.RepoPath, args.CWD, args.TimeoutSeconds, args.Filter)
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
}
