package mcp

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Eleven git tools, seven command tools and three run tools were merged into
// action-dispatch tools; the names they used to have are gone. Error messages
// and tool descriptions that still spell those names send a caller after a tool
// the server will not answer to, and unlike a stale comment there is nothing to
// notice — the model simply gets "unknown tool" and has to guess again.
//
// This walks the production source and fails on any of the retired names. The
// setup skills already get this treatment in internal/setup; this is the same
// check for the strings the server itself emits.
var retiredToolNames = []string{
	// file / edit
	"read_file", "read_files", "list_dir", "search_files", "snapshot_file", "write_file",
	// command
	"run_command", "cleanup_command_history", "abort_command", "list_command_history",
	"command_get", "filter_command_history", "project_health", "collect_command_output",
	// git — the tool names only; the identically named workflow steps are still real
	"git_status_tool", "git_diff_tool", "git_log_tool", "git_blame_tool",
	// task
	"task_current", "task_get", "task_list", "task_search", "task_upsert",
	"task_set_status", "task_delete",
	// run
	"run_pipeline", "run_workflow", "workflow_schema",
}

func TestNoProductionStringNamesARetiredTool(t *testing.T) {
	// Word boundaries, so task_batch_upsert does not trip on task_upsert and
	// task_registry_init does not trip on anything.
	patterns := make(map[string]*regexp.Regexp, len(retiredToolNames))
	for _, name := range retiredToolNames {
		patterns[name] = regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	}

	root := filepath.Join("..", "..", "internal")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path) // #nosec G304 -- walking the repo's own source tree.
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(source), "\n") {
			for name, pattern := range patterns {
				if pattern.MatchString(line) {
					t.Errorf("%s names the retired tool %q: %s", path, name, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk source: %v", err)
	}
}
