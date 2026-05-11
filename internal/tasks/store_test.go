package tasks

import (
	"errors"
	"strings"
	"testing"
)

func TestLegacyTaskStoreFailsClosed(t *testing.T) {
	store := NewStore(nil)
	if _, err := store.Add(AddRequest{RepoPath: t.TempDir(), ID: "task-1", Title: "legacy"}); !errors.Is(err, ErrLegacyTaskStoreDisabled) {
		t.Fatalf("Add err = %v, want ErrLegacyTaskStoreDisabled", err)
	}
	if _, err := store.List(ListRequest{RepoPath: t.TempDir()}); !errors.Is(err, ErrLegacyTaskStoreDisabled) {
		t.Fatalf("List err = %v, want ErrLegacyTaskStoreDisabled", err)
	}
	if err := store.Delete(DeleteRequest{RepoPath: t.TempDir(), ID: "task-1"}); !errors.Is(err, ErrLegacyTaskStoreDisabled) {
		t.Fatalf("Delete err = %v, want ErrLegacyTaskStoreDisabled", err)
	}
}

func TestWorktreeContextNormalization(t *testing.T) {
	task := Task{ID: "Task 123", TaskType: "Feature"}
	if err := NormalizeWorktreeFields(&task); err != nil {
		t.Fatal(err)
	}
	if task.ID != "task-123" || task.TaskType != "feature" || task.Branch != "feature/task-123" || task.WorktreePath != ".worktrees/task-123" {
		t.Fatalf("normalized task = %#v", task)
	}

	fromBranch := Task{ID: "task-456", Branch: "bug/task-456"}
	if err := NormalizeWorktreeFields(&fromBranch); err != nil {
		t.Fatal(err)
	}
	if fromBranch.TaskType != "bug" {
		t.Fatalf("task_type = %q, want bug", fromBranch.TaskType)
	}
}

func TestWorktreeContextRejectsInconsistentBranch(t *testing.T) {
	task := Task{ID: "task-123", TaskType: "feature", Branch: "bug/task-123"}
	if err := NormalizeWorktreeFields(&task); err == nil || !strings.Contains(err.Error(), "branch must be feature/task-123") {
		t.Fatalf("expected branch invariant error, got %v", err)
	}
}

func TestNormalizeModelLevel(t *testing.T) {
	got, err := NormalizeModelLevel("very-high")
	if err != nil {
		t.Fatal(err)
	}
	if got != "very_high" {
		t.Fatalf("model level = %q", got)
	}
	if _, err := NormalizeModelLevel("ultra"); err == nil {
		t.Fatal("expected unsupported model level")
	}
}
