package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, string, string) {
	t.Helper()
	helperRoot := t.TempDir()
	repo := t.TempDir()
	return NewStore(helperRoot), helperRoot, repo
}

func TestAddPersistsAndGetReadsBack(t *testing.T) {
	store, _, repo := newTestStore(t)
	note, err := store.Add(ScopeRepo, repo, "Release gotchas", "make quality takes ~48s", []string{"ci", "", "ci"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if want := "ci"; strings.Join(note.Tags, ",") != want {
		t.Errorf("tags: got %v, want [%s]", note.Tags, want)
	}
	if !strings.HasPrefix(note.ID, time.Now().UTC().Format("20060102")) {
		t.Errorf("id should start with the date: %s", note.ID)
	}
	wantPath := filepath.Join(repo, ".mcp-ai-helper", "notes", note.ID+".md")
	if note.Path != wantPath {
		t.Errorf("path: got %s, want %s", note.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("note file missing: %v", err)
	}
	got, err := store.Get(ScopeRepo, repo, note.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Release gotchas" || got.Body != "make quality takes ~48s" {
		t.Errorf("roundtrip changed the note: %+v", got)
	}
	if !got.CreatedAt.Equal(note.CreatedAt) || !got.UpdatedAt.Equal(note.UpdatedAt) {
		t.Errorf("timestamps changed: created %s/%s, updated %s/%s", got.CreatedAt, note.CreatedAt, got.UpdatedAt, note.UpdatedAt)
	}
}

func TestGlobalAndRepoNotebooksAreSeparate(t *testing.T) {
	store, helperRoot, repo := newTestStore(t)
	global, err := store.Add(ScopeGlobal, "", "Cross-repo lesson", "always snapshot before guarded_replace", nil)
	if err != nil {
		t.Fatalf("add global: %v", err)
	}
	if global.Path != filepath.Join(helperRoot, "notes", global.ID+".md") {
		t.Errorf("global path: %s", global.Path)
	}
	if global.Scope != ScopeGlobal {
		t.Errorf("scope: %s", global.Scope)
	}
	repoNotes, _, err := store.List(ScopeRepo, repo, "")
	if err != nil {
		t.Fatalf("list repo: %v", err)
	}
	if len(repoNotes) != 0 {
		t.Errorf("repo notebook should be empty, got %d", len(repoNotes))
	}
	globalNotes, _, err := store.List(ScopeGlobal, "", "")
	if err != nil {
		t.Fatalf("list global: %v", err)
	}
	if len(globalNotes) != 1 || globalNotes[0].ID != global.ID {
		t.Errorf("global notebook: %+v", globalNotes)
	}
}

func TestListSortsNewestFirstSkipsUnparsableAndFiltersTag(t *testing.T) {
	store, _, repo := newTestStore(t)
	first, err := store.Add(ScopeRepo, repo, "first", "one", nil)
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	second, err := store.Add(ScopeRepo, repo, "second", "two", []string{"important"})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	dir, err := store.Dir(ScopeRepo, repo)
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte("not frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, skipped, err := store.List(ScopeRepo, repo, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != first.ID {
		t.Errorf("list order: %+v", got)
	}
	if skipped != 1 {
		t.Errorf("skipped: %d", skipped)
	}
	tagged, _, err := store.List(ScopeRepo, repo, "important")
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if len(tagged) != 1 || tagged[0].ID != second.ID {
		t.Errorf("tag filter: %+v", tagged)
	}
}

func TestSearchIsCaseInsensitiveWithSnippet(t *testing.T) {
	store, _, repo := newTestStore(t)
	note, err := store.Add(ScopeRepo, repo, "Build gate", "The gate to run before commit is make quality, about 48 seconds.", nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	matches, err := store.Search(ScopeRepo, repo, "MAKE QUALITY", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(matches) != 1 || matches[0].ID != note.ID {
		t.Fatalf("matches: %+v", matches)
	}
	if !strings.Contains(matches[0].Snippet, "make quality") {
		t.Errorf("snippet: %q", matches[0].Snippet)
	}
	if matches[0].Offset < 0 || matches[0].Offset >= len(note.Body) {
		t.Errorf("offset: %d", matches[0].Offset)
	}
}

func TestUpdateReplacesOnlyGivenFields(t *testing.T) {
	store, _, repo := newTestStore(t)
	note, err := store.Add(ScopeRepo, repo, "old title", "old body", []string{"a"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	newTitle := "new title"
	updated, err := store.Update(ScopeRepo, repo, note.ID, UpdateFields{Title: &newTitle})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "new title" || updated.Body != "old body" {
		t.Errorf("update: %+v", updated)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "a" {
		t.Errorf("tags should be kept: %v", updated.Tags)
	}
	if !updated.UpdatedAt.After(note.UpdatedAt) {
		t.Errorf("updated_at should advance: %s", updated.UpdatedAt)
	}
	if _, err := store.Update(ScopeRepo, repo, note.ID, UpdateFields{}); err == nil {
		t.Errorf("empty update should fail")
	}
	reread, err := store.Get(ScopeRepo, repo, note.ID)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if reread.Title != "new title" {
		t.Errorf("update not persisted: %+v", reread)
	}
}

func TestDeleteRemovesAndReportsMissing(t *testing.T) {
	store, _, repo := newTestStore(t)
	note, err := store.Add(ScopeRepo, repo, "temp", "gone soon", nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := store.Delete(ScopeRepo, repo, note.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	err = store.Delete(ScopeRepo, repo, note.ID)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("second delete: %v", err)
	}
}

func TestRepoScopeRequiresRepoPath(t *testing.T) {
	store, _, _ := newTestStore(t)
	if _, err := store.Add(ScopeRepo, "", "t", "b", nil); err == nil {
		t.Errorf("add without repo_path should fail")
	}
}

func TestUnsafeNoteIDIsRejected(t *testing.T) {
	store, _, repo := newTestStore(t)
	_, err := store.Get(ScopeRepo, repo, "../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "invalid note id") {
		t.Errorf("unsafe id: %v", err)
	}
}
