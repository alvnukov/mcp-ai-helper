package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
)

func TestTaskActionCurrentIsBoundedAndSemanticallyUnambiguous(t *testing.T) {
	repoPath := t.TempDir()
	registryPath := filepath.Join(repoPath, "obsidian-tasks")
	if err := os.Mkdir(registryPath, 0o700); err != nil {
		t.Fatal(err)
	}
	backend := newObsidianTaskBackend(registryPath)
	baseTime := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		status := "todo"
		taskType := "bug"
		switch i {
		case 0, 1:
			status = "in_progress"
		case 2:
			status = "blocked"
			taskType = "epic"
		case 19:
			status = "done"
		}
		_, err := backend.Upsert(t.Context(), tasks.AddRequest{
			RepoPath: repoPath, ID: fmt.Sprintf("task-%02d", i), Title: fmt.Sprintf("Task %02d", i),
			Status: status, TaskType: taskType, Priority: "high", ModelLevel: "medium",
			CreatedAt: baseTime.Add(time.Duration(i) * time.Minute),
			UpdatedAt: baseTime.Add(time.Duration(i) * time.Minute), PreserveTimestamps: true,
		})
		if err != nil {
			t.Fatalf("seed task %d: %v", i, err)
		}
	}

	deps := &Server{cfg: &config.Config{TaskRegistry: config.TaskRegistryConfig{
		Backend: "obsidian", Obsidian: config.ObsidianRegistryConfig{Path: registryPath},
	}}}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"repo_path": repoPath, "action": "current"}
	result, err := taskActionCurrent(t.Context(), req, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("task current returned tool error: %s", resultText(t, result))
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items, ok := response["tasks"].([]any)
	if !ok {
		t.Fatalf("tasks type = %T", response["tasks"])
	}
	if len(items) != 12 {
		t.Fatalf("tasks_returned = %d, want 12", len(items))
	}
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["status"] == "blocked" || item["status"] == "done" || item["task_type"] == "epic" {
			t.Fatalf("non-executable task leaked into current: %#v", item)
		}
	}
	if items[0].(map[string]any)["status"] != "in_progress" {
		t.Fatalf("first task is not in_progress: %#v", items[0])
	}
	if response["active_total"] != float64(19) || response["executable_total"] != float64(18) || response["registry_total"] != float64(20) {
		t.Fatalf("ambiguous totals: %#v", response)
	}
	if _, exists := response["tasks_total"]; exists {
		t.Fatalf("ambiguous tasks_total must be absent: %#v", response)
	}
	omitted := response["omitted"].(map[string]any)
	if omitted["due_to_limit"] != float64(6) || omitted["non_executable_by_policy"] != float64(1) {
		t.Fatalf("omission metadata = %#v", omitted)
	}
	freshness := response["freshness"].(map[string]any)
	if freshness["revision"] == "" || freshness["latest_updated_at"] == "" {
		t.Fatalf("freshness metadata = %#v", freshness)
	}
	if strings.Contains(response["next_call"].(string), "task_context") {
		t.Fatalf("next_call names an optional hidden tool: %q", response["next_call"])
	}
	nextAction := response["next_action"].(map[string]any)
	if nextAction["tool"] != "task" {
		t.Fatalf("next_action = %#v", nextAction)
	}
}
