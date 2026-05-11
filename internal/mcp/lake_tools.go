package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

type lakeInitRequest struct {
	RepoPath string `json:"repo_path"`
}

type lakeInitResult struct {
	Initialized     bool   `json:"initialized"`
	WorkspaceDir    string `json:"workspace_dir,omitempty"`
	Toolchain       string `json:"toolchain,omitempty"`
	LeanVersion     string `json:"lean_version,omitempty"`
	AlreadyExisted  bool   `json:"already_existed,omitempty"`
	LakeBuildPassed bool   `json:"lake_build_passed,omitempty"`
	Blocker         string `json:"blocker,omitempty"`
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
		commands, err := deps.commandRunnerForRepo(args.RepoPath, "lake_smoke")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := runLakeSmoke(ctx, args, commands)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(result)
	})

	srv.AddTool(basemcp.NewTool("lake_init",
		basemcp.WithDescription("Initialize a minimal Lake/Lean workspace in a repository. Creates lean-toolchain, lakefile.lean, and a minimal Main.lean. Idempotent: returns ok if the workspace already exists."),
		basemcp.WithString("repo_path", basemcp.Required(), basemcp.Description("Repository root where the Lake project should be created.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args lakeInitRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		commands, err := deps.commandRunnerForRepo(args.RepoPath, "lake_init")
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		result, err := runLakeInit(ctx, args, commands)
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

func runLakeInit(ctx context.Context, req lakeInitRequest, commands *command.Runner) (lakeInitResult, error) {
	absPath, err := filepath.Abs(strings.TrimSpace(req.RepoPath))
	if err != nil {
		return lakeInitResult{}, fmt.Errorf("resolve repo_path: %w", err)
	}

	toolchainPath := filepath.Join(absPath, "lean-toolchain")
	lakefilePath := filepath.Join(absPath, "lakefile.lean")
	lakefileTomlPath := filepath.Join(absPath, "lakefile.toml")
	mainPath := filepath.Join(absPath, "Bootstrap.lean")

	// Idempotent: if workspace already exists, verify and return.
	if fileExists(toolchainPath) && (fileExists(lakefilePath) || fileExists(lakefileTomlPath)) {
		runner := lake.CommandRunner{Commands: commands, TimeoutSeconds: 30}
		buildResult, err := lake.Build(ctx, absPath, runner)
		passed := err == nil && buildResult.ExitCode == 0
		return lakeInitResult{
			Initialized:     passed,
			WorkspaceDir:    absPath,
			AlreadyExisted:  true,
			LakeBuildPassed: passed,
			Blocker:         buildBlocker(buildResult, err),
		}, nil
	}

	// Detect toolchain
	toolchain, leanVersion, err := detectToolchain(ctx, commands)
	if err != nil {
		return lakeInitResult{Blocker: fmt.Sprintf("cannot detect Lean toolchain: %v", err)}, nil
	}

	// Write lean-toolchain
	if !fileExists(toolchainPath) {
		if err := os.WriteFile(toolchainPath, []byte(toolchain+"\n"), 0644); err != nil {
			return lakeInitResult{Blocker: fmt.Sprintf("write lean-toolchain: %v", err)}, nil
		}
	}

	// Write lakefile.lean only when no Lake file exists.
	if !fileExists(lakefilePath) && !fileExists(lakefileTomlPath) {
		content := `import Lake
open Lake DSL

package bootstrap

@[default_target]
lean_lib Bootstrap
`
		if err := os.WriteFile(lakefilePath, []byte(content), 0644); err != nil {
			return lakeInitResult{Blocker: fmt.Sprintf("write lakefile.lean: %v", err)}, nil
		}
	}

	// Write Bootstrap.lean
	if !fileExists(mainPath) {
		content := `def hello := "world"
`
		if err := os.WriteFile(mainPath, []byte(content), 0644); err != nil {
			return lakeInitResult{Blocker: fmt.Sprintf("write Bootstrap.lean: %v", err)}, nil
		}
	}

	// Verify with lake build
	runner := lake.CommandRunner{Commands: commands, TimeoutSeconds: 60}
	buildResult, err := lake.Build(ctx, absPath, runner)
	passed := err == nil && buildResult.ExitCode == 0

	return lakeInitResult{
		Initialized:     passed,
		WorkspaceDir:    absPath,
		Toolchain:       toolchain,
		LeanVersion:     leanVersion,
		LakeBuildPassed: passed,
		Blocker:         buildBlocker(buildResult, err),
	}, nil
}

func detectToolchain(ctx context.Context, commands *command.Runner) (toolchain string, version string, err error) {
	// Try elan show first
	result, runErr := commands.Run(ctx, "elan show", ".", 10)
	if runErr == nil && result.ExitCode == 0 {
		for _, line := range result.StdoutTail {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if idx := strings.Index(line, " ("); idx >= 0 {
				tc := line[:idx]
				v := extractVersion(tc)
				if v != "" {
					return tc, v, nil
				}
			}
		}
	}

	// Fall back to lake --version
	result, runErr = commands.Run(ctx, "lake --version", ".", 10)
	if runErr == nil && result.ExitCode == 0 {
		for _, line := range result.StdoutTail {
			if idx := strings.Index(line, "Lean version "); idx >= 0 {
				v := strings.TrimSpace(line[idx+len("Lean version "):])
				if end := strings.IndexAny(v, ") \n"); end >= 0 {
					v = v[:end]
				}
				return "leanprover/lean4:v" + v, v, nil
			}
		}
	}

	return "", "", errors.New("elan not available and lake --version did not report Lean version")
}

func extractVersion(toolchain string) string {
	// "leanprover/lean4:v4.29.1" → "4.29.1"
	if idx := strings.LastIndex(toolchain, ":v"); idx >= 0 {
		return toolchain[idx+2:]
	}
	if idx := strings.LastIndex(toolchain, ":"); idx >= 0 {
		return toolchain[idx+1:]
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func buildBlocker(result lake.CommandResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result.ExitCode != 0 {
		if result.Blocker != "" {
			return result.Blocker
		}
		return fmt.Sprintf("lake build failed with exit code %d", result.ExitCode)
	}
	return ""
}
