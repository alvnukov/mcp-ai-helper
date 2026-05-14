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
	repo := seedLeanTestFixture(t)
	result, err := setTaskStatus(context.Background(), tasks.StatusRequest{RepoPath: repo, ID: "task-040", Status: "blocked"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("setTaskStatus returned error: %v", err)
	}
	if result.Source != "lean_registry" || !strings.Contains(result.Validation, "server-side transition applied") || result.Task.Status != "blocked" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != activeTasksLeanPath {
		t.Fatalf("unexpected changed files: %#v", result.ChangedFiles)
	}
	task, _, err := readTask(context.Background(), repo, "task-040", commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("readTask after mutation: %v", err)
	}
	if task.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", task.Status)
	}
}

func TestLeanTransitionServerRejectsInvalidStatusWithTypedDiagnostic(t *testing.T) {
	repo := seedLeanTestFixture(t)
	_, _, err := validateLeanTaskTransitionWithServer(context.Background(), repo, tasks.StatusRequest{RepoPath: repo, ID: "task-040", Status: "not-a-status"}, tasks.Task{ID: "task-040", Status: "not-a-status"})
	if err == nil || !strings.Contains(err.Error(), "invalid_status") {
		t.Fatalf("expected typed invalid_status rejection, got %v", err)
	}
}

func TestLeanUpsertBootstrapsEmptyTaskRepo(t *testing.T) {
	repo := t.TempDir()
	result, err := upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-first", Status: "todo", Title: "First task"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("upsertTask returned error: %v", err)
	}
	if result.Source != "lean_registry" || result.Task.ID != "task-first" {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}
	for _, path := range []string{"lean-toolchain", "lakefile.lean", "MCPAIHelperProject/ActiveTasks.lean", "MCPAIHelperProject/TaskRegistryExport.lean"} {
		if _, err := os.Stat(filepath.Join(repo, path)); err != nil {
			t.Fatalf("bootstrap did not create %s: %v", path, err)
		}
	}
	task, _, err := readTask(context.Background(), repo, "task-first", commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("read bootstrapped task: %v", err)
	}
	if task.Title != "First task" {
		t.Fatalf("unexpected bootstrapped task: %+v", task)
	}
}

func TestLeanUpsertCreatesDeterministicTaskAndValidates(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	result, err := upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-999", Status: "todo", Title: "Generated task", Body: "Created by test", Priority: "high", ModelLevel: "very_high", Tags: []string{"lean", "test"}}, commandRunnerForRepo(repo), legacyStoreForTest(t))
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
	if task.Title != "Generated task" || len(task.Tags) != 2 || task.ModelLevel != "very_high" {
		t.Fatalf("generated task fields not preserved: %+v", task)
	}
}

func TestLeanUpsertPersistsParentIDRelation(t *testing.T) {
	repo := seedLeanTestFixture(t)
	result, err := upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-child-parent", ParentID: "task-040", Status: "todo", Title: "Child task"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("upsertTask returned error: %v", err)
	}
	if result.Task.ParentID != "task-040" {
		t.Fatalf("upsert result parent_id = %q, want task-040", result.Task.ParentID)
	}

	child, _, err := readTask(context.Background(), repo, "task-child-parent", commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("read generated child: %v", err)
	}
	if child.ParentID != "task-040" {
		t.Fatalf("read child parent_id = %q, want task-040", child.ParentID)
	}

	all, _, err := readAllTasks(context.Background(), repo, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("read all tasks: %v", err)
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: repo, FocusTaskID: "task-040", MaxNodes: 20})
	if err != nil {
		t.Fatalf("build graph: %v", err)
	}
	foundEdge := false
	for _, edge := range graph.Edges {
		if edge.From == "task-040" && edge.To == "task-child-parent" && edge.Kind == "parent_child" && edge.Provenance == "explicit" {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Fatalf("focused graph missing explicit parent_child edge: %+v", graph)
	}
}

