package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestObsidianParseValidParentEpic(t *testing.T) {
	input := `---
id: obsidian-task-registry-backend
title: Add configurable Lean/Obsidian task registry backend
status: blocked
priority: high
model_level: very_high
task_type: epic
parent_id: null
tags:
  - tasks
  - registry
acceptance_criteria:
  - Parent remains non-executable
  - User config exposes explicit backend selection
verification_plan:
  - Review child tasks for explicit Lean parity
created_at: 0001-01-01T00:00:00Z
updated_at: 0001-01-01T00:00:00Z
---

## Body

Parent/epic. Add a configurable task registry backend.

## Acceptance Criteria

- Parent remains non-executable
- User config exposes explicit backend selection

## Verification Plan

1. Review child tasks
2. Check config-selection task
`
	note, err := parseNote([]byte(input), "obsidian-task-registry-backend")
	if err != nil {
		t.Fatalf("parseNote: %v", err)
	}
	if note.ID != "obsidian-task-registry-backend" {
		t.Fatalf("id = %q", note.ID)
	}
	if note.Title != "Add configurable Lean/Obsidian task registry backend" {
		t.Fatalf("title = %q", note.Title)
	}
	if note.Status != "blocked" {
		t.Fatalf("status = %q", note.Status)
	}
	if note.Priority != "high" {
		t.Fatalf("priority = %q", note.Priority)
	}
	if note.ModelLevel != "very_high" {
		t.Fatalf("model_level = %q", note.ModelLevel)
	}
	if note.TaskType != "epic" {
		t.Fatalf("task_type = %q", note.TaskType)
	}
	if len(note.Tags) != 2 || note.Tags[0] != "tasks" || note.Tags[1] != "registry" {
		t.Fatalf("tags = %v", note.Tags)
	}
	if !strings.Contains(note.Body, "configurable task registry backend") {
		t.Fatalf("body = %q", note.Body)
	}
	task := noteToTask(note)
	if task.ProjectionSource != "obsidian_registry" {
		t.Fatalf("projection_source = %q", task.ProjectionSource)
	}
}

func TestObsidianParseChildWithParentID(t *testing.T) {
	input := `---
id: lean-registry-backend-adapter
title: Introduce TaskRegistryBackend abstraction
status: todo
parent_id: obsidian-task-registry-backend
priority: high
model_level: medium
task_type: implementation
---

## Body

Implement the minimal backend abstraction.
`
	note, err := parseNote([]byte(input), "lean-registry-backend-adapter")
	if err != nil {
		t.Fatalf("parseNote: %v", err)
	}
	if note.ParentID != "obsidian-task-registry-backend" {
		t.Fatalf("parent_id = %q", note.ParentID)
	}
	task := noteToTask(note)
	if task.ParentID != "obsidian-task-registry-backend" {
		t.Fatalf("parent_id = %q", task.ParentID)
	}
}

func TestObsidianRoundTrip(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	result, err := backend.Upsert(nil, tasks.AddRequest{
		ID: "test-task", Status: "todo", Title: "Round Trip Test",
		Priority: "medium", ModelLevel: "low",
		Body: "This is the body.",
		AcceptanceCriteria: []string{"Must round-trip without loss"},
		VerificationPlan:   []string{"Run test", "Check output"},
		Tags: []string{"test", "roundtrip"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if result.Task.ID != "test-task" {
		t.Fatalf("id = %q", result.Task.ID)
	}
	got, _, err := backend.Get(nil, "", "test-task")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Round Trip Test" {
		t.Fatalf("title = %q", got.Title)
	}
	if got.Status != "todo" {
		t.Fatalf("status = %q", got.Status)
	}
	if got.Priority != "medium" {
		t.Fatalf("priority = %q", got.Priority)
	}
	if got.ModelLevel != "low" {
		t.Fatalf("model_level = %q", got.ModelLevel)
	}
	if got.Body != "This is the body." {
		t.Fatalf("body = %q", got.Body)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "test" {
		t.Fatalf("tags = %v", got.Tags)
	}
	if len(got.AcceptanceCriteria) != 1 {
		t.Fatalf("acceptance_criteria = %v", got.AcceptanceCriteria)
	}
	if len(got.VerificationPlan) != 2 {
		t.Fatalf("verification_plan = %v", got.VerificationPlan)
	}
	if got.ProjectionSource != "obsidian_registry" {
		t.Fatalf("projection_source = %q", got.ProjectionSource)
	}
}

func TestObsidianInvalidFrontmatter(t *testing.T) {
	input := `not yaml
---
id: test
---
`
	_, err := parseNote([]byte(input), "test")
	if err == nil || !strings.Contains(err.Error(), "missing opening ---") {
		t.Fatalf("expected missing opening --- error, got: %v", err)
	}
}

func TestObsidianMissingRequiredFields(t *testing.T) {
	input := `---
id: test
---
`
	_, err := parseNote([]byte(input), "test")
	if err == nil || !strings.Contains(err.Error(), "'title' is required") {
		t.Fatalf("expected missing title error, got: %v", err)
	}
}

func TestObsidianDelete(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.Upsert(nil, tasks.AddRequest{
		ID: "delete-me", Status: "todo", Title: "To Delete",
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	_, err = backend.Delete(nil, tasks.DeleteRequest{ID: "delete-me"})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "delete-me.md")); !os.IsNotExist(err) {
		t.Fatalf("file should not exist after delete")
	}
}

func TestObsidianSetStatus(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.Upsert(nil, tasks.AddRequest{
		ID: "status-test", Status: "todo", Title: "Status Test",
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	_, err = backend.SetStatus(nil, tasks.StatusRequest{ID: "status-test", Status: "done"})
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	task, _, err := backend.Get(nil, "", "status-test")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Status != "done" {
		t.Fatalf("status = %q", task.Status)
	}
}

func TestObsidianBatchUpsert(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.BatchUpsert(nil, tasks.BatchUpsertRequest{
		Tasks: []tasks.AddRequest{
			{ID: "batch-1", Status: "todo", Title: "Batch 1"},
			{ID: "batch-2", Status: "todo", Title: "Batch 2"},
		},
	})
	if err != nil {
		t.Fatalf("BatchUpsert: %v", err)
	}
	all, _, err := backend.ListAll(nil, "")
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(all))
	}
}

func TestObsidianListCurrent(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	backend.Upsert(nil, tasks.AddRequest{ID: "active-1", Status: "todo", Title: "Active 1"})
	backend.Upsert(nil, tasks.AddRequest{ID: "active-2", Status: "in_progress", Title: "Active 2"})
	backend.Upsert(nil, tasks.AddRequest{ID: "done-1", Status: "done", Title: "Done 1"})
	active, _, err := backend.ListCurrent(nil, "")
	if err != nil {
		t.Fatalf("ListCurrent: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active tasks, got %d", len(active))
	}
}
