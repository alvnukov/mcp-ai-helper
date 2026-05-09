package mcp

import (
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestUniqueStrings(t *testing.T) {
	got := uniqueStrings([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUniqueStringsTrimsWhitespace(t *testing.T) {
	got := uniqueStrings([]string{" a ", "b", "  a  "})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got = %#v, want [a b]", got)
	}
}

func TestUniqueStringsSkipsEmpty(t *testing.T) {
	got := uniqueStrings([]string{"", "a", "  ", "b"})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestUniqueStringsEmptyInput(t *testing.T) {
	if got := uniqueStrings(nil); len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
	if got := uniqueStrings([]string{}); len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestCurrentTasks(t *testing.T) {
	list := []tasks.Task{
		{ID: "1", Status: "todo"},
		{ID: "2", Status: "in_progress"},
		{ID: "3", Status: "blocked"},
		{ID: "4", Status: "done"},
		{ID: "5", Status: "deleted"},
	}
	got := currentTasks(list)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	for _, task := range got {
		switch task.Status {
		case "todo", "in_progress", "blocked":
		default:
			t.Fatalf("unexpected status %q in current tasks", task.Status)
		}
	}
}

func TestCurrentTasksEmpty(t *testing.T) {
	if got := currentTasks(nil); len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}
