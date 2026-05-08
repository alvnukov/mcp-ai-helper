package language

import "testing"

func TestDefaultRegistryIncludesGo(t *testing.T) {
	profile, ok := DefaultRegistry().Get("go")
	if !ok {
		t.Fatal("go profile missing")
	}
	if !profile.ImportCheck {
		t.Fatal("go profile should require import checks")
	}
	want := "Run gofmt only on .go files, never on README or YAML files."
	found := false
	for _, guardrail := range profile.Guardrails {
		if guardrail == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("go profile missing guardrail %q", want)
	}
}

func TestDetectMatchesGoFiles(t *testing.T) {
	profiles := DefaultRegistry().Detect([]string{"internal/pipeline/pipeline.go", "README.md", "go.mod"})
	if len(profiles) != 1 || profiles[0].ID != "go" {
		t.Fatalf("profiles = %#v, want go", profiles)
	}
}
