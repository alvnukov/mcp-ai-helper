package mcp

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// mcp-go's NewTool seeds every annotation field with a non-nil default
// (ReadOnly=false, Destructive=true, Idempotent=false, OpenWorld=true), so a
// nil check can never catch a regression and a tool whose annotations merely
// happen to equal the defaults is indistinguishable from one that set none.
//
// The six action-dispatch tools set all four hints explicitly so the published
// annotations are self-documenting and do not depend on whatever NewTool's
// defaults happen to be. This test pins the exact published value of each hint
// per tool; dropping any With*HintAnnotation call, or flipping one, fails here.
func TestActionDispatchToolsHaveExplicitAnnotations(t *testing.T) {
	t.Parallel()

	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	tools := srv.ListTools()

	type want struct {
		readOnly    bool
		destructive bool
		idempotent  bool
		openWorld   bool
	}
	cases := map[string]want{
		// All five actions (read, read_many, list, search, snapshot) are pure reads.
		"file": {readOnly: true, destructive: false, idempotent: true, openWorld: false},
		// replace/write mutate repo files.
		"edit": {readOnly: false, destructive: true, idempotent: false, openWorld: false},
		// commit mutates; status/diff/log read, but the tool is mixed.
		"git": {readOnly: false, destructive: true, idempotent: false, openWorld: false},
		// run executes arbitrary shell (network, fs).
		"command": {readOnly: false, destructive: true, idempotent: false, openWorld: true},
		// upsert/set_status/batch_upsert/delete mutate; current/get/list/search read.
		"task": {readOnly: false, destructive: true, idempotent: false, openWorld: false},
		// pipeline/workflow run commands and mutate files.
		"run": {readOnly: false, destructive: true, idempotent: false, openWorld: true},
	}
	check := func(got *bool, want bool, name, field string) {
		if got == nil {
			t.Errorf("%q: %s is nil; all four hints must be set explicitly", name, field)
			return
		}
		if *got != want {
			t.Errorf("%q: %s = %v, want %v", name, field, *got, want)
		}
	}
	for name, w := range cases {
		entry, ok := tools[name]
		if !ok {
			t.Errorf("tool %q is not registered", name)
			continue
		}
		a := entry.Tool.Annotations
		check(a.ReadOnlyHint, w.readOnly, name, "ReadOnlyHint")
		check(a.DestructiveHint, w.destructive, name, "DestructiveHint")
		check(a.IdempotentHint, w.idempotent, name, "IdempotentHint")
		check(a.OpenWorldHint, w.openWorld, name, "OpenWorldHint")
	}
}
