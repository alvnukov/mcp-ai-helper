package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestNewCanDisableReasoningPatterns(t *testing.T) {
	disabled := false
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	cfg.Layers.ReasoningPatterns.Enabled = &disabled
	srv := New(cfg)
	if _, ok := srv.ListTools()["reasoning_patterns"]; ok {
		t.Fatal("reasoning_patterns tool should not be registered when disabled")
	}
	if _, ok := srv.ListTools()["task_packet"]; !ok {
		t.Fatal("task_packet should remain registered when reasoning patterns are disabled")
	}
}

func TestCurrentTasksReturnsOnlyActiveStatuses(t *testing.T) {
	now := time.Now().UTC()
	list := []tasks.Task{
		{ID: "todo", Status: "todo", CreatedAt: now, UpdatedAt: now},
		{ID: "active", Status: "in_progress", CreatedAt: now, UpdatedAt: now},
		{ID: "blocked", Status: "blocked", CreatedAt: now, UpdatedAt: now},
		{ID: "done", Status: "done", CreatedAt: now, UpdatedAt: now},
	}
	current := currentTasks(list)
	if len(current) != 3 {
		t.Fatalf("current task count = %d, want 3", len(current))
	}
	if current[0].ID != "todo" || current[1].ID != "active" || current[2].ID != "blocked" {
		t.Fatalf("current tasks = %#v", current)
	}
}

func TestNewExposesAssistantGuidance(t *testing.T) {
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	if _, ok := srv.ListTools()["assistant_guidance"]; !ok {
		t.Fatal("assistant_guidance tool is not registered")
	}
	if _, ok := srv.ListTools()["server_setup_guidance"]; !ok {
		t.Fatal("server_setup_guidance tool is not registered")
	}
	if _, ok := srv.ListTools()["task_batch_upsert"]; !ok {
		t.Fatal("task_batch_upsert tool is not registered")
	}
	if _, ok := srv.ListTools()["task_update"]; !ok {
		t.Fatal("task_update tool is not registered")
	}
	if _, ok := srv.ListTools()["task_set_status"]; !ok {
		t.Fatal("task_set_status tool is not registered")
	}
	if _, ok := srv.ListTools()["plan_task_execution"]; !ok {
		t.Fatal("plan_task_execution tool is not registered")
	}
	if _, ok := srv.ListTools()["task_packet"]; !ok {
		t.Fatal("task_packet tool is not registered")
	}
	if _, ok := srv.ListTools()["reasoning_patterns"]; !ok {
		t.Fatal("reasoning_patterns tool is not registered")
	}
	resource, ok := srv.ListResources()[guidanceURI]
	if !ok {
		t.Fatal("guidance resource is not registered")
	}
	if !strings.Contains(resource.Resource.Description, "workflow-first") {
		t.Fatalf("guidance resource description = %q", resource.Resource.Description)
	}
	prompt, ok := srv.ListPrompts()["mcp-ai-helper-guidance"]
	if !ok {
		t.Fatal("guidance prompt is not registered")
	}
	if !strings.Contains(prompt.Prompt.Description, "calling LLM") {
		t.Fatalf("guidance prompt description = %q", prompt.Prompt.Description)
	}
	if !strings.Contains(cfg.AssistantGuidance, "Prefer one long run_workflow or run_pipeline call") {
		t.Fatal("guidance text does not describe workflow-first policy")
	}
	if !strings.Contains(cfg.AssistantGuidance, "no commit means the task is not done") {
		t.Fatal("guidance text does not describe commit closeout policy")
	}
}

func TestNewHidesDisabledLayers(t *testing.T) {
	disabled := false
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	cfg.Layers.Tasks.Enabled = &disabled
	cfg.Layers.Guidance.Enabled = &disabled
	cfg.Layers.Models.Enabled = &disabled
	cfg.Layers.Commands.Enabled = &disabled
	cfg.Layers.Workflows.Enabled = &disabled
	srv := New(cfg)
	tools := srv.ListTools()
	for _, name := range []string{"assistant_guidance", "list_models", "collect_command_output", "run_workflow", "task_batch_upsert"} {
		if _, ok := tools[name]; ok {
			t.Fatalf("tool %s should be hidden", name)
		}
	}
	if _, ok := srv.ListResources()[guidanceURI]; ok {
		t.Fatal("guidance resource should be hidden")
	}
}

func TestRunWorkflowSchemaIncludesWorkflowFields(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()
	tool, ok := tools["run_workflow"]
	if !ok {
		t.Fatal("run_workflow tool is not registered")
	}

	schemaBytes, err := json.Marshal(tool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	schema := string(schemaBytes)
	for _, field := range []string{"steps", "owned_files", "commit_message", "current_task_id", "task_on_start", "task_on_success", "task_on_failure"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("run_workflow schema does not contain %q: %s", field, schema)
		}
	}

	var inputSchema map[string]any
	if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	properties, ok := inputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("run_workflow schema properties missing: %s", schema)
	}
	steps, ok := properties["steps"].(map[string]any)
	if !ok {
		t.Fatalf("run_workflow steps schema missing: %s", schema)
	}
	items, ok := steps["items"].(map[string]any)
	if !ok {
		t.Fatalf("run_workflow steps item schema missing: %s", schema)
	}
	if got := items["type"]; got != "object" {
		t.Fatalf("run_workflow steps must advertise object items, got %v: %s", got, schema)
	}
}

