package mcp

import (
	"strings"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// config.Load and MergeRepoConfig already reject bad backend values, but
// buildTaskBackend used to answer any unknown value with the Lean backend
// anyway. A config that ever reaches this seam unvalidated would silently
// write to the wrong registry, so the seam itself has to fail closed.
func TestBuildTaskBackendFailsClosed(t *testing.T) {
	if _, err := buildTaskBackend(&config.Config{}, nil, nil); err != nil {
		t.Fatalf("empty backend must default to lean: %v", err)
	}
	if _, err := buildTaskBackend(&config.Config{TaskRegistry: config.TaskRegistryConfig{Backend: "lean"}}, nil, nil); err != nil {
		t.Fatalf("explicit lean backend rejected: %v", err)
	}
	if _, err := buildTaskBackend(&config.Config{TaskRegistry: config.TaskRegistryConfig{Backend: "obsidian"}}, nil, nil); err == nil || !strings.Contains(err.Error(), "path is required") {
		t.Fatalf("obsidian without a path: err = %v, want a path-required error", err)
	}
	if _, err := buildTaskBackend(&config.Config{TaskRegistry: config.TaskRegistryConfig{Backend: "obsidain"}}, nil, nil); err == nil || !strings.Contains(err.Error(), "unsupported task_registry.backend") { //nolint:misspell // the misspelling is the point
		t.Fatalf("typo backend: err = %v, want an unsupported-backend error", err)
	}
}
