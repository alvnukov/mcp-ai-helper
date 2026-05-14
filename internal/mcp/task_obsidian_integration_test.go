package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func setupObsidianTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	epic := `---
id: integ-epic
title: Integration Epic
status: blocked
priority: high
model_level: very_high
task_type: epic
tags:
  - integration
  - epic
acceptance_criteria:
  - Read works
  - Search works
verification_plan:
  - Get epic
  - Search
created_at: 2026-05-14T10:00:00Z
updated_at: 2026-05-14T10:00:00Z
---

## Body

Integration test epic body.

## Acceptance Criteria

- Read works
- Search works

## Verification Plan

1. Get epic
2. Search
`
	child := `---
id: integ-child
title: Integration Child
status: todo
priority: medium
model_level: medium
task_type: feature
parent_id: integ-epic
tags:
  - integration
  - child
acceptance_criteria:
  - parent_id preserved
verification_plan:
  - Check parent_id
created_at: 2026-05-14T10:05:00Z
updated_at: 2026-05-14T10:05:00Z
---

## Body

Child body with parent link.

## Acceptance Criteria

- parent_id preserved

## Verification Plan

1. Check parent_id
`
	os.WriteFile(filepath.Join(dir, "integ-epic.md"), []byte(epic), 0o644)
	os.WriteFile(filepath.Join(dir, "integ-child.md"), []byte(child), 0o644)
	return dir
}

