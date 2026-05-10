package tasks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/project"
)

func TestStoreAddListGetDelete(t *testing.T) {
	projectStore, err := project.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(projectStore)
	repoPath := t.TempDir()

	task, err := store.Add(AddRequest{
		RepoPath:           repoPath,
		Title:              "Improve filters",
		Body:               "Add go_test preset",
		ModelLevel:         "very_high",
		AcceptanceCriteria: []string{"preset is available", "  "},
		VerificationPlan:   []string{"run targeted tests"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.ID == "" {
		t.Fatal("expected generated task id")
	}

	listed, err := store.List(ListRequest{RepoPath: repoPath, Status: "todo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed %d tasks, want 1", len(listed))
	}

	got, err := store.Get(GetRequest{RepoPath: repoPath, ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != task.Title {
		t.Fatalf("title = %q, want %q", got.Title, task.Title)
	}
	if got.ModelLevel != "very_high" {
		t.Fatalf("model_level = %q, want very_high", got.ModelLevel)
	}
	if len(got.AcceptanceCriteria) != 1 || got.AcceptanceCriteria[0] != "preset is available" {
		t.Fatalf("acceptance_criteria = %#v", got.AcceptanceCriteria)
	}
	if len(got.VerificationPlan) != 1 || got.VerificationPlan[0] != "run targeted tests" {
		t.Fatalf("verification_plan = %#v", got.VerificationPlan)
	}

	worktreeTask, err := store.Add(AddRequest{RepoPath: repoPath, ID: "task-123", Title: "Task worktree", TaskType: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if worktreeTask.Branch != "feature/task-123" || worktreeTask.WorktreePath != ".worktrees/task-123" {
		t.Fatalf("worktree fields = %#v", worktreeTask)
	}

	if err := store.Delete(DeleteRequest{RepoPath: repoPath, ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(DeleteRequest{RepoPath: repoPath, ID: worktreeTask.ID}); err != nil {
		t.Fatal(err)
	}
	listed, err = store.List(ListRequest{RepoPath: repoPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed %d tasks after delete, want 0", len(listed))
	}
}

func TestStoreUpdateSearchAndBatchUpsert(t *testing.T) {
	projectStore, err := project.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(projectStore)
	repoPath := t.TempDir()
	first, err := store.Add(AddRequest{
		RepoPath: repoPath,
		ID:       "workflow-dsl",
		Title:    "Workflow DSL",
		Body:     "Add batch task sync",
		Status:   "todo",
		Priority: "high",
		Tags:     []string{"MCP", "tasks", "tasks"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tags) != 2 {
		t.Fatalf("tags = %#v, want deduplicated tags", first.Tags)
	}
	updated, err := store.Update(UpdateRequest{
		RepoPath:           repoPath,
		ID:                 first.ID,
		Status:             "in_progress",
		Body:               "Expose task_batch_upsert",
		ModelLevel:         "high",
		AcceptanceCriteria: []string{"batch criteria preserved"},
		VerificationPlan:   []string{"targeted store tests"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CreatedAt != first.CreatedAt {
		t.Fatal("update must preserve created_at")
	}
	if updated.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", updated.Status)
	}
	if updated.ModelLevel != "high" {
		t.Fatalf("model_level = %q, want high", updated.ModelLevel)
	}
	if len(updated.AcceptanceCriteria) != 1 || updated.AcceptanceCriteria[0] != "batch criteria preserved" {
		t.Fatalf("acceptance_criteria = %#v", updated.AcceptanceCriteria)
	}
	if len(updated.VerificationPlan) != 1 || updated.VerificationPlan[0] != "targeted store tests" {
		t.Fatalf("verification_plan = %#v", updated.VerificationPlan)
	}
	matches, err := store.List(ListRequest{RepoPath: repoPath, Query: "targeted store"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != first.ID {
		t.Fatalf("matches = %#v", matches)
	}
	result, err := store.BatchUpsert(BatchUpsertRequest{
		RepoPath:      repoPath,
		CloseMissing:  true,
		MissingStatus: "done",
		Tasks: []AddRequest{{
			ID:     "new-task",
			Title:  "New task",
			Status: "todo",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Upserted) != 1 || result.Upserted[0].ID != "new-task" {
		t.Fatalf("upserted = %#v", result.Upserted)
	}
	if result.Upserted[0].ModelLevel != "" {
		t.Fatalf("default model_level = %q, want empty", result.Upserted[0].ModelLevel)
	}
	if len(result.Closed) != 1 || result.Closed[0].ID != first.ID || result.Closed[0].Status != "done" {
		t.Fatalf("closed = %#v", result.Closed)
	}
}

func TestStoreWritesRepoLocalLeanTasks(t *testing.T) {
	projectStore, err := project.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(projectStore)
	repoPath := t.TempDir()

	task, err := store.Add(AddRequest{
		RepoPath: repoPath,
		ID:       "lean-task",
		Title:    "Lean task",
		Body:     "Tracked in repo",
		Status:   "todo",
	})
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.taskPath(repoPath, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(repoPath, "tasks", "lean-task.lean")
	if path != wantPath {
		t.Fatalf("task path = %q, want %q", path, wantPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, leanTaskPrefix) {
		t.Fatalf("task file missing Lean metadata: %s", text)
	}
	if strings.HasSuffix(path, ".json") {
		t.Fatalf("task path must not use JSON extension: %q", path)
	}
}