func TestRunPipelineSchemaIncludesTaskStatusFields(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()
	tool, ok := tools["run_pipeline"]
	if !ok {
		t.Fatal("run_pipeline tool is not registered")
	}

	schemaBytes, err := json.Marshal(tool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	schema := string(schemaBytes)
	for _, field := range []string{"timeout_seconds", "current_task_id", "task_on_start", "task_on_success", "task_on_failure"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("run_pipeline schema does not contain %q: %s", field, schema)
		}
	}
}

func TestIssueToolsRegistered(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()
	for _, name := range []string{"issue_add", "issue_list", "issue_accept"} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("%s tool is not registered", name)
		}
	}
}

func TestIssueLifecycleUsesLeanRegistry(t *testing.T) {
	repoPath := copyLeanRepoFixture(t)
	commands := commandRunnerForRepo(repoPath)
	store := legacyStoreForTest(t)

	created, err := addIssue(context.Background(), issueAddRequest{
		RepoPath:       repoPath,
		SourceRepoPath: "/tmp/source-repo",
		ID:             "task-998",
		Title:          "feedback routing",
		Body:           "record this for later",
		Priority:       "high",
		Tags:           []string{"routing"},
	}, commands, store)
	if err != nil {
		t.Fatalf("add issue: %v", err)
	}
	if created.Status != "todo" || created.ProjectionSource != "lean_registry" {
		t.Fatalf("created issue = %+v", created)
	}
	if !strings.Contains(created.Body, "source_repo_path: /tmp/source-repo") {
		t.Fatalf("body does not preserve source repo: %q", created.Body)
	}

	listed, err := listIssues(context.Background(), issueListRequest{RepoPath: repoPath, Query: "feedback routing"}, commands, store)
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed issues = %#v", listed)
	}

	accepted, err := acceptIssue(context.Background(), issueAcceptRequest{RepoPath: repoPath, ID: created.ID}, commands, store)
	if err != nil {
		t.Fatalf("accept issue: %v", err)
	}
	if accepted.Status != "in_progress" || accepted.ProjectionSource != "lean_registry" {
		t.Fatalf("accepted issue = %+v", accepted)
	}
}

func TestNewHidesDisabledIssuesLayer(t *testing.T) {
	t.Parallel()

	disabled := false
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	cfg.Layers.Issues.Enabled = &disabled
	srv := New(cfg)
	tools := srv.ListTools()
	for _, name := range []string{"issue_add", "issue_list", "issue_accept"} {
		if _, ok := tools[name]; ok {
			t.Fatalf("tool %s should be hidden", name)
		}
	}
	if _, ok := tools["task_add"]; !ok {
		t.Fatal("task_add should stay visible when only issues layer is disabled")
	}
}

func TestConfigToolsRegistered(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()
	for _, name := range []string{"config_schema", "config_read", "config_replace", "config_reload"} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("%s tool is not registered", name)
		}
	}
}

func TestWriteValidatedConfigRejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	_, err := writeValidatedConfig(path, "providers:\n  bad:\n    type: nope\n")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid config should not be written, stat err = %v", statErr)
	}
}

func TestWriteValidatedConfigWritesValidConfig(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	loaded, err := writeValidatedConfig(path, "assistant_guidance: test guidance\nproviders: {}\nmodels: {}\nrouting: {}\n")
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	if loaded.SourcePath != path {
		t.Fatalf("source path = %q, want %q", loaded.SourcePath, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test guidance") {
		t.Fatalf("written config missing guidance: %s", string(data))
	}
}

func TestLanguageToolsRegistered(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()
	for _, name := range []string{"language_profiles", "language_detect"} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("%s tool is not registered", name)
		}
	}
}

func TestCurrentGuidanceUsesUpdatedConfig(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AssistantGuidance: "first guidance"}
	if got := currentGuidance(cfg); got != "first guidance" {
		t.Fatalf("guidance = %q", got)
	}
	cfg.AssistantGuidance = "second guidance"
	if got := currentGuidance(cfg); got != "second guidance" {
		t.Fatalf("guidance after update = %q", got)
	}
}

func TestHealthToolRegistered(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tool, ok := srv.ListTools()["health"]
	if !ok {
		t.Fatal("health tool is not registered")
	}
	if !stringsContains(tool.Tool.Description, "health") {
		t.Fatalf("health tool description = %q", tool.Tool.Description)
	}
}

func TestLoadDepsReturnsConsistentSnapshot(t *testing.T) {
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	deps := &Server{cfg: cfg}
	c1, ch1, cm1, p1, ts1 := deps.loadDeps()
	c2, ch2, cm2, p2, ts2 := deps.loadDeps()
	if c1 != c2 {
		t.Fatal("config pointers should be same")
	}
	if ch1 != ch2 {
		t.Fatal("chat pointers should be same")
	}
	if cm1 != cm2 {
		t.Fatal("command pointers should be same")
	}
	if p1 != p2 {
		t.Fatal("pipeline pointers should be same")
	}
	if ts1 != ts2 {
		t.Fatal("taskStore pointers should be same")
	}
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
