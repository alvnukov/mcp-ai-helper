package mcp

import (
	"fmt"
    
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks")

func makeTask(id, status, title, parentID string, tags ...string) tasks.Task {
	return tasks.Task{
		ID: id, Status: status, Title: title, ParentID: parentID,
		Tags: tags, ProjectionSource: "lean_registry",
	}
}

func TestBuildTaskGraph_Full(t *testing.T) {
	all := []tasks.Task{
		makeTask("task-1", "done", "Goal", "", "goal"),
		makeTask("task-2", "done", "Child A", "task-1"),
		makeTask("task-3", "in_progress", "Child B", "task-1"),
		makeTask("task-4", "todo", "Grandchild", "task-2"),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(graph.Edges))
	}
	if graph.Provenance.Source != "lean_registry" {
		t.Errorf("expected lean_registry source, got %s", graph.Provenance.Source)
	}
	if graph.Truncated != nil {
		t.Error("unexpected truncation")
	}
}

func TestBuildTaskGraph_FocusTask(t *testing.T) {
	all := []tasks.Task{
		makeTask("task-1", "done", "Epic", ""),
		makeTask("task-2", "done", "Child A", "task-1"),
		makeTask("task-3", "in_progress", "Child B", "task-1"),
		makeTask("task-4", "todo", "Grandchild", "task-2"),
		makeTask("task-5", "todo", "Unrelated", ""),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "task-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should include: task-1 (parent), task-2 (focus), task-3 (sibling), task-4 (child)
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 nodes in focused graph, got %d", len(graph.Nodes))
	}
	// Should NOT include task-5 (unrelated)
	for _, n := range graph.Nodes {
		if n.ID == "task-5" {
			t.Error("unrelated task should not be in focused graph")
		}
	}
}

func TestBuildTaskGraph_MissingFocus(t *testing.T) {
	_, err := BuildTaskGraph(nil, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing focus task")
	}
}

func TestBuildTaskGraph_MaxNodes(t *testing.T) {
	all := make([]tasks.Task, 0, 10)
	for i := 0; i < 10; i++ {
		all = append(all, makeTask(fmt.Sprintf("task-%d", i), "todo", "T", ""))
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", MaxNodes: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if graph.Truncated == nil {
		t.Fatal("expected truncation metadata")
	}
	if graph.Truncated.OmittedNodes != 7 {
		t.Errorf("expected 7 omitted, got %d", graph.Truncated.OmittedNodes)
	}
}

func TestBuildTaskGraph_EmptyTasks(t *testing.T) {
	graph, err := BuildTaskGraph(nil, TaskGraphRequest{RepoPath: "/test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(graph.Nodes))
	}
	if graph.Truncated != nil {
		t.Error("no truncation for empty graph")
	}
}

func TestTaskGraphNode_HasRequiredFields(t *testing.T) {
	task := makeTask("task-1", "todo", "Test", "parent-1", "tag1")
	node := taskToNode(task)
	if node.ID != "task-1" {
		t.Errorf("ID mismatch")
	}
	if node.Status != "todo" {
		t.Errorf("Status mismatch")
	}
	if node.Title != "Test" {
		t.Errorf("Title mismatch")
	}
	if node.ParentID != "parent-1" {
		t.Errorf("ParentID mismatch")
	}
	if len(node.Tags) != 1 || node.Tags[0] != "tag1" {
		t.Errorf("Tags mismatch")
	}
}

func TestBuildTaskGraph_Provenance(t *testing.T) {
	all := []tasks.Task{
		makeTask("parent", "done", "P", ""),
		makeTask("child", "todo", "C", "parent"),
	}
	graph, _ := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test"})
	if len(graph.Edges) != 1 {
		t.Fatal("expected 1 edge")
	}
	if graph.Edges[0].Provenance != "explicit" {
		t.Errorf("expected explicit provenance, got %s", graph.Edges[0].Provenance)
	}
	if graph.Edges[0].Kind != "parent_child" {
		t.Errorf("expected parent_child kind, got %s", graph.Edges[0].Kind)
	}
}

func TestValidateTaskGraphRequest(t *testing.T) {
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: ""}); err == nil {
		t.Error("expected error for empty repo_path")
	}
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: "/test", MaxNodes: -1}); err == nil {
		t.Error("expected error for negative max_nodes")
	}
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: "/test", MaxNodes: 10, MaxBytes: 1000}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
