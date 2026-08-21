package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/notes"
)

func noteTestDeps(t *testing.T) *Server {
	t.Helper()
	return &Server{notesStore: notes.NewStore(t.TempDir())}
}

func notePayload(t *testing.T, handler actionHandler, args map[string]any) map[string]any {
	t.Helper()
	result := callWithAction(t, handler, args)
	if result.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, result))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}

func noteErrorText(t *testing.T, handler actionHandler, args map[string]any) string {
	t.Helper()
	result := callWithAction(t, handler, args)
	if !result.IsError {
		t.Fatalf("expected an error, got: %s", resultText(t, result))
	}
	return resultText(t, result)
}

func noteField(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	note, ok := payload["note"].(map[string]any)
	if !ok {
		t.Fatalf("no note in payload: %+v", payload)
	}
	return note
}

func TestNoteActionsRoundtripInRepoScope(t *testing.T) {
	deps := noteTestDeps(t)
	repo := t.TempDir()
	added := notePayload(t, withDeps(noteActionAdd, deps), map[string]any{
		"repo_path": repo, "title": "Gate", "body": "run make quality", "tags": []any{"ci"},
	})
	id, _ := noteField(t, added)["id"].(string)
	if id == "" {
		t.Fatalf("no id: %+v", added)
	}

	read := notePayload(t, withDeps(noteActionRead, deps), map[string]any{"repo_path": repo, "id": id})
	if got := noteField(t, read)["body"]; got != "run make quality" {
		t.Errorf("read body: %v", got)
	}

	list := notePayload(t, withDeps(noteActionList, deps), map[string]any{"repo_path": repo})
	if items, _ := list["notes"].([]any); len(items) != 1 {
		t.Errorf("list: %+v", list)
	}

	found := notePayload(t, withDeps(noteActionSearch, deps), map[string]any{"repo_path": repo, "query": "QUALITY"})
	if items, _ := found["matches"].([]any); len(items) != 1 {
		t.Errorf("search: %+v", found)
	}

	updated := notePayload(t, withDeps(noteActionUpdate, deps), map[string]any{
		"repo_path": repo, "id": id, "title": "Gate v2",
	})
	if got := noteField(t, updated)["title"]; got != "Gate v2" {
		t.Errorf("update title: %v", got)
	}

	deleted := notePayload(t, withDeps(noteActionDelete, deps), map[string]any{"repo_path": repo, "id": id})
	if deleted["status"] != "ok" {
		t.Errorf("delete: %+v", deleted)
	}

	errText := noteErrorText(t, withDeps(noteActionRead, deps), map[string]any{"repo_path": repo, "id": id})
	if !strings.Contains(errText, "not found") {
		t.Errorf("read after delete: %s", errText)
	}
}

func TestNoteGlobalScopeNeedsNoRepoPath(t *testing.T) {
	deps := noteTestDeps(t)
	added := notePayload(t, withDeps(noteActionAdd, deps), map[string]any{
		"scope": "global", "title": "Lesson", "body": "snapshot before replace",
	})
	if got := noteField(t, added)["scope"]; got != "global" {
		t.Errorf("scope: %v", got)
	}
	list := notePayload(t, withDeps(noteActionList, deps), map[string]any{"scope": "global"})
	if items, _ := list["notes"].([]any); len(items) != 1 {
		t.Errorf("global list: %+v", list)
	}
}

func TestNoteScopeDefaultsToRepoAndRejectsUnknown(t *testing.T) {
	deps := noteTestDeps(t)
	errText := noteErrorText(t, withDeps(noteActionAdd, deps), map[string]any{"title": "t", "body": "b"})
	if !strings.Contains(errText, "repo_path") {
		t.Errorf("default scope should demand repo_path: %s", errText)
	}
	errText = noteErrorText(t, withDeps(noteActionAdd, deps), map[string]any{"scope": "team", "title": "t", "body": "b"})
	if !strings.Contains(errText, "repo or global") {
		t.Errorf("unknown scope: %s", errText)
	}
}

func TestNoteUpdateWithoutFieldsFails(t *testing.T) {
	deps := noteTestDeps(t)
	repo := t.TempDir()
	added := notePayload(t, withDeps(noteActionAdd, deps), map[string]any{"repo_path": repo, "title": "t", "body": "b"})
	id, _ := noteField(t, added)["id"].(string)
	errText := noteErrorText(t, withDeps(noteActionUpdate, deps), map[string]any{"repo_path": repo, "id": id})
	if !strings.Contains(errText, "at least one") {
		t.Errorf("empty update: %s", errText)
	}
}
