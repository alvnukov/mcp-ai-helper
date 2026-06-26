package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestObsidianParsesClosingDotsAndNormalizesFields(t *testing.T) {
	input := `---
id: normalize-task
title: Normalize Task
status: In-Progress
priority: HIGH
model_level: Very High
tags:
  - Tasks
  -  Backend 
...

## Body

Normalized.
`
	note, err := parseNote([]byte(input), "normalize-task")
	if err != nil {
		t.Fatalf("parseNote: %v", err)
	}
	if note.Status != "in_progress" || note.Priority != "high" || note.ModelLevel != "very_high" {
		t.Fatalf("normalized fields = status:%q priority:%q model:%q", note.Status, note.Priority, note.ModelLevel)
	}
	if len(note.Tags) != 2 || note.Tags[0] != "tasks" || note.Tags[1] != "backend" {
		t.Fatalf("tags = %#v", note.Tags)
	}
}

func TestObsidianRejectsInvalidStatus(t *testing.T) {
	input := `---
id: bad-status
title: Bad Status
status: waiting
---
`
	_, err := parseNote([]byte(input), "bad-status")
	if err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("expected invalid status error, got: %v", err)
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
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".worktrees", "test-task"), 0o700); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	createdAt := time.Date(2026, 5, 3, 10, 0, 0, 1, time.UTC)
	updatedAt := time.Date(2026, 5, 4, 11, 0, 0, 2, time.UTC)
	result, err := backend.Upsert(nil, tasks.AddRequest{
		RepoPath: repo,
		ID:       "test-task", Status: "todo", Title: "Round Trip Test",
		Priority: "medium", ModelLevel: "low",
		TaskType: "feature", Branch: "feature/test-task", WorktreePath: ".worktrees/test-task",
		Body:               "This is the body.",
		AcceptanceCriteria: []string{"Must round-trip without loss"},
		VerificationPlan:   []string{"Run test", "Check output"},
		Tags:               []string{"test", "roundtrip"},
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if result.Task.ID != "test-task" {
		t.Fatalf("id = %q", result.Task.ID)
	}
	got, _, err := backend.Get(nil, repo, "test-task")
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
	if got.TaskType != "feature" || got.Branch != "feature/test-task" || got.WorktreePath != ".worktrees/test-task" {
		t.Fatalf("worktree fields = task_type:%q branch:%q worktree:%q", got.TaskType, got.Branch, got.WorktreePath)
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
	if !got.CreatedAt.Equal(createdAt) || !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("timestamps = created:%s updated:%s", got.CreatedAt, got.UpdatedAt)
	}
	if !got.WorktreeExists || !strings.HasSuffix(got.CodePath, filepath.Join(".worktrees", "test-task")) {
		t.Fatalf("worktree context = exists:%v code_path:%q", got.WorktreeExists, got.CodePath)
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

func TestObsidianParsesPlainScalarTitleWithColon(t *testing.T) {
	input := `---
id: task-001
title: task_current fails after successful helper rebuild: unknown executable task_registry_export
status: done
---

## Body

Human-authored task note.
`
	note, err := parseNote([]byte(input), "task-001")
	if err != nil {
		t.Fatalf("parseNote: %v", err)
	}
	want := "task_current fails after successful helper rebuild: unknown executable task_registry_export"
	if note.Title != want {
		t.Fatalf("title = %q", note.Title)
	}
}

func TestObsidianParsesFrontmatterListItemsWithColon(t *testing.T) {
	input := `---
id: colon-list
title: Colon List
status: done
acceptance_criteria:
  - Contract includes test-first examples: valid fixtures, invalid fixtures and round-trip expectations.
verification_plan:
  - Check examples for deterministic parse/write behavior, config routing clarity and no silent data loss.
---

## Body

Done task note.
`
	note, err := parseNote([]byte(input), "colon-list")
	if err != nil {
		t.Fatalf("parseNote: %v", err)
	}
	want := "Contract includes test-first examples: valid fixtures, invalid fixtures and round-trip expectations."
	if len(note.AcceptanceCriteria) != 1 || note.AcceptanceCriteria[0] != want {
		t.Fatalf("acceptance_criteria = %#v", note.AcceptanceCriteria)
	}
}

func TestObsidianParsesFrontmatterListItemsStartingWithBacktick(t *testing.T) {
	input := "---\nid: task-118\ntitle: Read Files\nstatus: done\nacceptance_criteria:\n  - `read_files` is registered with an accurate MCP input schema: required `repo_path` and required string-array `paths`.\n---\n"
	note, err := parseNote([]byte(input), "task-118")
	if err != nil {
		t.Fatalf("parseNote: %v", err)
	}
	if len(note.AcceptanceCriteria) != 1 || !strings.Contains(note.AcceptanceCriteria[0], "read_files") {
		t.Fatalf("acceptance_criteria = %#v", note.AcceptanceCriteria)
	}
}

func TestObsidianWriterQuotesColonScalars(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.Upsert(nil, tasks.AddRequest{
		ID: "colon-title", Status: "todo", Title: "Config: backend selection",
		AcceptanceCriteria: []string{"Examples: lean and obsidian"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, _, err := backend.Get(nil, "", "colon-title")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Config: backend selection" {
		t.Fatalf("title = %q", got.Title)
	}
	if len(got.AcceptanceCriteria) != 1 || got.AcceptanceCriteria[0] != "Examples: lean and obsidian" {
		t.Fatalf("acceptance_criteria = %#v", got.AcceptanceCriteria)
	}
}

func TestObsidianListAllAutoInitializesMissingRegistryDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing-registry")
	backend := newObsidianTaskBackend(dir)
	all, _, err := backend.ListAll(nil, "")
	if err != nil {
		t.Fatalf("ListAll should auto-initialize missing registry dir: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(all))
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("registry dir was not created: info=%v err=%v", info, err)
	}
	meta := backend.(*obsidianTaskBackend).ListMetadata()
	if !strings.Contains(meta.Validation, "0 task") {
		t.Fatalf("validation = %q", meta.Validation)
	}
}

func TestObsidianListAllDegradesOnInvalidNote(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.Upsert(nil, tasks.AddRequest{ID: "valid", Status: "todo", Title: "Valid"})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	bad := []byte("---\nid: invalid\nstatus: todo\n---\n")
	if err := os.WriteFile(filepath.Join(dir, "invalid.md"), bad, 0o644); err != nil {
		t.Fatal(err)
	}
	all, _, err := backend.ListAll(nil, "")
	if err != nil {
		t.Fatalf("ListAll should degrade instead of failing: %v", err)
	}
	if len(all) != 1 || all[0].ID != "valid" {
		t.Fatalf("ListAll tasks = %#v, want only valid task", all)
	}
	meta := backend.(*obsidianTaskBackend).ListMetadata()
	if len(meta.Diagnostics) != 1 || meta.Diagnostics[0].Code != "invalid_projection" {
		t.Fatalf("diagnostics = %#v", meta.Diagnostics)
	}
	if !strings.Contains(meta.Validation, "1 diagnostic") {
		t.Fatalf("validation = %q", meta.Validation)
	}
	if _, err := os.Stat(filepath.Join(dir, "invalid.md")); err != nil {
		t.Fatalf("invalid note must be preserved: %v", err)
	}
}

func TestObsidianListAllAutoHealsFilenameIDMismatch(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	mismatched := []byte("---\nid: canonical-task\ntitle: Canonical Task\nstatus: todo\n---\n\n## Body\n\nOriginal body.\n")
	if err := os.WriteFile(filepath.Join(dir, "wrong-name.md"), mismatched, 0o644); err != nil {
		t.Fatal(err)
	}
	all, _, err := backend.ListAll(nil, "")
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 || all[0].ID != "canonical-task" || all[0].Body != "Original body." {
		t.Fatalf("tasks = %#v", all)
	}
	if _, err := os.Stat(filepath.Join(dir, "wrong-name.md")); !os.IsNotExist(err) {
		t.Fatalf("wrong-name.md should be renamed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "canonical-task.md")); err != nil {
		t.Fatalf("canonical-task.md missing after auto-heal: %v", err)
	}
	meta := backend.(*obsidianTaskBackend).ListMetadata()
	if len(meta.Diagnostics) != 1 || meta.Diagnostics[0].Code != "projection_id_auto_healed" {
		t.Fatalf("diagnostics = %#v", meta.Diagnostics)
	}
	if !stringSliceContains(meta.ChangedFiles, "wrong-name.md") || !stringSliceContains(meta.ChangedFiles, "canonical-task.md") {
		t.Fatalf("changed files = %#v", meta.ChangedFiles)
	}
	got, _, err := backend.Get(nil, "", "canonical-task")
	if err != nil {
		t.Fatalf("Get canonical-task after auto-heal: %v", err)
	}
	if got.Title != "Canonical Task" {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestObsidianListAllDoesNotOverwriteConflictingMismatch(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.Upsert(nil, tasks.AddRequest{ID: "canonical-task", Status: "todo", Title: "Existing Canonical"})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	mismatched := []byte("---\nid: canonical-task\ntitle: Conflicting Task\nstatus: todo\n---\n")
	if err := os.WriteFile(filepath.Join(dir, "wrong-name.md"), mismatched, 0o644); err != nil {
		t.Fatal(err)
	}
	all, _, err := backend.ListAll(nil, "")
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 || all[0].Title != "Existing Canonical" {
		t.Fatalf("tasks = %#v", all)
	}
	if _, err := os.Stat(filepath.Join(dir, "wrong-name.md")); err != nil {
		t.Fatalf("conflicting note must be preserved: %v", err)
	}
	meta := backend.(*obsidianTaskBackend).ListMetadata()
	if len(meta.Diagnostics) != 1 || meta.Diagnostics[0].Code != "projection_id_conflict" {
		t.Fatalf("diagnostics = %#v", meta.Diagnostics)
	}
	if len(meta.ChangedFiles) != 0 {
		t.Fatalf("conflict must not report changed files: %#v", meta.ChangedFiles)
	}
}

func stringSliceContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
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
	result, err := backend.BatchUpsert(nil, tasks.BatchUpsertRequest{
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
	if !stringSliceContains(result.ChangedFiles, "batch-1.md") || !stringSliceContains(result.ChangedFiles, "batch-2.md") {
		t.Fatalf("changed files = %#v", result.ChangedFiles)
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

func TestObsidianRoundTripAllFields(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	original := tasks.AddRequest{
		ID: "full-task", Status: "in_progress", Title: "Full Field Task",
		Priority: "critical", ModelLevel: "high", TaskType: "feature",
		ParentID: "parent-epic",
		Tags:     []string{"critical", "security", "backend"},
		Branch:   "feature/full-task", WorktreePath: ".worktrees/full-task",
		AcceptanceCriteria: []string{"All fields survive round-trip", "No silent drops"},
		VerificationPlan:   []string{"Write", "Read back", "Compare"},
		Body:               "This task has every supported field populated.\nSecond paragraph.",
	}
	_, err := backend.Upsert(nil, original)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, _, err := backend.Get(nil, "", "full-task")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"id", got.ID, "full-task"},
		{"title", got.Title, "Full Field Task"},
		{"status", got.Status, "in_progress"},
		{"priority", got.Priority, "critical"},
		{"model_level", got.ModelLevel, "high"},
		{"task_type", got.TaskType, "feature"},
		{"parent_id", got.ParentID, "parent-epic"},
		{"branch", got.Branch, "feature/full-task"},
		{"worktree_path", got.WorktreePath, ".worktrees/full-task"},
		{"projection_source", got.ProjectionSource, "obsidian_registry"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.field, c.got, c.want)
		}
	}
	if len(got.Tags) != 3 {
		t.Errorf("tags: got %d, want 3: %v", len(got.Tags), got.Tags)
	}
	if len(got.AcceptanceCriteria) != 2 {
		t.Errorf("acceptance_criteria: got %d, want 2: %v", len(got.AcceptanceCriteria), got.AcceptanceCriteria)
	}
	if len(got.VerificationPlan) != 3 {
		t.Errorf("verification_plan: got %d, want 3: %v", len(got.VerificationPlan), got.VerificationPlan)
	}
	if got.Body != original.Body {
		t.Errorf("body: got %q, want %q", got.Body, original.Body)
	}
}

func TestObsidianMissingRequiredFieldWouldFail(t *testing.T) {
	fixture := `---
id: no-title-task
status: todo
---

## Body

No title here.
`
	_, err := parseNote([]byte(fixture), "no-title-task")
	if err == nil {
		t.Fatal("expected missing title to fail, got nil")
	}
	if !strings.Contains(err.Error(), "'title' is required") {
		t.Fatalf("expected title required error, got: %v", err)
	}
}

func TestObsidianLeanSpecificFieldsNotDropped(t *testing.T) {
	dir := t.TempDir()
	backend := newObsidianTaskBackend(dir)
	_, err := backend.Upsert(nil, tasks.AddRequest{
		ID: "lean-fields", Status: "todo", Title: "Lean Fields Test",
		Branch:             "feature/lean-fields",
		WorktreePath:       ".worktrees/lean-fields",
		AcceptanceCriteria: []string{"Branch must survive"},
		VerificationPlan:   []string{"Check branch", "Check worktree"},
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, _, err := backend.Get(nil, "", "lean-fields")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Branch != "feature/lean-fields" {
		t.Errorf("Lean field 'branch' was silently dropped: got %q", got.Branch)
	}
	if got.WorktreePath != ".worktrees/lean-fields" {
		t.Errorf("Lean field 'worktree_path' was silently dropped: got %q", got.WorktreePath)
	}
	if len(got.AcceptanceCriteria) == 0 {
		t.Error("Lean field 'acceptance_criteria' was silently dropped")
	}
	if len(got.VerificationPlan) == 0 {
		t.Error("Lean field 'verification_plan' was silently dropped")
	}
}

func TestTaskListResponseIncludesCountsAndRegistryMetadata(t *testing.T) {
	backend := &obsidianTaskBackend{}
	backend.setListMetadata(taskListMetadata{
		Validation: "obsidian registry validated: 2 task(s), 1 diagnostic(s), 1 changed file(s)",
		Diagnostics: []taskRegistryDiagnostic{{
			Code:     "invalid_projection",
			File:     "bad.md",
			Message:  "skipped invalid Obsidian task note",
			Severity: "warning",
		}},
		ChangedFiles: []string{"repaired.md"},
	})
	all := []tasks.Task{
		{ID: "todo-task", Status: "todo", Title: "Todo"},
		{ID: "done-task", Status: "done", Title: "Done"},
	}

	response := taskListResponse(backend, all[:1], all, "obsidian_registry")

	counts, ok := response["counts_by_status"].(map[string]int)
	if !ok {
		t.Fatalf("counts_by_status type = %T", response["counts_by_status"])
	}
	if counts["todo"] != 1 || counts["done"] != 1 {
		t.Fatalf("counts_by_status = %#v", counts)
	}
	if response["validation"] == "" {
		t.Fatalf("validation missing from response: %#v", response)
	}
	if diagnostics, ok := response["diagnostics"].([]taskRegistryDiagnostic); !ok || len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", response["diagnostics"])
	}
	if changedFiles, ok := response["changed_files"].([]string); !ok || !stringSliceContains(changedFiles, "repaired.md") {
		t.Fatalf("changed_files = %#v", response["changed_files"])
	}
}
