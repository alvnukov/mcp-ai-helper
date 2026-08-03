package mcp

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
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

// structured() is the single place that decides the wire shape of every
// successful tool result, so the contract belongs here rather than in each
// tool's own test. The payload goes in content and nowhere else: repeating it
// in structuredContent would put the same bytes on the wire twice, and that
// field earns its place only beside a declared outputSchema, which no tool in
// this server declares.
func TestStructuredResultCarriesPayloadOnce(t *testing.T) {
	res, err := structured(map[string]any{"status": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if res.StructuredContent != nil {
		t.Fatalf("payload repeated in structuredContent: %#v", res.StructuredContent)
	}
	want := `{"status":"ok"}`
	if got := resultText(t, res); got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
