package mcp

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

func TestInspectGenericAccessUsesResolvedRepoAndGlobalConfig(t *testing.T) {
	repo := t.TempDir()
	globalConfig := filepath.Join(t.TempDir(), ".mcp-ai-helper", "config.yaml")
	server := &Server{cfg: &config.Config{
		SourcePath: globalConfig,
		TaskRegistry: config.TaskRegistryConfig{
			Backend:  "obsidian",
			Obsidian: config.ObsidianRegistryConfig{Path: "obsidian-tasks"},
		},
	}}
	repoCfg := &config.RepoConfig{
		TaskRegistry: &config.TaskRegistryConfig{
			Backend:  "obsidian",
			Obsidian: config.ObsidianRegistryConfig{Path: "private-tasks"},
		},
	}
	tests := []struct {
		name   string
		tool   string
		action string
		args   map[string]any
		want   string
	}{
		{"configured registry read", "file", "read", map[string]any{
			"repo_path": repo, "path": "private-tasks/task.md",
		}, "task action=current/list/search/get"},
		{"repo config edit", "edit", "replace", map[string]any{
			"repo_path": repo, "path": ".mcp-ai-helper.yaml",
		}, "needs_user_action"},
		{"global config read", "file", "read", map[string]any{
			"repo_path": repo, "path": globalConfig,
		}, "config_read"},
		{"environment dump", "command", "run", map[string]any{
			"repo_path": repo, "command": "env | sort",
		}, "secret_handles"},
		{"workflow registry read", "run", "workflow", map[string]any{
			"repo_path": repo,
			"steps": []any{
				map[string]any{
					"tool": "command",
					"args": map[string]any{"command": "cat private-tasks/task.md"},
				},
			},
		}, "task action=current/list/search/get"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := server.inspectGenericAccess(test.tool, test.action, test.args, repoCfg)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("inspectGenericAccess() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInspectGenericAccessAddsProtectedSearchExclusions(t *testing.T) {
	repo := t.TempDir()
	server := &Server{cfg: &config.Config{
		SourcePath: filepath.Join(t.TempDir(), "config.yaml"),
		TaskRegistry: config.TaskRegistryConfig{
			Backend:  "obsidian",
			Obsidian: config.ObsidianRegistryConfig{Path: "obsidian-tasks"},
		},
	}}
	repoCfg := &config.RepoConfig{
		TaskRegistry: &config.TaskRegistryConfig{
			Backend:  "obsidian",
			Obsidian: config.ObsidianRegistryConfig{Path: "private-tasks"},
		},
	}
	args := map[string]any{
		"repo_path":    repo,
		"pattern":      "token",
		"no_ignore":    true,
		"glob_exclude": []any{"vendor/**"},
	}
	if err := server.inspectGenericAccess("file", "search", args, repoCfg); err != nil {
		t.Fatalf("search rejected: %v", err)
	}
	excludes := stringSlice(args["glob_exclude"])
	for _, want := range []string{
		"vendor/**",
		"private-tasks/**",
		"obsidian-tasks/**",
		".mcp-ai-helper.yaml",
	} {
		if !slices.Contains(excludes, want) {
			t.Errorf("glob_exclude = %v, missing %q", excludes, want)
		}
	}
	sourceArgs := map[string]any{"repo_path": repo, "path": "internal/config/schema.go"}
	if err := server.inspectGenericAccess("file", "read", sourceArgs, repoCfg); err != nil {
		t.Fatalf("ordinary source read rejected: %v", err)
	}
}
