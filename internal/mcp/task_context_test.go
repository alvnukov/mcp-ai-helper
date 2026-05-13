package mcp

import (
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func makeCTask(id, status, title, parentID string, tags ...string) tasks.Task {
	return tasks.Task{
		ID: id, Status: status, Title: title, ParentID: parentID,
		Tags: tags, ProjectionSource: "lean_registry",
	}
}

// === Success scenarios ===

func TestBuildTaskContext_ExecutableChild(t *testing.T) {
	// Executable child task under a done parent
	all := []tasks.Task{
		makeCTask("goal", "done", "Project Goal", "", "goal"),
		makeCTask("epic", "done", "Graph Feature", "goal"),
		makeCTask("task-1", "done", "Design schema", "epic"),
		makeCTask("task-2", "in_progress", "Implement builder", "epic"),
		makeCTask("task-3", "todo", "Add MCP tool", "epic"),
	}
	ctx, err := BuildTaskContext(all, TaskContextRequest{RepoPath: "/test", TaskID: "task-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Selected task
	if ctx.Task.ID != "task-2" {
		t.Errorf("expected task-2, got %s", ctx.Task.ID)
	}
	// Goal chain: goal -> epic (parent)
	if len(ctx.GoalChain) != 2 {
		t.Fatalf("expected 2 items in goal chain, got %d", len(ctx.GoalChain))
	}
	if ctx.GoalChain[0].ID != "goal" {
		t.Errorf("goal chain first should be goal, got %s", ctx.GoalChain[0].ID)
	}
	if ctx.GoalChain[1].ID != "epic" {
		t.Errorf("goal chain second should be epic, got %s", ctx.GoalChain[1].ID)
	}
	// Already done: task-1 is done sibling
	if len(ctx.AlreadyDone) != 1 {
		t.Fatalf("expected 1 done, got %d", len(ctx.AlreadyDone))
	}
	if ctx.AlreadyDone[0].ID != "task-1" {
		t.Errorf("expected task-1 done, got %s", ctx.AlreadyDone[0].ID)
	}
	// Planned next: task-3 is todo sibling
	if len(ctx.PlannedNext) != 1 {
		t.Fatalf("expected 1 planned next, got %d", len(ctx.PlannedNext))
	}
	if ctx.PlannedNext[0].ID != "task-3" {
		t.Errorf("expected task-3 planned, got %s", ctx.PlannedNext[0].ID)
	}
	// Usage contract
	if ctx.UsageContract.IntendedUse == "" {
		t.Error("usage_contract.intended_use should not be empty")
	}
	if ctx.UsageContract.MustNot == "" {
		t.Error("usage_contract.must_not should not be empty")
	}
	if ctx.UsageContract.IfTruncated == "" {
		t.Error("usage_contract.if_truncated should not be empty")
	}
}

func TestBuildTaskContext_BlockedParent(t *testing.T) {
	all := []tasks.Task{
		makeCTask("goal", "in_progress", "Goal", "", "goal"),
		makeCTask("epic", "blocked", "Blocked Epic", "goal"),
		makeCTask("task-1", "todo", "Child task", "epic"),
	}
	ctx, err := BuildTaskContext(all, TaskContextRequest{RepoPath: "/test", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Blockers: parent epic is blocked
	if len(ctx.Blockers) == 0 {
		t.Error("expected blockers when parent is blocked")
	}
	// Warnings: parent blocked
	foundW := false
	for _, w := range ctx.Warnings {
		if strings.Contains(w, "blocked") {
			foundW = true
		}
	}
	if !foundW {
		t.Error("expected warning about blocked parent")
	}
}

func TestBuildTaskContext_GoalItself(t *testing.T) {
	// Selected task is the goal itself
	all := []tasks.Task{
		makeCTask("goal", "in_progress", "Sole Goal", "", "goal"),
	}
	ctx, err := BuildTaskContext(all, TaskContextRequest{RepoPath: "/test", TaskID: "goal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.GoalChain) != 1 {
		t.Fatalf("goal chain should be just the goal, got %d", len(ctx.GoalChain))
	}
	if ctx.GoalChain[0].ID != "goal" {
		t.Errorf("expected goal, got %s", ctx.GoalChain[0].ID)
	}
}

func TestBuildTaskContext_TaskWithOutOfScope(t *testing.T) {
	// Task with body containing "Out of scope:" section
	all := []tasks.Task{
		makeCTask("task-1", "todo", "Implement X", ""),
	}
	all[0].Body = "Some body.\nOut of scope:\n- No UI\n- No tests"
	ctx, err := BuildTaskContext(all, TaskContextRequest{RepoPath: "/test", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.NonGoals) < 2 {
		t.Errorf("expected at least 2 non-goals from body")
	}
}

// === Errors ===

func TestBuildTaskContext_MissingTask(t *testing.T) {
	_, err := BuildTaskContext(nil, TaskContextRequest{RepoPath: "/test", TaskID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestBuildTaskContext_EmptyTaskID(t *testing.T) {
	_, err := BuildTaskContext(nil, TaskContextRequest{RepoPath: "/test", TaskID: ""})
	if err == nil {
		t.Fatal("expected error for empty task_id")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention 'required', got: %v", err)
	}
}

// === Truncation ===

func TestBuildTaskContext_MaxNodesTruncation(t *testing.T) {
	all := []tasks.Task{
		makeCTask("goal", "in_progress", "Goal", "", "goal"),
		makeCTask("epic", "done", "Epic", "goal"),
		makeCTask("task-1", "done", "T1", "epic"),
		makeCTask("task-2", "done", "T2", "epic"),
		makeCTask("task-3", "done", "T3", "epic"),
		makeCTask("task-4", "in_progress", "Focus", "epic"),
	}
	ctx, err := BuildTaskContext(all, TaskContextRequest{RepoPath: "/test", TaskID: "task-4", MaxNodes: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Truncated == nil {
		t.Fatal("expected truncation with tiny max_nodes")
	}
}

// === Validation ===

func TestValidateTaskContextRequest(t *testing.T) {
	if err := validateTaskContextRequest(TaskContextRequest{RepoPath: "", TaskID: "t-1"}); err == nil {
		t.Error("expected error for empty repo_path")
	}
	if err := validateTaskContextRequest(TaskContextRequest{RepoPath: "/t", TaskID: ""}); err == nil {
		t.Error("expected error for empty task_id")
	}
	if err := validateTaskContextRequest(TaskContextRequest{RepoPath: "/t", TaskID: "t-1", MaxNodes: -1}); err == nil {
		t.Error("expected error for negative max_nodes")
	}
	if err := validateTaskContextRequest(TaskContextRequest{RepoPath: "/t", TaskID: "t-1"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
