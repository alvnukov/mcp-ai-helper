package tasks

import (
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

	task, err := store.Add(AddRequest{RepoPath: repoPath, Title: "Improve filters", Body: "Add go_test preset"})
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

	if err := store.Delete(DeleteRequest{RepoPath: repoPath, ID: task.ID}); err != nil {
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
	updated, err := store.Update(UpdateRequest{RepoPath: repoPath, ID: first.ID, Status: "in_progress", Body: "Expose task_batch_upsert"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CreatedAt != first.CreatedAt {
		t.Fatal("update must preserve created_at")
	}
	if updated.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", updated.Status)
	}
	matches, err := store.List(ListRequest{RepoPath: repoPath, Query: "batch"})
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
	if len(result.Closed) != 1 || result.Closed[0].ID != first.ID || result.Closed[0].Status != "done" {
		t.Fatalf("closed = %#v", result.Closed)
	}
}
