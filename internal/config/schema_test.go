package config

import (
	"strings"
	"testing"
)

// README promises config_schema covers "every field"; task_registry.backend
// selects the registry backend and was absent from the schema, so a model
// configuring the helper could not discover the valid values.
func TestSchemaDocumentsTaskRegistryBackend(t *testing.T) {
	fields, ok := Schema()["fields"].([]FieldDoc)
	if !ok {
		t.Fatalf("schema fields have type %T, want []FieldDoc", Schema()["fields"])
	}
	for _, field := range fields {
		if field.Path != "task_registry.backend" {
			continue
		}
		if len(field.Examples) == 0 {
			t.Fatalf("task_registry.backend documents no allowed values")
		}
		return
	}
	t.Fatal("config schema is missing task_registry.backend")
}

func TestSchemaDocumentsConfigAllowRepositoryWorkflow(t *testing.T) {
	workflow, ok := Schema()["workflow"].([]string)
	if !ok {
		t.Fatalf("schema workflow has type %T, want []string", Schema()["workflow"])
	}
	joined := strings.Join(workflow, "\n")
	for _, fragment := range []string{"config_allow_repository", "startup repository", "process restart"} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("config schema workflow does not mention %q: %s", fragment, joined)
		}
	}
}
