package lake

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/command"
)

// Workspace is a repo-local Lean/Lake project root.
type Workspace struct {
	Dir string `json:"dir"`
}

// CommandResult is the compact, MCP-facing shape returned by the backend.
type CommandResult struct {
	WorkspaceDetected bool     `json:"workspace_detected"`
	WorkspaceDir      string   `json:"workspace_dir,omitempty"`
	Command           []string `json:"command,omitempty"`
	ExitCode          int      `json:"exit_code"`
	Output            []string `json:"output,omitempty"`
	Diagnostics       []string `json:"diagnostics,omitempty"`
	Blocker           string   `json:"blocker,omitempty"`
}

// Runner executes a Lake command through the helper's bounded command layer.
type Runner interface {
	Run(ctx context.Context, workspaceDir string, args []string) (CommandResult, error)
}

// CommandRunner adapts internal/command.Runner for Lake checks.
type CommandRunner struct {
	Commands       *command.Runner
	TimeoutSeconds int
}

func ResolveWorkspace(repoPath string) (Workspace, error) {
	if strings.TrimSpace(repoPath) == "" {
		return Workspace{}, errors.New("repo_path is required")
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve repo_path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Workspace{}, fmt.Errorf("Lake workspace blocker: repo_path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return Workspace{}, fmt.Errorf("Lake workspace blocker: repo_path is not a directory: %s", abs)
	}
	if _, err := os.Stat(filepath.Join(abs, "lean-toolchain")); err != nil {
		return Workspace{}, errors.New("Lake workspace blocker: missing lean-toolchain")
	}
	if _, err := os.Stat(filepath.Join(abs, "lakefile.lean")); err == nil {
		return Workspace{Dir: abs}, nil
	}
	if _, err := os.Stat(filepath.Join(abs, "lakefile.toml")); err == nil {
		return Workspace{Dir: abs}, nil
	}
	return Workspace{}, errors.New("Lake workspace blocker: missing lakefile.lean or lakefile.toml")
}

func Build(ctx context.Context, repoPath string, runner Runner) (CommandResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return blockerResult(err), nil
	}
	if runner == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake runner is not configured"}, nil
	}
	return runner.Run(ctx, ws.Dir, []string{"lake", "build"})
}

func CheckFile(ctx context.Context, repoPath string, relLeanFile string, runner Runner) (CommandResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return blockerResult(err), nil
	}
	relClean := filepath.Clean(relLeanFile)
	if filepath.IsAbs(relClean) || relClean == "." || strings.HasPrefix(relClean, ".."+string(os.PathSeparator)) || relClean == ".." {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lean file must be repo-relative and inside the workspace"}, nil
	}
	if filepath.Ext(relClean) != ".lean" {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lean file must have .lean extension"}, nil
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, relClean)); err != nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lean file is not accessible: " + relClean}, nil
	}
	if runner == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake runner is not configured"}, nil
	}
	return runner.Run(ctx, ws.Dir, []string{"lake", "env", "lean", relClean})
}

func RunExe(ctx context.Context, repoPath string, exeName string, exeArgs []string, runner Runner) (CommandResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return blockerResult(err), nil
	}
	exe := strings.TrimSpace(exeName)
	if exe == "" || strings.ContainsAny(exe, `/\`) {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake executable name must be non-empty and not contain a path"}, nil
	}
	if runner == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake runner is not configured"}, nil
	}
	args := append([]string{"lake", "exe", exe}, exeArgs...)
	return runner.Run(ctx, ws.Dir, args)
}

func (r CommandRunner) Run(ctx context.Context, workspaceDir string, args []string) (CommandResult, error) {
	if r.Commands == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: workspaceDir, ExitCode: -1, Blocker: "command runner is not configured"}, nil
	}
	if len(args) == 0 {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: workspaceDir, ExitCode: -1, Blocker: "empty Lake command"}, nil
	}
	result, err := r.Commands.RunFilteredInRepo(ctx, shellQuote(args), workspaceDir, "", r.TimeoutSeconds, command.Filter{Keywords: []string{"error", "warning", "failed", "unknown", "invalid", "not found"}, CaseInsensitive: true, MaxLines: 40})
	if err != nil {
		return CommandResult{}, err
	}
	diagnostics := result.FilteredLines
	if len(diagnostics) == 0 {
		diagnostics = FilterDiagnostics(strings.Join(append(result.StdoutTail, result.StderrTail...), "\n"))
	}
	return CommandResult{WorkspaceDetected: true, WorkspaceDir: workspaceDir, Command: append([]string(nil), args...), ExitCode: result.ExitCode, Output: append([]string(nil), result.StdoutTail...), Diagnostics: diagnostics}, nil
}

func FilterDiagnostics(output string) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	diagnostics := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" {
			continue
		}
		if strings.Contains(lower, "error") || strings.Contains(lower, "warning") || strings.Contains(lower, "failed") || strings.Contains(lower, "unknown") || strings.Contains(lower, "invalid") || strings.Contains(lower, "not found") {
			diagnostics = append(diagnostics, trimmed)
			if len(diagnostics) == 40 {
				break
			}
		}
	}
	if len(diagnostics) == 0 {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				diagnostics = append(diagnostics, trimmed)
				if len(diagnostics) == 12 {
					break
				}
			}
		}
	}
	return diagnostics
}

func blockerResult(err error) CommandResult {
	return CommandResult{WorkspaceDetected: false, ExitCode: -1, Blocker: err.Error()}
}

func shellQuote(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			parts = append(parts, "''")
			continue
		}
		if strings.IndexFunc(arg, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' || r == '/' || r == ':')
		}) == -1 {
			parts = append(parts, arg)
			continue
		}
		parts = append(parts, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
	}
	return strings.Join(parts, " ")
}
