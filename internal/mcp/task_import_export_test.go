package mcp

import (
	"errors"
	"os"
	"testing"

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
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	be1 := newObsidianTaskBackend(dir1)
	be2 := newObsidianTaskBackend(dir2)
	be1.Upsert(nil, tasks.AddRequest{
		ID: "roundtrip", Status: "todo", Title: "Round Trip",
		Priority: "high", ModelLevel: "medium",
		Body:               "Test body.",
		Tags:               []string{"test"},
		AcceptanceCriteria: []string{"Must survive round trip"},
		VerificationPlan:   []string{"Export", "Import", "Verify"},
	})
	_, err := exportTasks(nil, be1, be2, "/repo", ImportExportRequest{})
	if err != nil {
		t.Fatalf("first export: %v", err)
	}
	got, _, err := be2.Get(nil, "", "roundtrip")
	if err != nil {
		t.Fatalf("Get from be2: %v", err)
	}
	if got.Title != "Round Trip" || got.Priority != "high" || got.ModelLevel != "medium" {
		t.Fatalf("fields lost: %+v", got)
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
}