func TestLeanBatchUpsertClosesMissingActiveTasks(t *testing.T) {
	repo := seedLeanTestFixture(t)
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
	if err == nil || !strings.Contains(err.Error(), "validate Lean task registry before transition") {
		t.Fatalf("expected Lean build rejection, got %v", err)
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
	for _, file := range []string{"lean-toolchain", "lakefile.lean", "MCPAIHelperProject.lean", "MCPAIHelperProject/ProjectState.lean", "MCPAIHelperProject/Samples.lean", "MCPAIHelperProject/Registry.lean", "MCPAIHelperProject/TaskRegistryExport.lean"} {
		data, err := taskRegistryBootstrapTemplates.ReadFile("task_registry_templates/" + file)
		if err != nil {
			t.Fatalf("read embedded fixture %s: %v", file, err)
		}
		if err := os.WriteFile(filepath.Join(repo, file), data, 0o600); err != nil {
			t.Fatalf("write fixture file %s: %v", file, err)
		}
	}
	activePath := filepath.Join(repo, "MCPAIHelperProject", "ActiveTasks.lean")
	if err := os.WriteFile(activePath, []byte(emptyActiveTasksLeanSource), 0o600); err != nil {
		t.Fatalf("write empty ActiveTasks.lean: %v", err)
	}
	return repo
}

func seedLeanTestFixture(t *testing.T) string {
	t.Helper()
	repo := copyLeanRepoFixture(t)
	runner := commandRunnerForRepo(repo)
	store := legacyStoreForTest(t)
	seeds := []tasks.AddRequest{
		{RepoPath: repo, ID: "task-006", Status: "blocked", Title: "Test task 006", Body: "Test body for task-006", Priority: "high", ModelLevel: "high", Tags: []string{"test"}, TaskType: "feature", WorktreePath: ".worktrees/task-006"},
		{RepoPath: repo, ID: "task-040", Status: "done", Title: "Test task 040", Body: "Test body for task-040", Priority: "critical", Tags: []string{"tasks"}, TaskType: "feature"},
		{RepoPath: repo, ID: "task-043", Status: "done", Title: "Test task 043", Body: "Test body for task-043", Priority: "high", Tags: []string{"cleanup"}, TaskType: "feature"},
	}
	for _, seed := range seeds {
		if _, err := upsertTask(context.Background(), seed, runner, store); err != nil {
			t.Fatalf("seed %s: %v", seed.ID, err)
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
	repo := seedLeanTestFixture(t)
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
	_, commands, workflows, store, _ := buildDeps(cfg)
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
	if source != "lean_registry" || got.Status != "done" {
		t.Fatalf("workflow did not use Lean registry: source=%q task=%#v", source, got)
	}
}

func TestRunWorkflowCurrentTaskIDBlocksLeanTaskOnSkippedGate(t *testing.T) {
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
	_, commands, workflows, store, _ := buildDeps(cfg)
	if _, err := upsertTask(context.Background(), tasks.AddRequest{RepoPath: repo, ID: "task-999", Status: "todo", Title: "Skipped gate fixture"}, commands, store); err != nil {
		t.Fatalf("create canonical task: %v", err)
	}

	result, err := workflows.RunWorkflow(context.Background(), pipeline.WorkflowRequest{
		RepoPath:      repo,
		CurrentTaskID: "task-999",
		Steps: []pipeline.WorkflowStep{{
			ID:   "skipped-gate",
			Tool: "command",
			If:   "file_exists missing-gate.txt",
			Args: map[string]any{"command": "printf should-not-run"},
		}},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q reason = %q", result.Status, result.Reason)
	}
	got, source, err := readTask(context.Background(), repo, "task-999", commands, store)
	if err != nil {
		t.Fatalf("read canonical task: %v", err)
	}
	if source != "lean_registry" || got.Status != "blocked" {
		t.Fatalf("skipped gate should block task closeout: source=%q task=%#v", source, got)
	}
}

func TestLeanDeleteRemovesTaskAndValidates(t *testing.T) {
	repo := seedLeanTestFixture(t)
	result, err := deleteTask(context.Background(), tasks.DeleteRequest{RepoPath: repo, ID: "task-040"}, commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err != nil {
		t.Fatalf("deleteTask returned error: %v", err)
	}
	if result.Source != "lean_registry" || result.Task.ID != "task-040" || !strings.Contains(result.Validation, "server-side delete applied") {
		t.Fatalf("unexpected delete result: %+v", result)
	}
	_, _, err = readTask(context.Background(), repo, "task-040", commandRunnerForRepo(repo), legacyStoreForTest(t))
	if err == nil || !strings.Contains(err.Error(), "task not found") {
		t.Fatalf("expected task to be absent after delete, got %v", err)
	}
}
