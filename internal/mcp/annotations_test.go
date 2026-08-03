package mcp

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// mcp-go's NewTool defaults every tool to ReadOnlyHint=false,
// DestructiveHint=true. With no annotations set (the prior state), clients
// that auto-approve read-only calls refused every helper tool — even pure-read
// ones like `file`. The six action-dispatch tools now set their annotations
// explicitly. This test pins them so a regression is caught at the tool that
// most affects client auto-approval behaviour.
func TestActionDispatchToolsHaveExplicitAnnotations(t *testing.T) {
	t.Parallel()

	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	tools := srv.ListTools()

	type want struct {
		readOnly    *bool
		destructive *bool
		idempotent  *bool
		openWorld   *bool
	}
	boolPtr := func(v bool) *bool { return &v }
	cases := map[string]want{
		// All five actions (read, read_many, list, search, snapshot) are pure reads.
		"file": {readOnly: boolPtr(true), destructive: boolPtr(false), idempotent: boolPtr(true), openWorld: boolPtr(false)},
		// replace/write mutate repo files.
		"edit": {destructive: boolPtr(true), idempotent: boolPtr(false), openWorld: boolPtr(false)},
		// commit mutates; status/diff/log read, but the tool is mixed.
		"git": {destructive: boolPtr(true), idempotent: boolPtr(false), openWorld: boolPtr(false)},
		// run executes arbitrary shell (network, fs).
		"command": {destructive: boolPtr(true), idempotent: boolPtr(false), openWorld: boolPtr(true)},
		// upsert/set_status/batch_upsert/delete mutate; current/get/list/search read.
		"task": {destructive: boolPtr(true), idempotent: boolPtr(false), openWorld: boolPtr(false)},
		// pipeline/workflow run commands and mutate files.
		"run": {destructive: boolPtr(true), idempotent: boolPtr(false), openWorld: boolPtr(true)},
	}
	for name, w := range cases {
		entry, ok := tools[name]
		if !ok {
			t.Errorf("tool %q is not registered", name)
			continue
		}
		a := entry.Tool.Annotations
		// Annotation is considered "explicit" when ReadOnlyHint is set — the
		// mcp-go default leaves it nil, so a non-nil value means the tool set it.
		if a.ReadOnlyHint == nil {
			t.Errorf("%q: ReadOnlyHint is nil (default-leaking); annotations must be set explicitly", name)
		}
		if w.readOnly != nil && (a.ReadOnlyHint == nil || *a.ReadOnlyHint != *w.readOnly) {
			t.Errorf("%q: ReadOnlyHint = %v, want %v", name, a.ReadOnlyHint, *w.readOnly)
		}
		if w.destructive != nil && (a.DestructiveHint == nil || *a.DestructiveHint != *w.destructive) {
			t.Errorf("%q: DestructiveHint = %v, want %v", name, a.DestructiveHint, *w.destructive)
		}
		if w.idempotent != nil && (a.IdempotentHint == nil || *a.IdempotentHint != *w.idempotent) {
			t.Errorf("%q: IdempotentHint = %v, want %v", name, a.IdempotentHint, *w.idempotent)
		}
		if w.openWorld != nil && (a.OpenWorldHint == nil || *a.OpenWorldHint != *w.openWorld) {
			t.Errorf("%q: OpenWorldHint = %v, want %v", name, a.OpenWorldHint, *w.openWorld)
		}
	}
}
