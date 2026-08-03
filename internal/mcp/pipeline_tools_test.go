package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"
)

// schemaStepTools decodes the step list out of run action=schema. Everything
// below a step entry arrives as JSON, so nested objects are map[string]any —
// asserting any narrower type here silently yields nil and makes the caller's
// checks vacuous.
func schemaStepTools(t *testing.T) []map[string]any {
	t.Helper()

	result, err := runActionSchema()
	if err != nil {
		t.Fatalf("runActionSchema: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("schema result has no content")
	}
	textContent, ok := basemcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("schema result Content[0] is not TextContent: %T", result.Content[0])
	}
	var payload struct {
		StepTools []map[string]any `json:"step_tools"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &payload); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if len(payload.StepTools) == 0 {
		t.Fatal("step_tools is empty or absent; the schema must list the workflow steps under \"step_tools\"")
	}
	return payload.StepTools
}

// runActionSchema tells the model how to build workflow steps. The engine
// dispatches on WorkflowStep.Tool (pipeline.go: switch step.Tool), so each entry
// must carry its name under the key "tool" — not "type". A model that followed
// the old schema and built steps with "type" landed in the default case with
// Reason "unknown workflow tool: " and had to guess why.
func TestRunActionSchemaUsesToolNotType(t *testing.T) {
	t.Parallel()

	want := map[string]bool{
		"command": true, "guarded_replace": true, "task_batch_upsert": true,
		"task_transition": true, "git_commit_owned": true, "git_prepare_task_worktree": true,
	}
	got := make(map[string]bool, len(want))
	for _, entry := range schemaStepTools(t) {
		if rawType, hasType := entry["type"]; hasType {
			t.Errorf("step entry still uses the misleading \"type\" key (value=%v); engine dispatches on \"tool\"", rawType)
		}
		name, ok := entry["tool"].(string)
		if !ok {
			t.Errorf("step entry has no string \"tool\" key: %#v", entry)
			continue
		}
		got[name] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("step_tools missing tool %q", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("step_tools has unexpected tool %q", name)
		}
	}
}

// guarded_replace inside a workflow binds only old/new; old_b64/new_b64 are not
// decoded by the step (WorkflowEdit has no such fields). The schema must not
// advertise them for the workflow path — that sends a model toward a step that
// silently drops its payload.
func TestRunActionSchemaDoesNotAdvertiseBase64ForWorkflowGuardedReplace(t *testing.T) {
	t.Parallel()

	var checked bool
	for _, entry := range schemaStepTools(t) {
		if name, _ := entry["tool"].(string); name != "guarded_replace" {
			continue
		}
		checked = true
		fields, ok := entry["fields"].(map[string]any)
		if !ok || len(fields) == 0 {
			t.Fatalf("guarded_replace entry has no readable fields map (%T); the base64 check below would pass on nothing", entry["fields"])
		}
		for field := range fields {
			if strings.Contains(field, "b64") {
				t.Errorf("guarded_replace schema still advertises %q as a workflow field; base64 is only supported by edit action=replace", field)
			}
		}
		// The restriction must be stated so a model with backslash-heavy text
		// knows to reach for edit action=replace instead of the workflow step.
		desc, _ := entry["description"].(string)
		if !strings.Contains(desc, "edit action=replace") {
			t.Errorf("guarded_replace description must point to edit action=replace for base64; got %q", desc)
		}
	}
	if !checked {
		t.Fatal("no guarded_replace entry in step_tools; nothing was checked")
	}
}

// The schema must name every argument the step reads, or a model cannot reach
// the ones that decide what close_missing writes and to which tasks.
func TestRunActionSchemaDocumentsBatchUpsertCloseMissingArguments(t *testing.T) {
	t.Parallel()

	for _, entry := range schemaStepTools(t) {
		if name, _ := entry["tool"].(string); name != "task_batch_upsert" {
			continue
		}
		fields, ok := entry["fields"].(map[string]any)
		if !ok {
			t.Fatalf("task_batch_upsert entry has no readable fields map: %T", entry["fields"])
		}
		for _, field := range []string{"tasks", "close_missing", "missing_status", "active_statuses"} {
			if _, present := fields[field]; !present {
				t.Errorf("task_batch_upsert schema does not document %q", field)
			}
		}
		return
	}
	t.Fatal("no task_batch_upsert entry in step_tools")
}
