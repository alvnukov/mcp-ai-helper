package accesspolicy

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPolicyRejectsProtectedPathsWithSpecificAlternatives(t *testing.T) {
	repo := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), ".mcp-ai-helper", "config.yaml")
	policy := New(repo, "private-tasks", globalConfig)
	tests := []struct {
		name       string
		tool       string
		action     string
		target     string
		want       string
		wantTarget string
	}{
		{"configured registry", "file", "read", "private-tasks/item.md", "task action=current/list/search/get", "private-tasks"},
		{"repo config", "edit", "replace", ".mcp-ai-helper.yaml", "needs_user_action", ".mcp-ai-helper.yaml"},
		{"global config", "file", "read", globalConfig, "current helper config", "config.yaml"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := policy.CheckPath(test.tool, test.action, test.target)
			if err == nil || !strings.Contains(err.Error(), test.want) ||
				!strings.Contains(err.Error(), test.wantTarget) {
				t.Fatalf("CheckPath() error = %v, want %q and %q", err, test.want, test.wantTarget)
			}
		})
	}
	if err := policy.CheckPath("file", "read", "internal/config/schema.go"); err != nil {
		t.Fatalf("ordinary source read rejected: %v", err)
	}
}

func TestPolicyRejectsHighConfidenceCommandPatterns(t *testing.T) {
	repo := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), ".mcp-ai-helper", "config.yaml")
	policy := New(repo, "private-tasks", globalConfig)
	rejected := []struct {
		command string
		want    string
	}{
		{"cat private-tasks/task.md", "task action=current/list/search/get"},
		{"sed -n '1p' .mcp-ai-helper.yaml", "current helper config"},
		{"head -1 " + globalConfig, "config_reload"},
		{"env | sort", "secret_handles"},
		{"printenv OPENAI_API_KEY", "secret_handles"},
		{"cat /proc/self/environ", "secret_handles"},
	}
	for _, test := range rejected {
		err := policy.CheckCommand("command", "run", test.command)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("CheckCommand(%q) error = %v, want %q", test.command, err, test.want)
		}
	}
	for _, command := range []string{
		"printf 'private-tasks/task.md'",
		"env GOFLAGS=-mod=readonly go test ./internal/config",
		"go test ./internal/config",
		"curl -H \"Authorization: Bearer $API_TOKEN\" https://example.invalid",
	} {
		if err := policy.CheckCommand("command", "run", command); err != nil {
			t.Errorf("ordinary command %q rejected: %v", command, err)
		}
	}
}

func TestSearchExcludesProtectedResources(t *testing.T) {
	repo := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), ".mcp-ai-helper", "config.yaml")
	policy := New(repo, "private-tasks", globalConfig)
	got := policy.SearchExcludes(repo)
	for _, want := range []string{
		".mcp-ai-helper.yaml",
		"obsidian-tasks/**",
		"private-tasks/**",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("SearchExcludes() = %v, missing %q", got, want)
		}
	}
	if err := policy.CheckCommand("command", "run", "printf 'MCPAIHelperProject/ActiveTasks.lean'"); err != nil {
		t.Fatalf("mere text mention should be allowed, got %v", err)
	}
}
