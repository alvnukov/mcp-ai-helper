package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestExportLeanToObsidian(t *testing.T) {
	dir := t.TempDir()
	obsidian := newObsidianTaskBackend(dir)
	lean := newRecordingTaskBackend()
	result, err := exportTasks(nil, lean, obsidian, "/repo", ImportExportRequest{})
	if err != nil {
		t.Fatalf("exportTasks: %v", err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(result.Added))
	}
	if result.Added[0].ID != "task-1" {
		t.Fatalf("id = %q", result.Added[0].ID)
	}
	got, _, err := obsidian.Get(nil, "", "task-1")
	if err != nil {
		t.Fatalf("Get from obsidian: %v", err)
	}
	if got.Title != "First" {
		t.Fatalf("title = %q", got.Title)
	}
}

func TestExportDryRun(t *testing.T) {
	dir := t.TempDir()
	obsidian := newObsidianTaskBackend(dir)
	lean := newRecordingTaskBackend()
	result, err := exportTasks(nil, lean, obsidian, "/repo", ImportExportRequest{DryRun: true})
	if err != nil {
		t.Fatalf("exportTasks: %v", err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added in dry-run, got %d", len(result.Added))
	}
	if _, err := os.Stat(dir + "/task-1.md"); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write files")
	}
}

func TestExportConflictDetection(t *testing.T) {
	dir := t.TempDir()
	obsidian := newObsidianTaskBackend(dir)
	obsidian.Upsert(nil, tasks.AddRequest{ID: "task-1", Status: "done", Title: "Existing"})
	lean := newRecordingTaskBackend()
	result, err := exportTasks(nil, lean, obsidian, "/repo", ImportExportRequest{})
	if !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("expected duplicate ID error, got %v", err)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0] != "task-1" {
		t.Fatalf("expected conflict for task-1, got %v", result.Conflicts)
	}
}

func TestExportConflictFailsWithoutPartialWrites(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	source := newObsidianTaskBackend(sourceDir)
	target := newObsidianTaskBackend(targetDir)
	if _, err := source.Upsert(nil, tasks.AddRequest{ID: "a-new", Status: "todo", Title: "New"}); err != nil {
		t.Fatal(err)
	}
	if _, err := source.Upsert(nil, tasks.AddRequest{ID: "b-existing", Status: "todo", Title: "Existing From Source"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Upsert(nil, tasks.AddRequest{ID: "b-existing", Status: "done", Title: "Existing Target"}); err != nil {
		t.Fatal(err)
	}
	_, err := exportTasks(nil, source, target, "/repo", ImportExportRequest{})
	if !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("expected duplicate ID error, got %v", err)
	}
	if _, err := os.Stat(targetDir + "/a-new.md"); !os.IsNotExist(err) {
		t.Fatalf("conflicting export must not partially write a-new.md")
	}
}

func TestExportRoundTrip(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".worktrees", "roundtrip"), 0o700); err != nil {
		t.Fatal(err)
	}
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	be1 := newObsidianTaskBackend(dir1)
	be2 := newObsidianTaskBackend(dir2)
	createdAt := time.Date(2026, 5, 1, 12, 0, 0, 123, time.UTC)
	updatedAt := time.Date(2026, 5, 2, 13, 0, 0, 456, time.UTC)
	be1.Upsert(nil, tasks.AddRequest{
		RepoPath: repo,
		ID:       "roundtrip", Status: "todo", Title: "Round Trip",
		Priority: "high", ModelLevel: "medium",
		TaskType: "feature", Branch: "feature/roundtrip", WorktreePath: ".worktrees/roundtrip",
		ParentID:           "parent-task",
		Body:               "Test body.",
		Tags:               []string{"test"},
		AcceptanceCriteria: []string{"Must survive round trip"},
		VerificationPlan:   []string{"Export", "Import", "Verify"},
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	})
	_, err := exportTasks(nil, be1, be2, repo, ImportExportRequest{})
	if err != nil {
		t.Fatalf("first export: %v", err)
	}
	got, _, err := be2.Get(nil, repo, "roundtrip")
	if err != nil {
		t.Fatalf("Get from be2: %v", err)
	}
	if got.Title != "Round Trip" || got.Priority != "high" || got.ModelLevel != "medium" {
		t.Fatalf("fields lost: %+v", got)
	}
	if got.TaskType != "feature" || got.Branch != "feature/roundtrip" || got.WorktreePath != ".worktrees/roundtrip" || got.ParentID != "parent-task" {
		t.Fatalf("worktree fields lost: %+v", got)
	}
	if got.Body != "Test body." {
		t.Fatalf("body = %q", got.Body)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "test" {
		t.Fatalf("tags = %v", got.Tags)
	}
	if len(got.AcceptanceCriteria) != 1 {
		t.Fatalf("acceptance_criteria lost: %v", got.AcceptanceCriteria)
	}
	if len(got.VerificationPlan) != 3 {
		t.Fatalf("verification_plan lost: %v", got.VerificationPlan)
	}
	if !got.CreatedAt.Equal(createdAt) || !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("timestamps lost: created=%s updated=%s", got.CreatedAt, got.UpdatedAt)
	}
	if !got.WorktreeExists || !strings.HasSuffix(got.CodePath, filepath.Join(".worktrees", "roundtrip")) {
		t.Fatalf("worktree context missing: exists=%v code_path=%q", got.WorktreeExists, got.CodePath)
	}
}

