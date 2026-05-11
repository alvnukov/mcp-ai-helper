package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/project"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestReadCurrentTasksPrefersLeanExporter(t *testing.T) {
	repoRoot := filepath.Clean("../..")
	list, source, err := readCurrentTasks(context.Background(), repoRoot, commandRunnerForRepo(repoRoot), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("readCurrentTasks returned error: %v", err)
	}
	if source != "lean_registry" {
		t.Fatalf("source = %q, want lean_registry", source)
	}
	if !containsTaskWithSource(list, "task-006", "lean_registry") {
		t.Fatalf("task-006 missing from Lean current tasks: %#v", list)
	}
}

func TestReadTaskPrefersLeanExporter(t *testing.T) {
	repoRoot := filepath.Clean("../..")
	task, source, err := readTask(context.Background(), repoRoot, "task-006", commandRunnerForRepo(repoRoot), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("readTask returned error: %v", err)
	}
	if source != "lean_registry" || task.ProjectionSource != "lean_registry" {
		t.Fatalf("unexpected source: source=%q task=%#v", source, task)
	}
	if task.ID != "task-006" || task.Body == "" || len(task.Tags) == 0 {
		t.Fatalf("core fields were not projected from Lean: %#v", task)
	}
	if task.ModelLevel != "" {
		t.Fatalf("model_level for unmigrated task = %q, want empty", task.ModelLevel)
	}
	if task.WorktreePath != ".worktrees/task-006" {
		t.Fatalf("worktree_path = %q", task.WorktreePath)
	}
	if !strings.HasSuffix(task.CodePath, filepath.Join(".worktrees", "task-006")) {
		t.Fatalf("code_path = %q", task.CodePath)
	}
}

func TestReadTaskExporterFailureDoesNotFallbackToLegacy(t *testing.T) {
	repoRoot := filepath.Clean("../..")
	_, source, err := readTask(context.Background(), repoRoot, "missing-task", commandRunnerForRepo(repoRoot), legacyStoreForTest(t))
	if err == nil {
		t.Fatal("expected missing Lean task error")
	}
	if source != "lean_registry" || !strings.Contains(err.Error(), "task not found") {
		t.Fatalf("unexpected error source=%q err=%v", source, err)
	}
}

func TestReadCurrentTasksRequiresLeanRegistry(t *testing.T) {
	repo := t.TempDir()
	_, source, err := readCurrentTasks(context.Background(), repo, nil, legacyStoreForTest(t))
	if err == nil {
		t.Fatal("expected missing Lean registry error")
	}
	if source != "lean_registry" || !errors.Is(err, ErrNoLakeWorkspace) {
		t.Fatalf("unexpected source=%q err=%v", source, err)
	}
}

func commandRunnerForRepo(repoRoot string) *command.Runner {
	return command.NewRunner(config.CommandPolicy{AllowedCWDs: []string{repoRoot}, DefaultTimeoutSeconds: 20, MaxOutputBytes: 20000, MaxLines: 80})
}

func legacyStoreForTest(t *testing.T) *tasks.Store {
	t.Helper()
	root := filepath.Join(t.TempDir(), "helper")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("create helper root: %v", err)
	}
	projects, err := project.NewStore(root)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}
	return tasks.NewStore(projects)
}

func containsTaskWithSource(list []tasks.Task, id string, source string) bool {
	for _, task := range list {
		if task.ID == id && task.ProjectionSource == source {
			return true
		}
	}
	return false
}

func TestReadCurrentTasksReportsProjectionSource(t *testing.T) {
	repoRoot := filepath.Clean("../..")
	list, source, err := readCurrentTasks(context.Background(), repoRoot, commandRunnerForRepo(repoRoot), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("readCurrentTasks returned error: %v", err)
	}
	if source != "lean_registry" {
		t.Fatalf("source = %q, want lean_registry", source)
	}
	for _, task := range list {
		if task.ProjectionSource != "lean_registry" {
			t.Fatalf("task %s projection_source = %q", task.ID, task.ProjectionSource)
		}
	}
}
