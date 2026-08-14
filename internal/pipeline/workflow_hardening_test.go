package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
)

// Same-file serialization keys on the raw args.path string, so "same.txt"
// and "./same.txt" counted as two files: two locks, two hash-chain entries,
// two changed-file spellings, and a lost-update race between them.
func TestSameFileStepsNormalizePathSpellings(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "same.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	steps := []WorkflowStep{
		{ID: "edit-a", Tool: "guarded_replace", Args: map[string]any{"path": "same.txt", "old": "old", "new": "first"}},
		{ID: "edit-b", Tool: "guarded_replace", Args: map[string]any{"path": "./same.txt", "old": "first", "new": "second"}},
	}

	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{RepoPath: dir, Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q (%s)", result.Status, result.Reason)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "same.txt" {
		t.Fatalf("changed files = %v, want the single normalized path", result.ChangedFiles)
	}
	content, err := os.ReadFile(filepath.Join(dir, "same.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "second\n" {
		t.Fatalf("content = %q: same-file steps raced instead of serializing", content)
	}
}

// flakyTransitionBackend fails its Nth SetStatus so a multi-task transition
// dies partway and the rollback path becomes observable.
type flakyTransitionBackend struct {
	tasks     map[string]tasks.Task
	setCalls  int
	failOnSet int
}

func (b *flakyTransitionBackend) Get(_ context.Context, _ string, id string) (tasks.Task, error) {
	task, ok := b.tasks[id]
	if !ok {
		return tasks.Task{}, fmt.Errorf("task %q not found", id)
	}
	return task, nil
}

func (b *flakyTransitionBackend) List(_ context.Context, _ string) ([]tasks.Task, error) {
	items := make([]tasks.Task, 0, len(b.tasks))
	for _, task := range b.tasks {
		items = append(items, task)
	}
	return items, nil
}

func (b *flakyTransitionBackend) SetStatus(_ context.Context, req tasks.StatusRequest) (tasks.Task, error) {
	b.setCalls++
	if b.failOnSet > 0 && b.setCalls == b.failOnSet {
		return tasks.Task{}, errors.New("backend exploded")
	}
	task, ok := b.tasks[req.ID]
	if !ok {
		return tasks.Task{}, fmt.Errorf("task %q not found", req.ID)
	}
	task.Status = req.Status
	b.tasks[req.ID] = task
	return task, nil
}

func (b *flakyTransitionBackend) SetStatusWithResult(ctx context.Context, req tasks.StatusRequest) (TaskStatusMutation, error) {
	task, err := b.SetStatus(ctx, req)
	return TaskStatusMutation{Task: task}, err
}

func (b *flakyTransitionBackend) BatchUpsert(context.Context, tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error) {
	return tasks.BatchUpsertResult{}, errors.New("not implemented")
}

// A transition over several tasks applied SetStatus one by one with no
// rollback: a failure partway left the earlier tasks moved, and the retry
// wedged on the From guard.
func TestTaskTransitionRollsBackPartialFailure(t *testing.T) {
	dir := t.TempDir()
	backend := &flakyTransitionBackend{
		tasks: map[string]tasks.Task{
			"task-a": {ID: "task-a", Title: "A", Status: "todo"},
			"task-b": {ID: "task-b", Title: "B", Status: "todo"},
		},
		failOnSet: 2,
	}
	runner := NewRunnerWithTaskBackend(testConfig(dir), nil, backend)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{{
			ID:   "transition",
			Tool: "task_transition",
			Args: map[string]any{"task_ids": []any{"task-a", "task-b"}, "from": "todo", "to": "done"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status == "ok" {
		t.Fatal("transition should have failed on the second task")
	}
	if got := backend.tasks["task-a"].Status; got != "todo" {
		t.Fatalf("task-a status = %q, want rollback to todo", got)
	}
}
