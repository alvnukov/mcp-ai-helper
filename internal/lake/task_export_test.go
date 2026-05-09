package lake

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestTaskRegistryExporterGetTaskThroughLakeExe(t *testing.T) {
	result := runTaskRegistryExporter(t, "--get", "task-034")
	if result.ExitCode != 0 {
		t.Fatalf("expected exporter success, got %+v", result)
	}

	var task map[string]any
	decodeJSONOutput(t, result, &task)
	if task["id"] != "task-034" {
		t.Fatalf("unexpected task id: %#v", task["id"])
	}
	if task["status"] != "done" {
		t.Fatalf("unexpected task status: %#v", task["status"])
	}
	if _, ok := task["tags"].([]any); !ok {
		t.Fatalf("tags field missing or not an array: %#v", task["tags"])
	}
}

func TestTaskRegistryExporterListActiveThroughLakeExe(t *testing.T) {
	result := runTaskRegistryExporter(t, "--list-active")
	if result.ExitCode != 0 {
		t.Fatalf("expected exporter success, got %+v", result)
	}

	var payload struct {
		Tasks []map[string]any `json:"tasks"`
	}
	decodeJSONOutput(t, result, &payload)
	if len(payload.Tasks) == 0 {
		t.Fatalf("expected migrated active tasks, got none")
	}

	var migrated map[string]any
	for _, task := range payload.Tasks {
		if task["id"] == "task-006" {
			migrated = task
			break
		}
	}
	if migrated == nil {
		t.Fatalf("task-006 missing from active Lean export: %#v", payload.Tasks)
	}
	if migrated["status"] != "todo" || migrated["title"] == "" || migrated["body"] == "" {
		t.Fatalf("task-006 core fields were not preserved: %#v", migrated)
	}
	if tags, ok := migrated["tags"].([]any); !ok || len(tags) == 0 {
		t.Fatalf("task-006 tags were not preserved: %#v", migrated["tags"])
	}
}

func TestTaskRegistryExporterGetMissingTaskFails(t *testing.T) {
	result := runTaskRegistryExporter(t, "--get", "missing-task")
	if result.ExitCode == 0 {
		t.Fatalf("expected missing task failure, got %+v", result)
	}
	if !strings.Contains(strings.Join(result.Diagnostics, "\n"), "task not found") {
		t.Fatalf("expected not-found diagnostic, got %#v", result.Diagnostics)
	}
}

func runTaskRegistryExporter(t *testing.T, args ...string) CommandResult {
	t.Helper()
	repoRoot := filepath.Clean("../..")
	runner := CommandRunner{
		Commands: command.NewRunner(config.CommandPolicy{
			AllowedCWDs:           []string{repoRoot},
			DefaultTimeoutSeconds: 20,
			MaxOutputBytes:        20000,
			MaxLines:              80,
		}),
		TimeoutSeconds: 20,
	}
	result, err := RunExe(context.Background(), repoRoot, "task_registry_export", args, runner)
	if err != nil {
		t.Fatalf("RunExe returned error: %v", err)
	}
	return result
}

func decodeJSONOutput(t *testing.T, result CommandResult, target any) {
	t.Helper()
	output := strings.Join(result.Output, "\n")
	if output == "" {
		t.Fatalf("exporter produced no JSON output: %+v", result)
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		t.Fatalf("decode exporter JSON %q: %v", output, err)
	}
}