func TestExportLeanToCleanObsidianPreservesAllProjectedFields(t *testing.T) {
	repo := t.TempDir()
	source := newLakeTaskBackend(commandRunnerForRepo(repo), legacyStoreForTest(t))
	targetDir := t.TempDir()
	target := newObsidianTaskBackend(targetDir)

	_, err := source.Upsert(context.Background(), tasks.AddRequest{
		RepoPath: repo,
		ID:       "lean-clean-export", Status: "todo", Title: "Lean Clean Export",
		Priority: "high", ModelLevel: "medium",
		TaskType: "design_epic",
		Body:     "Lean source body.",
		Tags:     []string{"lean", "export"},
		AcceptanceCriteria: []string{
			"All projected fields survive export",
		},
		VerificationPlan: []string{"Export to clean Obsidian target", "Read target back"},
	})
	if err != nil {
		t.Fatalf("seed Lean source: %v", err)
	}

	before, _, err := target.ListAll(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListAll before export: %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("clean target is not empty: %d", len(before))
	}

	sourceTasks, _, err := source.ListAll(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListAll from Lean source: %v", err)
	}
	if len(sourceTasks) == 0 {
		t.Fatal("Lean source returned no tasks")
	}
	sourceByID := make(map[string]tasks.Task, len(sourceTasks))
	for _, task := range sourceTasks {
		sourceByID[task.ID] = task
	}
	if _, ok := sourceByID["lean-clean-export"]; !ok {
		t.Fatalf("seeded Lean task missing from source list: %v", sourceByID)
	}

	result, err := exportTasks(context.Background(), source, target, repo, ImportExportRequest{})
	if err != nil {
		t.Fatalf("export Lean to clean Obsidian: %v", err)
	}
	if len(result.Added) != len(sourceTasks) || len(result.Updated) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("unexpected export result: added=%d updated=%d conflicts=%v source=%d", len(result.Added), len(result.Updated), result.Conflicts, len(sourceTasks))
	}

	exported, _, err := target.ListAll(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListAll exported Obsidian target: %v", err)
	}
	if len(exported) != len(sourceTasks) {
		t.Fatalf("exported task count = %d, want %d", len(exported), len(sourceTasks))
	}
	for _, got := range exported {
		want, ok := sourceByID[got.ID]
		if !ok {
			t.Fatalf("exported unexpected task %s", got.ID)
		}
		assertExportedTaskPreserved(t, want, got)
	}
}

