package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/lake"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

const leanTaskRegistryExporter = "task_registry_export"

type leanTaskListPayload struct {
	Tasks []leanTaskProjection `json:"tasks"`
}

type leanTaskProjection struct {
	ID           string   `json:"id"`
	TaskType     string   `json:"task_type"`
	Branch       string   `json:"branch"`
	WorktreePath string   `json:"worktree_path"`
	Status       string   `json:"status"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	Priority     string   `json:"priority"`
	ModelLevel   string   `json:"model_level"`
	Tags         []string `json:"tags"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
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

func readLeanTaskList(ctx context.Context, repoPath string, commands *command.Runner, args []string) ([]tasks.Task, bool, error) {
	result, usedLean, err := runLeanTaskExporter(ctx, repoPath, commands, args)
	if err != nil || !usedLean {
		return nil, usedLean, err
	}
	var payload leanTaskListPayload
	if err := json.Unmarshal([]byte(strings.Join(result.Output, "\n")), &payload); err != nil {
		return nil, true, fmt.Errorf("decode Lean task export: %w", err)
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
	result, usedLean, err := runLeanTaskExporter(ctx, repoPath, commands, []string{"--get", id})
	if err != nil || !usedLean {
		return tasks.Task{}, usedLean, err
	}
	var payload leanTaskProjection
	if err := json.Unmarshal([]byte(strings.Join(result.Output, "\n")), &payload); err != nil {
		return tasks.Task{}, true, fmt.Errorf("decode Lean task export: %w", err)
	}
	task, err := payload.toTask()
	if err != nil {
		return tasks.Task{}, true, err
	}
	return tasks.WithWorktreeContext(repoPath, task), true, nil
}

func runLeanTaskExporter(ctx context.Context, repoPath string, commands *command.Runner, args []string) (lake.CommandResult, bool, error) {
	result, err := lake.RunExe(ctx, repoPath, leanTaskRegistryExporter, args, lake.CommandRunner{Commands: commands, TimeoutSeconds: 20})
	if err != nil {
		return lake.CommandResult{}, false, err
	}
	if !result.WorkspaceDetected {
		return result, false, nil
	}
	if result.ExitCode != 0 {
		diagnostic := strings.TrimSpace(strings.Join(result.Diagnostics, "\n"))
		if diagnostic == "" {
			diagnostic = result.Blocker
		}
		if diagnostic == "" {
			diagnostic = "Lean task exporter failed"
		}
		return result, true, fmt.Errorf("Lean task exporter failed: %s", diagnostic)
	}
	if len(result.Output) == 0 {
		return result, true, fmt.Errorf("Lean task exporter produced no JSON output")
	}
	return result, true, nil
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
		ID:               p.ID,
		TaskType:         p.TaskType,
		Branch:           p.Branch,
		WorktreePath:     p.WorktreePath,
		Status:           p.Status,
		Title:            p.Title,
		Body:             p.Body,
		Priority:         p.Priority,
		ModelLevel:       modelLevel,
		Tags:             p.Tags,
		ProjectionSource: "lean_registry",
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}

func parseLeanTaskTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, value)
}
