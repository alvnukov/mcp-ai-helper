package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/pipeline"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestLeanSetStatusUpdatesRegistryAndValidates(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	result, err := setTaskStatus(context.Background(), tasks.StatusRequest{RepoPath: repo, ID: "task-040", Status: "blocked"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("setTaskStatus returned error: %v", err)
	}
	if result.Source != "lean_registry" || result.Validation != "lake build" || result.Task.Status != "blocked" {
		t.Fatalf("unexpected result: %+v", result)
	}
	task, _, err := readTask(context.Background(), repo, "task-040", commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("readTask after mutation: %v", err)
	}
	if task.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", task.Status)
	}
}

func TestLeanUpsertCreatesDeterministicTaskAndValidates(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	result, err := upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-999", Status: "todo", Title: "Generated task", Body: "Created by test", Priority: "high", Tags: []string{"lean", "test"}}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("upsertTask returned error: %v", err)
	}
	if result.Source != "lean_registry" || result.Task.ID != "task-999" {
		t.Fatalf("unexpected result: %+v", result)
	}
	task, _, err := readTask(context.Background(), repo, "task-999", commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("read generated task: %v", err)
	}
	if task.Title != "Generated task" || len(task.Tags) != 2 {
		t.Fatalf("generated task fields not preserved: %+v", task)
	}
}

func TestLeanBatchUpsertClosesMissingActiveTasks(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	result, err := batchUpsertTasks(context.Background(), tasks.BatchUpsertRequest{RepoPath: repo, CloseMissing: true, MissingStatus: "done", Tasks: []tasks.AddRequest{{ID: "task-999", Status: "todo", Title: "Only task"}}}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("batchUpsertTasks returned error: %v", err)
	}
	if result.Source != "lean_registry" || len(result.Upserted) != 1 || len(result.Closed) == 0 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
}

func TestLeanMutationRejectsDuplicateID(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	path := filepath.Join(repo, activeTasksLeanPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\ndef duplicateId : ArtifactId :=\n  { value := \"task-040\" }\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = setTaskStatus(context.Background(), tasks.StatusRequest{RepoPath: repo, ID: "task-040", Status: "done"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err == nil || !strings.Contains(err.Error(), "occurrence count") {
		t.Fatalf("expected duplicate id rejection, got %v", err)
	}
}

func TestLeanMutationRollsBackFailedLakeValidation(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	activePath := filepath.Join(repo, activeTasksLeanPath)
	before, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(repo, "MCPAIHelperProject/Registry.lean")
	registry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registryPath, append(registry, []byte("\n#check (\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-998", Status: "todo", Title: "Validation fails"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err == nil || !strings.Contains(err.Error(), "validate Lean task registry") {
		t.Fatalf("expected validation failure, got %v", err)
	}
	after, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("Lean registry was not rolled back after failed validation")
	}
}

func TestLeanMutationRejectsConflictMarkers(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	path := filepath.Join(repo, activeTasksLeanPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte("<<<<<<< HEAD\n"), data...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = setTaskStatus(context.Background(), tasks.StatusRequest{RepoPath: repo, ID: "task-040", Status: "done"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err == nil || !strings.Contains(err.Error(), "conflict markers") {
		t.Fatalf("expected conflict marker rejection, got %v", err)
	}
}

func TestLeanMutationRequiresLeanRegistry(t *testing.T) {
	repo := t.TempDir()
	_, err := upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-999", Status: "todo", Title: "No fallback"}, nil, legacyStoreForTest(t))
	if err == nil || !strings.Contains(err.Error(), "Lake workspace blocker") {
		t.Fatalf("expected missing Lean workspace blocker, got %v", err)
	}
}

func copyLeanRepoFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, dir := range []string{"MCPAIHelperProject"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o700); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
	}
	for _, file := range []string{"lean-toolchain", "lakefile.lean", "MCPAIHelperProject.lean", "MCPAIHelperProject/ProjectState.lean", "MCPAIHelperProject/Samples.lean", "MCPAIHelperProject/ActiveTasks.lean", "MCPAIHelperProject/Registry.lean", "MCPAIHelperProject/TaskRegistryExport.lean"} {
		data, err := os.ReadFile(filepath.Join("../..", file))
		if err != nil {
			t.Fatalf("read fixture source %s: %v", file, err)
		}
		if err := os.WriteFile(filepath.Join(repo, file), data, 0o600); err != nil {
			t.Fatalf("write fixture file %s: %v", file, err)
		}
	}
	return repo
}

func TestLeanMutationCommandRunnerPolicy(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	runner := command.NewRunner(config.CommandPolicy{AllowedCWDs: []string{repo}, DefaultTimeoutSeconds: 20, MaxOutputBytes: 20000, MaxLines: 80})
	if runner == nil {
		t.Fatal("runner is nil")
	}
}

func TestRunWorkflowTaskTransitionUsesLeanRegistry(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{},
		Models:    map[string]config.ModelConfig{},
		CommandPolicy: config.CommandPolicy{
			AllowedCWDs:           []string{repo},
			DefaultTimeoutSeconds: 20,
			MaxOutputBytes:        20000,
			MaxLines:              80,
		},
	}
	_, commands, workflows, store := buildDeps(cfg)
	if _, err := store.Add(tasks.AddRequest{RepoPath: repo, ID: "task-043", Title: "Legacy shadow", Status: "done"}); err != nil {
		t.Fatalf("write legacy shadow: %v", err)
	}
	if _, err := setTaskStatus(context.Background(), tasks.StatusRequest{RepoPath: repo, ID: "task-043", Status: "todo"}, commands, store); err != nil {
		t.Fatalf("prepare canonical task status: %v", err)
	}

	result, err := workflows.RunWorkflow(context.Background(), pipeline.WorkflowRequest{
		RepoPath: repo,
		Steps: []pipeline.WorkflowStep{{
			ID:   "transition",
			Tool: "task_transition",
			Args: map[string]any{"task_ids": []string{"task-043"}, "from": "todo", "to": "done"},
		}},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q reason = %q", result.Status, result.Reason)
	}
	got, source, err := readTask(context.Background(), repo, "task-043", commands, store)
	if err != nil {
		t.Fatalf("read canonical task: %v", err)
	}
	if source != "lean_registry" || got.Status != "done" || got.Title == "Legacy shadow" {
		t.Fatalf("workflow did not use Lean registry: source=%q task=%#v", source, got)
	}
}
