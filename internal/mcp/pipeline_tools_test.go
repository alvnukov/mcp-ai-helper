package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/pipeline"
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

// The schema and the binder must name the same arguments. A field the schema
// advertises but WorkflowEdit does not carry sends a model toward a payload that
// json.Unmarshal drops without a word; a field WorkflowEdit carries but the
// schema omits is one nobody knows to use. Both directions are checked against
// the struct the step actually decodes into, because that struct is what decides
// which arguments survive.
func TestRunActionSchemaNamesEveryFieldGuardedReplaceBinds(t *testing.T) {
	t.Parallel()

	bound := map[string]bool{}
	editType := reflect.TypeOf(pipeline.WorkflowEdit{})
	for i := range editType.NumField() {
		name, _, _ := strings.Cut(editType.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			bound[name] = true
		}
	}

	var checked bool
	for _, entry := range schemaStepTools(t) {
		if name, _ := entry["tool"].(string); name != "guarded_replace" {
			continue
		}
		checked = true
		fields, ok := entry["fields"].(map[string]any)
		if !ok || len(fields) == 0 {
			t.Fatalf("guarded_replace entry has no readable fields map (%T); the checks below would pass on nothing", entry["fields"])
		}
		for field := range fields {
			if !bound[field] {
				t.Errorf("guarded_replace schema advertises %q, which WorkflowEdit does not bind; a step sending it would lose it silently", field)
			}
		}
		for field := range bound {
			if _, present := fields[field]; !present {
				t.Errorf("WorkflowEdit binds %q but the schema does not name it; a model cannot reach an argument it is never told about", field)
			}
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
