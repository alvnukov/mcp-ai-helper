package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/lake"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

const leanTaskRegistryExporter = "task_registry_export"

var ErrLeanTaskExporterMissing = errors.New("Lean task registry exporter is not configured")

type leanTaskListPayload struct {
	Tasks []leanTaskProjection `json:"tasks"`
}

type leanTaskProjection struct {
	ID                 string   `json:"id"`
	TaskType           string   `json:"task_type"`
	Branch             string   `json:"branch"`
	WorktreePath       string   `json:"worktree_path"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Priority           string   `json:"priority"`
	ModelLevel         string   `json:"model_level"`
	Tags               []string `json:"tags"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	VerificationPlan   []string `json:"verification_plan"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

var ErrNoLakeWorkspace = errors.New("no Lake workspace detected; run lake_init to bootstrap a Lean project")

func readCurrentTasks(ctx context.Context, repoPath string, commands *command.Runner, _ *tasks.Store) ([]tasks.Task, string, error) {
	leanTasks, usedLean, err := readLeanTaskList(ctx, repoPath, commands, []string{"--list-active"})
	if err != nil {
		return nil, "lean_registry", err
	}
	if !usedLean {
		return nil, "lean_registry", ErrNoLakeWorkspace
	}
	return leanTasks, "lean_registry", nil
}

func readAllTasks(ctx context.Context, repoPath string, commands *command.Runner, _ *tasks.Store) ([]tasks.Task, string, error) {
	leanTasks, usedLean, err := readLeanTaskList(ctx, repoPath, commands, []string{"--list-all"})
	if err != nil {
		return nil, "lean_registry", err
	}
	if !usedLean {
		return nil, "lean_registry", ErrNoLakeWorkspace
	}
	return leanTasks, "lean_registry", nil
}

func readTask(ctx context.Context, repoPath string, id string, commands *command.Runner, _ *tasks.Store) (tasks.Task, string, error) {
	leanTask, usedLean, err := readLeanTask(ctx, repoPath, id, commands)
	if err != nil {
		return tasks.Task{}, "lean_registry", err
	}
	if !usedLean {
		return tasks.Task{}, "lean_registry", ErrNoLakeWorkspace
	}
	return leanTask, "lean_registry", nil
}

func leanTaskExporterConfigured(repoPath string) (bool, error) {
	ws, err := lake.ResolveWorkspace(repoPath)
	if err != nil {
		return false, nil
	}

	activeTasksPath := filepath.Join(ws.Dir, "MCPAIHelperProject", "ActiveTasks.lean")
	if _, err := os.Stat(activeTasksPath); err != nil {
		return false, nil
	}

	exporterPath := filepath.Join(ws.Dir, "MCPAIHelperProject", "TaskRegistryExport.lean")
	if _, err := os.Stat(exporterPath); err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("%w: found MCPAIHelperProject/ActiveTasks.lean but missing MCPAIHelperProject/TaskRegistryExport.lean; add the Lean registry exporter module and declare the %q executable in the Lake config, then rebuild/restart the helper", ErrLeanTaskExporterMissing, leanTaskRegistryExporter)
		}
		return false, fmt.Errorf("inspect Lean task registry exporter: %w", err)
	}

	return true, nil
}

func readLeanTaskList(ctx context.Context, repoPath string, commands *command.Runner, args []string) ([]tasks.Task, bool, error) {
	configured, err := leanTaskExporterConfigured(repoPath)
	if err != nil {
		return nil, true, err
	}
	if !configured {
		return nil, false, nil
	}
	if commands != nil {
		if err := validateLeanRegistryBuild(ctx, repoPath, commands, "before read"); err != nil {
			lake.ResetServerRPC(repoPath)
			return nil, true, fmt.Errorf("Lean task read failed: %w", err)
		}
	}
	active, err := leanTaskListMode(args)
	if err != nil {
		return nil, true, err
	}
	envelope, err := callLeanTaskRead(ctx, repoPath, "MCPAIHelperProject.TaskRegistryExport.taskList", "task.list", map[string]bool{"active": active})
	if err != nil {
		return nil, true, err
	}
	var payload leanTaskListPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, true, fmt.Errorf("decode Lean task list RPC: %w", err)
	}
	out := make([]tasks.Task, 0, len(payload.Tasks))
	for _, item := range payload.Tasks {
		task, err := item.toTask()
		if err != nil {
			return nil, true, err
		}
		out = append(out, tasks.WithWorktreeContext(repoPath, task))
	}
	return out, true, nil
}

func readLeanTask(ctx context.Context, repoPath string, id string, commands *command.Runner) (tasks.Task, bool, error) {
	configured, err := leanTaskExporterConfigured(repoPath)
	if err != nil {
		return tasks.Task{}, true, err
	}
	if !configured {
		return tasks.Task{}, false, nil
	}
	if commands != nil {
		if err := validateLeanRegistryBuild(ctx, repoPath, commands, "before read"); err != nil {
			lake.ResetServerRPC(repoPath)
			return tasks.Task{}, true, fmt.Errorf("Lean task read failed: %w", err)
		}
	}
	envelope, err := callLeanTaskRead(ctx, repoPath, "MCPAIHelperProject.TaskRegistryExport.taskGet", "task.get", map[string]string{"id": id})
	if err != nil {
		return tasks.Task{}, true, err
	}
	var payload leanTaskProjection
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return tasks.Task{}, true, fmt.Errorf("decode Lean task get RPC: %w", err)
	}
	task, err := payload.toTask()
	if err != nil {
		return tasks.Task{}, true, err
	}
	return tasks.WithWorktreeContext(repoPath, task), true, nil
}

func leanTaskListMode(args []string) (bool, error) {
	if len(args) != 1 {
		return false, fmt.Errorf("unsupported Lean task list args: %v", args)
	}
	switch args[0] {
	case "--list-active":
		return true, nil
	case "--list-all":
		return false, nil
	default:
		return false, fmt.Errorf("unsupported Lean task list arg: %s", args[0])
	}
}

func callLeanTaskRead(ctx context.Context, repoPath string, method string, operation string, params any) (leanRegistryEnvelope, error) {
	result, err := lake.CallServerRPC(ctx, repoPath, lake.RPCRequest{SourceFile: "MCPAIHelperProject/TaskRegistryExport.lean", Method: method, Params: params, TimeoutSeconds: 20})
	if err != nil {
		return leanRegistryEnvelope{}, err
	}
	if result.Blocker != "" {
		return leanRegistryEnvelope{}, fmt.Errorf("Lean task read blocker: %s", result.Blocker)
	}
	var envelope leanRegistryEnvelope
	if err := json.Unmarshal(result.Result, &envelope); err != nil {
		return leanRegistryEnvelope{}, fmt.Errorf("decode Lean task read envelope: %w", err)
	}
	if envelope.SchemaVersion != 1 {
		return leanRegistryEnvelope{}, fmt.Errorf("unsupported Lean task read schema_version: %d", envelope.SchemaVersion)
	}
	if envelope.Operation != operation {
		return leanRegistryEnvelope{}, fmt.Errorf("unexpected Lean task read operation: %q", envelope.Operation)
	}
	if !envelope.OK {
		return leanRegistryEnvelope{}, fmt.Errorf("Lean task read rejected: %s", leanRegistryDiagnosticsMessage(envelope.Diagnostics))
	}
	return envelope, nil
}

func (p leanTaskProjection) toTask() (tasks.Task, error) {
	createdAt, err := parseLeanTaskTime(p.CreatedAt)
	if err != nil {
		return tasks.Task{}, fmt.Errorf("parse created_at for %s: %w", p.ID, err)
	}
	updatedAt, err := parseLeanTaskTime(p.UpdatedAt)
	if err != nil {
		return tasks.Task{}, fmt.Errorf("parse updated_at for %s: %w", p.ID, err)
	}
	modelLevel := ""
	if strings.TrimSpace(p.ModelLevel) != "" {
		modelLevel, err = tasks.NormalizeModelLevel(p.ModelLevel)
		if err != nil {
			return tasks.Task{}, err
		}
	}
	return tasks.Task{
		ID:                 p.ID,
		TaskType:           p.TaskType,
		Branch:             p.Branch,
		WorktreePath:       p.WorktreePath,
		Status:             p.Status,
		Title:              p.Title,
		Body:               p.Body,
		Priority:           p.Priority,
		ModelLevel:         modelLevel,
		Tags:               p.Tags,
		AcceptanceCriteria: p.AcceptanceCriteria,
		VerificationPlan:   p.VerificationPlan,
		ProjectionSource:   "lean_registry",
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

func parseLeanTaskTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}
