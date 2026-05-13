package mcp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func makeTask(id, status, title, parentID string, tags ...string) tasks.Task {
	return tasks.Task{
		ID: id, Status: status, Title: title, ParentID: parentID,
		Tags: tags, ProjectionSource: "lean_registry",
	}
}

// === Full graph ===

func TestBuildTaskGraph_Full(t *testing.T) {
	all := []tasks.Task{
		makeTask("task-1", "done", "Goal", "", "goal"),
		makeTask("task-2", "done", "Child B", "task-1"),
		makeTask("task-3", "in_progress", "Child A", "task-1"),
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

// === Focused graph ===

func TestBuildTaskGraph_FocusTask(t *testing.T) {
	all := []tasks.Task{
		makeTask("task-1", "done", "Epic", ""),
		makeTask("task-2", "done", "Child B", "task-1"),
		makeTask("task-3", "in_progress", "Child A", "task-1"),
		makeTask("task-4", "todo", "Grandchild", "task-2"),
		makeTask("task-5", "todo", "Unrelated", ""),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "task-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 4 {
		t.Fatalf("expected 4 nodes in focused graph, got %d", len(graph.Nodes))
	}
	// Focus task must be first
	if graph.Nodes[0].ID != "task-2" {
		t.Errorf("focus task should be first node")
	}
	for _, n := range graph.Nodes {
		if n.ID == "task-5" {
			t.Error("unrelated task should not be in focused graph")
		}
	}
}

func TestBuildTaskGraph_FocusTask_Root(t *testing.T) {
	// Focus on root task (no parent, no siblings)
	all := []tasks.Task{
		makeTask("root", "in_progress", "Root", ""),
		makeTask("child-1", "todo", "Child 1", "root"),
		makeTask("child-2", "todo", "Child 2", "root"),
		makeTask("unrelated", "todo", "Unrelated", ""),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "root"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Root + 2 children = 3 nodes
	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if graph.Nodes[0].ID != "root" {
		t.Error("focus task should be first node")
	}
	// No siblings when focus has no parent
	for _, n := range graph.Nodes {
		if n.ID == "unrelated" {
			t.Error("unrelated task should not be in focused graph")
		}
	}
}

func TestBuildTaskGraph_FocusTask_DeepAncestry(t *testing.T) {
	all := []tasks.Task{
		makeTask("grandparent", "done", "Grandparent", ""),
		makeTask("parent", "done", "Parent", "grandparent"),
		makeTask("focus", "in_progress", "Focus", "parent"),
		makeTask("sibling", "todo", "Sibling", "parent"),
		makeTask("child", "todo", "Child", "focus"),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "focus"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// grandparent, parent, focus, sibling, child = 5 nodes
	if len(graph.Nodes) != 5 {
		t.Fatalf("expected 5 nodes with deep ancestry, got %d", len(graph.Nodes))
	}
	// Verify ancestors are present
	found := map[string]bool{}
	for _, n := range graph.Nodes {
		found[n.ID] = true
	}
	for _, want := range []string{"grandparent", "parent", "focus", "sibling", "child"} {
		if !found[want] {
			t.Errorf("missing node: %s", want)
		}
	}
	// Edges: grandparent->parent, parent->focus, parent->sibling, focus->child
	if len(graph.Edges) != 4 {
		t.Fatalf("expected 4 edges, got %d", len(graph.Edges))
	}
}

// === Truncation & bounds ===

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

func TestBuildTaskGraph_FocusTask_MaxNodesPreservesFocus(t *testing.T) {
	// Focus task must survive even when max_nodes is tiny
	all := []tasks.Task{
		makeTask("parent", "done", "Parent", ""),
		makeTask("focus", "in_progress", "Focus", "parent"),
		makeTask("sibling", "todo", "Sibling", "parent"),
		makeTask("child", "todo", "Child", "focus"),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "focus", MaxNodes: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) != 1 {
		t.Fatalf("expected 1 node (focus), got %d", len(graph.Nodes))
	}
	if graph.Nodes[0].ID != "focus" {
		t.Errorf("focus task should be the sole node")
	}
	if graph.Truncated == nil {
		t.Fatal("expected truncation when max_nodes excludes relevant tasks")
	}
}

func TestBuildTaskGraph_MaxBytes(t *testing.T) {
	all := []tasks.Task{
		makeTask("task-1", "done", "A", ""),
		makeTask("task-2", "todo", "B", ""),
		makeTask("task-3", "todo", "C", ""),
		makeTask("task-4", "todo", "D", ""),
	}
	// Set max_bytes to a small value to force truncation
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", MaxNodes: 50, MaxBytes: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graph.Nodes) > 0 {
		// Should have been truncated to empty or near-empty due to 1 byte limit
		if graph.Truncated == nil {
			t.Error("expected truncation with tiny max_bytes")
		}
	}
}

func TestBuildTaskGraph_MaxBytes_PreservesFocus(t *testing.T) {
	all := []tasks.Task{
		makeTask("focus", "in_progress", "Focus", ""),
		makeTask("other", "todo", "Other", ""),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "focus", MaxBytes: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Focus task should survive even tiny max_bytes
	if len(graph.Nodes) == 0 {
		t.Fatal("focus task should survive max_bytes truncation")
	}
	if graph.Nodes[0].ID != "focus" {
		t.Errorf("expected focus task, got %s", graph.Nodes[0].ID)
	}
}

// === Empty & errors ===

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

func TestBuildTaskGraph_MissingFocus(t *testing.T) {
	_, err := BuildTaskGraph(nil, TaskGraphRequest{RepoPath: "/test", FocusTaskID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing focus task")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

// === Edges & provenance ===

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

func TestBuildTaskGraph_OrphanParent(t *testing.T) {
	// Task with parent_id pointing to non-existent task
	all := []tasks.Task{
		makeTask("orphan", "todo", "Orphan", "missing-parent"),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Node should exist
	if len(graph.Nodes) != 1 {
		t.Fatal("expected 1 node for orphan task")
	}
	// But no edge should be created
	if len(graph.Edges) != 0 {
		t.Errorf("expected 0 edges for orphan task, got %d", len(graph.Edges))
	}
}

func TestBuildTaskGraph_OnlyExplicitEdges(t *testing.T) {
	// No inferred edges should be present
	all := []tasks.Task{
		makeTask("a", "todo", "A", ""),
		makeTask("b", "todo", "B", ""),
		makeTask("c", "todo", "C", "a"),
	}
	graph, _ := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test"})
	for _, e := range graph.Edges {
		if e.Provenance == "inferred" {
			t.Errorf("inferred edge should not be present: %s -> %s", e.From, e.To)
		}
	}
}

// === Determinism ===

func TestBuildTaskGraph_Determinstic(t *testing.T) {
	all := []tasks.Task{
		makeTask("task-c", "todo", "C", ""),
		makeTask("task-a", "todo", "A", ""),
		makeTask("task-b", "todo", "B", "task-a"),
	}
	req := TaskGraphRequest{RepoPath: "/test"}
	graph1, _ := BuildTaskGraph(all, req)
	graph2, _ := BuildTaskGraph(all, req)
	if len(graph1.Nodes) != len(graph2.Nodes) {
		t.Fatal("node count differs between calls")
	}
	for i := range graph1.Nodes {
		if graph1.Nodes[i].ID != graph2.Nodes[i].ID {
			t.Errorf("node order differs at index %d: %s vs %s", i, graph1.Nodes[i].ID, graph2.Nodes[i].ID)
		}
	}
	for i := range graph1.Edges {
		if graph1.Edges[i].From != graph2.Edges[i].From || graph1.Edges[i].To != graph2.Edges[i].To {
			t.Errorf("edge order differs at index %d", i)
		}
	}
}

// === Node fields ===

func TestTaskGraphNode_HasRequiredFields(t *testing.T) {
	task := makeTask("task-1", "todo", "Test", "parent-1", "tag1")
	task.Priority = "high"
	task.ModelLevel = "medium"
	task.TaskType = "feature"
	node := taskToNode(task)
	if node.ID != "task-1" {
		t.Errorf("ID mismatch: %s", node.ID)
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
	if node.Priority != "high" {
		t.Errorf("Priority mismatch: %s", node.Priority)
	}
	if node.ModelLevel != "medium" {
		t.Errorf("ModelLevel mismatch")
	}
	if node.TaskType != "feature" {
		t.Errorf("TaskType mismatch: %s", node.TaskType)
	}
	if len(node.Tags) != 1 || node.Tags[0] != "tag1" {
		t.Errorf("Tags mismatch: %v", node.Tags)
	}
}

func TestTaskGraphNode_EmptyTags(t *testing.T) {
	task := makeTask("t-1", "todo", "T", "") // no tags variadic args
	node := taskToNode(task)
	if node.Tags != nil {
		t.Errorf("nil tags should stay nil")
	}
}

// === Validation ===

func TestValidateTaskGraphRequest(t *testing.T) {
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: ""}); err == nil {
		t.Error("expected error for empty repo_path")
	}
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: "/test", MaxNodes: -1}); err == nil {
		t.Error("expected error for negative max_nodes")
	}
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: "/test", MaxBytes: -1}); err == nil {
		t.Error("expected error for negative max_bytes")
	}
	// Valid requests should pass
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: "/test"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := validateTaskGraphRequest(TaskGraphRequest{RepoPath: "/test", MaxNodes: 10, MaxBytes: 1000}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// === OmittedEdges count ===

func TestBuildTaskGraph_OmittedEdges(t *testing.T) {
	all := []tasks.Task{
		makeTask("parent", "done", "P", ""),
		makeTask("child-1", "todo", "C1", "parent"),
		makeTask("child-2", "todo", "C2", "parent"),
		makeTask("child-3", "todo", "C3", "parent"),
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", MaxNodes: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.Truncated == nil {
		t.Fatal("expected truncation")
	}
	if graph.Truncated.OmittedEdges == 0 {
		t.Error("OmittedEdges should be > 0 when truncated nodes have edges")
	}
}

func TestBuildTaskGraph_NoOmittedEdgesWithoutParents(t *testing.T) {
	// All tasks without parents -- truncation should have OmittedEdges = 0
	all := make([]tasks.Task, 0, 10)
	for i := 0; i < 10; i++ {
		all = append(all, makeTask(fmt.Sprintf("task-%d", i), "todo", "T", ""))
	}
	graph, err := BuildTaskGraph(all, TaskGraphRequest{RepoPath: "/test", MaxNodes: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.Truncated == nil {
		t.Fatal("expected truncation")
	}
	if graph.Truncated.OmittedEdges != 0 {
		t.Errorf("OmittedEdges should be 0 when no tasks have parents, got %d", graph.Truncated.OmittedEdges)
	}
}
