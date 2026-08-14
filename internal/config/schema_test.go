package config

import "testing"

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
