package mcp

import (
	"context"
	"errors"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/lake"
)

type lakeSmokeRequest struct {
	RepoPath       string `json:"repo_path"`
	Mode           string `json:"mode"`
	File           string `json:"file"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func registerLakeTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("lake_smoke",
		basemcp.WithDescription("Run a compact Lean/Lake workspace smoke check through the bounded command pipeline."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root containing lean-toolchain and lakefile.lean or lakefile.toml.")),
		basemcp.WithString("mode", basemcp.Description("Smoke mode: build or check. Defaults to build.")),
		basemcp.WithString("file", basemcp.Description("Repo-relative Lean file for check mode.")),
		basemcp.WithNumber("timeout_seconds", basemcp.Description("Optional command timeout in seconds.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args lakeSmokeRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, commands, _, _ := deps.loadDeps()
		result, err := runLakeSmoke(ctx, args, commands)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})
}

func runLakeSmoke(ctx context.Context, req lakeSmokeRequest, commands *command.Runner) (lake.CommandResult, error) {
	runner := lake.CommandRunner{Commands: commands, TimeoutSeconds: req.TimeoutSeconds}
	switch strings.TrimSpace(req.Mode) {
	case "", "build":
		return lake.Build(ctx, req.RepoPath, runner)
	case "check":
		if strings.TrimSpace(req.File) == "" {
			return lake.CommandResult{WorkspaceDetected: false, ExitCode: -1, Blocker: "file is required for Lean check mode"}, nil
		}
		return lake.CheckFile(ctx, req.RepoPath, req.File, runner)
	default:
		return lake.CommandResult{}, errors.New("unsupported lake_smoke mode: " + req.Mode)
	}
}
