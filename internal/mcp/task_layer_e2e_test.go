package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/lake"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestLeanBackedTaskLayerEndToEnd(t *testing.T) {
	repo := copyLeanRepoFixture(t)
	commands := commandRunnerForRepo(repo)
	store := legacyStoreForTest(t)
	ctx := context.Background()

	build, err := lake.Build(ctx, repo, lake.CommandRunner{Commands: commands, TimeoutSeconds: 20})
	if err != nil {
		t.Fatalf("lake build returned error: %v", err)
	}
	if !build.WorkspaceDetected || build.ExitCode != 0 {
		t.Fatalf("lake build failed: %+v", build)
	}

	current, source, err := readCurrentTasks(ctx, repo, commands, store)
	if err != nil {
		t.Fatalf("read current tasks: %v", err)
	}
	if source != "lean_registry" || !containsTaskWithSource(current, "task-006", "lean_registry") {
		t.Fatalf("current tasks did not come from Lean registry: source=%q tasks=%#v", source, current)
	}

	got, source, err := readTask(ctx, repo, "task-006", commands, store)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if source != "lean_registry" || got.ID != "task-006" || got.Body == "" {
		t.Fatalf("task_get projection invalid: source=%q task=%#v", source, got)
	}

	statusResult, err := setTaskStatus(ctx, tasks.StatusRequest{RepoPath: repo, ID: "task-006", Status: "blocked"}, commands, store)
	if err != nil {
		t.Fatalf("set status: %v", err)
	}
	if statusResult.Source != "lean_registry" || statusResult.Task.Status != "blocked" {
		t.Fatalf("set status did not use Lean registry: %+v", statusResult)
	}

	upsertResult, err := upsertTask(ctx, tasks.AddRequest{RepoPath: repo, ID: "task-997", Status: "todo", Title: "E2E generated task", Body: "created through Lean-backed task_upsert", Priority: "high", Tags: []string{"e2e", "lean"}}, commands, store)
	if err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	if upsertResult.Source != "lean_registry" || upsertResult.Task.ID != "task-997" {
		t.Fatalf("upsert did not use Lean registry: %+v", upsertResult)
	}

	batchResult, err := batchUpsertTasks(ctx, tasks.BatchUpsertRequest{RepoPath: repo, CloseMissing: false, Tasks: []tasks.AddRequest{{ID: "task-996", Status: "todo", Title: "E2E batch task", Priority: "medium"}}}, commands, store)
	if err != nil {
		t.Fatalf("batch upsert task: %v", err)
	}
	if batchResult.Source != "lean_registry" || len(batchResult.Upserted) != 1 || batchResult.Upserted[0].ID != "task-996" {
		t.Fatalf("batch upsert did not use Lean registry: %+v", batchResult)
	}

	legacyShadow, err := store.Add(tasks.AddRequest{RepoPath: repo, ID: "task-006", Status: "todo", Title: "Legacy shadow", Body: "must not be read"})
	if err != nil {
		t.Fatalf("write legacy shadow: %v", err)
	}
	if legacyShadow.Title != "Legacy shadow" {
		t.Fatalf("unexpected legacy setup: %+v", legacyShadow)
	}
	leanAgain, source, err := readTask(ctx, repo, "task-006", commands, store)
	if err != nil {
		t.Fatalf("read task after legacy shadow: %v", err)
	}
	if source != "lean_registry" || leanAgain.Title == "Legacy shadow" {
		t.Fatalf("silent legacy fallback occurred: source=%q task=%#v", source, leanAgain)
	}

	activePath := filepath.Join(repo, activeTasksLeanPath)
	data, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(activePath, append(data, []byte("\ndef duplicateTaskID : ArtifactId :=\n  { value := \"task-006\" }\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	_, source, err = readCurrentTasks(ctx, repo, commands, store)
	if err == nil {
		t.Fatal("expected invalid registry diagnostics")
	}
	if source != "lean_registry" || !strings.Contains(err.Error(), "Lean task exporter failed") {
		t.Fatalf("invalid registry did not fail closed: source=%q err=%v", source, err)
	}
}
