package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"
)

// runActionSchema tells the model how to build workflow steps. The engine
// dispatches on WorkflowStep.Tool (pipeline.go: switch step.Tool), so each
// step_types entry must carry its name under the key "tool" — not "type". A
// model that followed the old schema and built steps with "type" landed in the
// default case with Reason "unknown workflow tool: " and had to guess why.
func TestRunActionSchemaUsesToolNotType(t *testing.T) {
	t.Parallel()

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
	text := textContent.Text
	var payload struct {
		StepTypes []map[string]any `json:"step_types"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if len(payload.StepTypes) == 0 {
		t.Fatal("step_types is empty")
	}
	want := map[string]bool{
		"command": true, "guarded_replace": true, "task_batch_upsert": true,
		"task_transition": true, "git_commit_owned": true, "git_prepare_task_worktree": true,
	}
	got := make(map[string]bool, len(payload.StepTypes))
	for _, entry := range payload.StepTypes {
		rawType, hasType := entry["type"]
		if hasType {
			t.Errorf("step_types entry still uses the misleading \"type\" key (value=%v); engine dispatches on \"tool\"", rawType)
		}
		name, ok := entry["tool"].(string)
		if !ok {
			t.Errorf("step_types entry has no string \"tool\" key: %#v", entry)
			continue
		}
		got[name] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("step_types missing tool %q", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("step_types has unexpected tool %q", name)
		}
	}
}

// guarded_replace inside a workflow binds only old/new; old_b64/new_b64 are not
// decoded by the step (WorkflowEdit has no such fields). The schema must not
// advertise them for the workflow path — that sends a model toward a step that
// silently drops its payload.
func TestRunActionSchemaDoesNotAdvertiseBase64ForWorkflowGuardedReplace(t *testing.T) {
	t.Parallel()

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
	text := textContent.Text
	var payload struct {
		StepTypes []map[string]any `json:"step_types"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	for _, entry := range payload.StepTypes {
		name, _ := entry["tool"].(string)
		if name != "guarded_replace" {
			continue
		}
		desc, _ := entry["description"].(string)
		fields, _ := entry["fields"].(map[string]string)
		for field := range fields {
			if strings.Contains(field, "b64") {
				t.Errorf("guarded_replace schema still advertises %q as a workflow field; base64 is only supported by edit action=replace", field)
			}
		}
		// The restriction must be stated so a model with backslash-heavy text
		// knows to reach for edit action=replace instead of the workflow step.
		if !strings.Contains(desc, "edit action=replace") {
			t.Errorf("guarded_replace description must point to edit action=replace for base64; got %q", desc)
		}
	}
}