func TestExportDuplicateSourceIDsFailsWithoutPartialWrites(t *testing.T) {
	source := listOnlyTaskBackend{items: []tasks.Task{
		{ID: "dup", Status: "todo", Title: "First"},
		{ID: "dup", Status: "blocked", Title: "Second"},
	}}
	target := newObsidianTaskBackend(t.TempDir())

	result, err := exportTasks(context.Background(), source, target, t.TempDir(), ImportExportRequest{})
	if !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("expected duplicate source ID error, got %v", err)
	}
	if result == nil || len(result.Conflicts) != 1 || result.Conflicts[0] != "dup" || len(result.Losses) != 1 {
		t.Fatalf("unexpected duplicate result: %+v", result)
	}
	exported, _, listErr := target.ListAll(context.Background(), "")
	if listErr != nil {
		t.Fatalf("ListAll target: %v", listErr)
	}
	if len(exported) != 0 {
		t.Fatalf("duplicate source export must not partially write, got %d tasks", len(exported))
	}
}

type listOnlyTaskBackend struct {
	items []tasks.Task
}

func (b listOnlyTaskBackend) ListCurrent(context.Context, string) ([]tasks.Task, string, error) {
	return append([]tasks.Task(nil), b.items...), "test_backend", nil
}

func (b listOnlyTaskBackend) ListAll(ctx context.Context, repoPath string) ([]tasks.Task, string, error) {
	return b.ListCurrent(ctx, repoPath)
}

func (b listOnlyTaskBackend) Get(context.Context, string, string) (tasks.Task, string, error) {
	return tasks.Task{}, "test_backend", errors.New("not implemented")
}

func (b listOnlyTaskBackend) Upsert(context.Context, tasks.AddRequest) (taskMutationResult, error) {
	return taskMutationResult{}, errors.New("not implemented")
}

func (b listOnlyTaskBackend) SetStatus(context.Context, tasks.StatusRequest) (taskMutationResult, error) {
	return taskMutationResult{}, errors.New("not implemented")
}

func (b listOnlyTaskBackend) BatchUpsert(context.Context, tasks.BatchUpsertRequest) (taskBatchMutationResult, error) {
	return taskBatchMutationResult{}, errors.New("not implemented")
}

func (b listOnlyTaskBackend) Delete(context.Context, tasks.DeleteRequest) (taskMutationResult, error) {
	return taskMutationResult{}, errors.New("not implemented")
}

func equalStringLists(a []string, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func assertExportedTaskPreserved(t *testing.T, want tasks.Task, got tasks.Task) {
	t.Helper()
	if got.ProjectionSource != "obsidian_registry" {
		t.Fatalf("%s projection_source = %q", want.ID, got.ProjectionSource)
	}
	if got.ID != want.ID || got.TaskType != want.TaskType || got.Branch != want.Branch || got.WorktreePath != want.WorktreePath || got.ParentID != want.ParentID || got.Status != want.Status || got.Title != want.Title || got.Body != want.Body || got.Priority != want.Priority || got.ModelLevel != want.ModelLevel {
		t.Fatalf("%s scalar fields changed:\nwant=%+v\ngot=%+v", want.ID, want, got)
	}
	if !equalStringLists(got.Tags, want.Tags) || !equalStringLists(got.AcceptanceCriteria, want.AcceptanceCriteria) || !equalStringLists(got.VerificationPlan, want.VerificationPlan) {
		t.Fatalf("%s list fields changed:\nwant=%+v\ngot=%+v", want.ID, want, got)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Fatalf("%s timestamps changed: want created=%s updated=%s got created=%s updated=%s", want.ID, want.CreatedAt, want.UpdatedAt, got.CreatedAt, got.UpdatedAt)
	}
	if got.CodePath != want.CodePath || got.WorktreeExists != want.WorktreeExists {
		t.Fatalf("%s worktree context changed: want exists=%v code_path=%q got exists=%v code_path=%q", want.ID, want.WorktreeExists, want.CodePath, got.WorktreeExists, got.CodePath)
	}
}