func TestObsidianIntegrationServer(t *testing.T) {
	dir := setupObsidianTestDir(t)
	cfg := &config.Config{
		TaskRegistry: config.TaskRegistryConfig{
			Backend: "obsidian",
			Obsidian: config.ObsidianRegistryConfig{Path: dir},
		},
	}
	cfg.CommandPolicy.AllowedCWDs = []string{"."}
	cfg.CommandPolicy.DefaultTimeoutSeconds = 20

	_, _, _, _, backend := buildDeps(cfg)
	repoPath := "/test-repo"

	t.Run("ListAll", func(t *testing.T) {
		all, source, err := backend.ListAll(nil, repoPath)
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if source != "obsidian_registry" {
			t.Fatalf("source = %q, want obsidian_registry", source)
		}
		if len(all) != 2 {
			t.Fatalf("expected 2 tasks, got %d", len(all))
		}
	})

	t.Run("ListCurrent", func(t *testing.T) {
		active, _, err := backend.ListCurrent(nil, repoPath)
		if err != nil {
			t.Fatalf("ListCurrent: %v", err)
		}
		if len(active) != 1 {
			t.Fatalf("expected 1 active task, got %d", len(active))
		}
		if active[0].ID != "integ-child" {
			t.Fatalf("active task id = %q, want integ-child", active[0].ID)
		}
	})

	t.Run("GetEpic", func(t *testing.T) {
		task, source, err := backend.Get(nil, repoPath, "integ-epic")
		if err != nil {
			t.Fatalf("Get integ-epic: %v", err)
		}
		if source != "obsidian_registry" {
			t.Fatalf("source = %q", source)
		}
		if task.ID != "integ-epic" {
			t.Fatalf("id = %q", task.ID)
		}
		if task.Title != "Integration Epic" {
			t.Fatalf("title = %q", task.Title)
		}
		if task.Status != "blocked" {
			t.Fatalf("status = %q", task.Status)
		}
		if task.Priority != "high" {
			t.Fatalf("priority = %q", task.Priority)
		}
		if task.ModelLevel != "very_high" {
			t.Fatalf("model_level = %q", task.ModelLevel)
		}
		if task.TaskType != "epic" {
			t.Fatalf("task_type = %q", task.TaskType)
		}
		if len(task.Tags) != 2 {
			t.Fatalf("tags = %v", task.Tags)
		}
		if !strings.Contains(task.Body, "Integration test epic body") {
			t.Fatalf("body = %q", task.Body)
		}
		if len(task.AcceptanceCriteria) != 2 {
			t.Fatalf("acceptance_criteria = %v", task.AcceptanceCriteria)
		}
		if len(task.VerificationPlan) != 2 {
			t.Fatalf("verification_plan = %v", task.VerificationPlan)
		}
		if task.ProjectionSource != "obsidian_registry" {
			t.Fatalf("projection_source = %q", task.ProjectionSource)
		}
	})

	t.Run("GetChild", func(t *testing.T) {
		task, _, err := backend.Get(nil, repoPath, "integ-child")
		if err != nil {
			t.Fatalf("Get integ-child: %v", err)
		}
		if task.ParentID != "integ-epic" {
			t.Fatalf("parent_id = %q, want integ-epic", task.ParentID)
		}
		if task.Status != "todo" {
			t.Fatalf("status = %q", task.Status)
		}
	})

	t.Run("Search", func(t *testing.T) {
		all, _, err := backend.ListAll(nil, repoPath)
		if err != nil {
			t.Fatalf("ListAll for search: %v", err)
		}
		found := 0
		for _, t := range all {
			if strings.Contains(t.Body, "epic body") {
				found++
			}
		}
		if found != 1 {
			t.Fatalf("search for 'epic body' in body: got %d matches", found)
		}
	})

	t.Run("SetStatus", func(t *testing.T) {
		result, err := backend.SetStatus(nil, tasks.StatusRequest{
			RepoPath: repoPath, ID: "integ-child", Status: "in_progress",
		})
		if err != nil {
			t.Fatalf("SetStatus: %v", err)
		}
		if result.Task.Status != "in_progress" {
			t.Fatalf("status after SetStatus = %q", result.Task.Status)
		}
		if result.Source != "obsidian_registry" {
			t.Fatalf("source = %q", result.Source)
		}
		if !strings.Contains(result.Validation, "file written") {
			t.Fatalf("validation = %q", result.Validation)
		}
		got, _, err := backend.Get(nil, repoPath, "integ-child")
		if err != nil {
			t.Fatalf("Get after SetStatus: %v", err)
		}
		if got.Status != "in_progress" {
			t.Fatalf("persisted status = %q", got.Status)
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		result, err := backend.Upsert(nil, tasks.AddRequest{
			RepoPath: repoPath, ID: "integ-new", Status: "todo",
			Title: "New Task", Priority: "low", ModelLevel: "low",
			Body: "New task body.",
		})
		if err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		if result.Task.ID != "integ-new" {
			t.Fatalf("id = %q", result.Task.ID)
		}
		if result.Source != "obsidian_registry" {
			t.Fatalf("source = %q", result.Source)
		}
		all, _, err := backend.ListAll(nil, repoPath)
		if err != nil {
			t.Fatalf("ListAll after upsert: %v", err)
		}
		if len(all) != 3 {
			t.Fatalf("expected 3 tasks after upsert, got %d", len(all))
		}
	})

	t.Run("Delete", func(t *testing.T) {
		_, err := backend.Delete(nil, tasks.DeleteRequest{RepoPath: repoPath, ID: "integ-new"})
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "integ-new.md")); !os.IsNotExist(err) {
			t.Fatalf("file should be deleted")
		}
	})

	t.Run("BatchUpsert", func(t *testing.T) {
		result, err := backend.BatchUpsert(nil, tasks.BatchUpsertRequest{
			RepoPath: repoPath,
			Tasks: []tasks.AddRequest{
				{ID: "batch-1", Status: "todo", Title: "Batch One"},
				{ID: "batch-2", Status: "todo", Title: "Batch Two"},
			},
		})
		if err != nil {
			t.Fatalf("BatchUpsert: %v", err)
		}
		if len(result.Upserted) != 2 {
			t.Fatalf("expected 2 upserted, got %d", len(result.Upserted))
		}
		if result.Source != "obsidian_registry" {
			t.Fatalf("source = %q", result.Source)
		}
	})

	t.Run("ImportExport", func(t *testing.T) {
		dir2 := t.TempDir()
		targetBackend := newObsidianTaskBackend(dir2)
		res, err := exportTasks(context.Background(), backend, targetBackend, repoPath, ImportExportRequest{})
		if err != nil {
			t.Fatalf("exportTasks: %v", err)
		}
		if len(res.Added) < 1 {
			t.Fatalf("expected at least 1 exported, got %d added", len(res.Added))
		}
		exported, _, err := targetBackend.ListAll(nil, repoPath)
		if err != nil {
			t.Fatalf("ListAll on target: %v", err)
		}
		if len(exported) < 1 {
			t.Fatal("no tasks in target after export")
		}
	})

	t.Run("ImportExportDryRun", func(t *testing.T) {
		dir2 := t.TempDir()
		targetBackend := newObsidianTaskBackend(dir2)
		res, err := exportTasks(context.Background(), backend, targetBackend, repoPath, ImportExportRequest{DryRun: true})
		if err != nil {
			t.Fatalf("exportTasks dry-run: %v", err)
		}
		if !res.DryRun {
			t.Fatal("expected DryRun=true")
		}
		if len(res.Added) < 1 {
			t.Fatal("dry-run should report adds")
		}
		exported, _, _ := targetBackend.ListAll(nil, repoPath)
		if len(exported) != 0 {
			t.Fatalf("dry-run should not write files, got %d", len(exported))
		}
	})
}
