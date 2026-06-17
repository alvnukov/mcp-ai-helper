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

func TestNewOmitsLegacyPlanningToolsFromStartupSurface(t *testing.T) {
	t.Parallel()

	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	for _, name := range []string{"reasoning_patterns", "task_packet", "plan_task_execution"} {
		if _, ok := srv.ListTools()[name]; ok {
			t.Fatalf("legacy planning tool %s should not be in startup surface", name)
		}
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
	for _, name := range []string{"assistant_guidance", "server_setup_guidance", "tool_manifest", "task_batch_upsert", "task_set_status", "task_graph", "task_context", "web_fetch"} {
		if _, ok := srv.ListTools()[name]; !ok {
			t.Fatalf("%s tool is not registered", name)
		}
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
	if !strings.Contains(cfg.AssistantGuidance, "one self-contained run_workflow") {
		t.Fatal("guidance text does not describe workflow-first policy")
	}
	if !strings.Contains(cfg.AssistantGuidance, "no such unified commit means the task is not done") {
		t.Fatal("guidance text does not describe commit closeout policy")
	}
	for _, want := range []string{"tool_manifest", "command_get", "filter_command_history", "issue_add", "issue_list", "issue_accept"} {
		if !strings.Contains(currentGuidance(cfg), want) {
			t.Fatalf("guidance text does not describe tool discovery for %q", want)
		}
	}
}

func TestStartupSurfaceBudgetStaysCompact(t *testing.T) {
	t.Parallel()

	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	tools := make([]map[string]any, 0, len(srv.ListTools()))
	for name, tool := range srv.ListTools() {
		tools = append(tools, map[string]any{
			"name":         name,
			"description":  tool.Tool.Description,
			"input_schema": tool.Tool.InputSchema,
		})
	}
	resources := make([]map[string]string, 0, len(srv.ListResources()))
	for name, resource := range srv.ListResources() {
		resources = append(resources, map[string]string{
			"name":        name,
			"description": resource.Resource.Description,
			"uri":         resource.Resource.URI,
		})
	}
	prompts := make([]map[string]string, 0, len(srv.ListPrompts()))
	for name, prompt := range srv.ListPrompts() {
		prompts = append(prompts, map[string]string{
			"name":        name,
			"description": prompt.Prompt.Description,
		})
	}
	payload := map[string]any{"tools": tools, "resources": resources, "prompts": prompts}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal startup surface: %v", err)
	}
	if len(data) > 48000 {
		t.Fatalf("startup surface too large: %d bytes", len(data))
	}
	for _, name := range []string{"fetch_url", "task_add", "task_update", "task_tree", "reasoning_patterns", "task_packet", "plan_task_execution"} {
		if _, ok := srv.ListTools()[name]; ok {
			t.Fatalf("duplicate/legacy tool %s should not be registered", name)
		}
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
	for _, name := range []string{"assistant_guidance", "list_models", "collect_command_output", "command_get", "run_workflow", "task_batch_upsert", "web_fetch"} {
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
	for _, field := range []string{"steps", "owned_files", "commit_message", "current_task_id", "task_on_start", "task_on_success", "task_on_failure", "secret_handles"} {
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

func TestToolManifestListsFeedbackAndCommandTools(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	manifest := toolManifest(srv)
	tools, ok := manifest["tools"].([]string)
	if !ok {
		t.Fatalf("tool_manifest tools field = %#v", manifest["tools"])
	}
	joined := " " + strings.Join(tools, " ") + " "
	for _, want := range []string{"command_get", "filter_command_history", "issue_add", "issue_list", "issue_accept"} {
		if !strings.Contains(joined, " "+want+" ") {
			t.Fatalf("tool_manifest missing %q: %#v", want, tools)
		}
	}
}

func TestCommandGetToolIsRegistered(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	if _, ok := srv.ListTools()["command_get"]; !ok {
		t.Fatal("command_get tool is not registered")
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
	for _, field := range []string{"timeout_seconds", "mcp_wait_seconds", "current_task_id", "task_on_start", "task_on_success", "task_on_failure"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("run_pipeline schema does not contain %q: %s", field, schema)
		}
	}
}

func TestTaskBatchUpsertSchemaAdvertisesTaskObjects(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tool, ok := srv.ListTools()["task_batch_upsert"]
	if !ok {
		t.Fatal("task_batch_upsert tool is not registered")
	}

	schemaBytes, err := json.Marshal(tool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var inputSchema map[string]any
	if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	properties, ok := inputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("task_batch_upsert schema properties missing: %s", string(schemaBytes))
	}
	tasksSchema, ok := properties["tasks"].(map[string]any)
	if !ok {
		t.Fatalf("tasks schema missing: %s", string(schemaBytes))
	}
	items, ok := tasksSchema["items"].(map[string]any)
	if !ok {
		t.Fatalf("tasks item schema missing: %s", string(schemaBytes))
	}
	if got := items["type"]; got != "object" {
		t.Fatalf("task_batch_upsert.tasks must advertise object items, got %v: %s", got, string(schemaBytes))
	}
	itemProperties, ok := items["properties"].(map[string]any)
	if !ok || itemProperties["id"] == nil || itemProperties["title"] == nil || itemProperties["acceptance_criteria"] == nil {
		t.Fatalf("tasks item schema does not expose task fields: %s", string(schemaBytes))
	}
}

func TestTaskGraphAndContextToolsAdvertiseUsageContract(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()

	graphTool, ok := tools["task_graph"]
	if !ok {
		t.Fatal("task_graph tool is not registered")
	}
	graphSchema, err := json.Marshal(graphTool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal task_graph schema: %v", err)
	}
	graphHelp := graphTool.Tool.Description + " " + string(graphSchema)
	for _, want := range []string{"task-123", "parent_child", "provenance=explicit", "truncated", "next_call", "task_current"} {
		if !strings.Contains(graphHelp, want) {
			t.Fatalf("task_graph help missing %q: %s", want, graphHelp)
		}
	}

	contextTool, ok := tools["task_context"]
	if !ok {
		t.Fatal("task_context tool is not registered")
	}
	contextSchema, err := json.Marshal(contextTool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal task_context schema: %v", err)
	}
	contextHelp := contextTool.Tool.Description + " " + string(contextSchema)
	for _, want := range []string{"task-123", "task_current", "task_graph", "usage_contract", "truncated", "next_call"} {
		if !strings.Contains(contextHelp, want) {
			t.Fatalf("task_context help missing %q: %s", want, contextHelp)
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
	backend := newLakeTaskBackend(commandRunnerForRepo(repoPath), legacyStoreForTest(t))

	created, err := addIssue(context.Background(), issueAddRequest{
		RepoPath:       repoPath,
		SourceRepoPath: "/tmp/source-repo",
		ID:             "task-998",
		Title:          "feedback routing",
		Body:           "record this for later",
		Priority:       "high",
		Tags:           []string{"routing"},
	}, backend)
	if err != nil {
		t.Fatalf("add issue: %v", err)
	}
	if created.Status != "todo" || created.ProjectionSource != "lean_registry" {
		t.Fatalf("created issue = %+v", created)
	}
	if !strings.Contains(created.Body, "source_repo_path: /tmp/source-repo") {
		t.Fatalf("body does not preserve source repo: %q", created.Body)
	}

	listed, err := listIssues(context.Background(), issueListRequest{RepoPath: repoPath, Query: "feedback routing"}, backend)
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("listed issues = %#v", listed)
	}

	accepted, err := acceptIssue(context.Background(), issueAcceptRequest{RepoPath: repoPath, ID: created.ID}, backend)
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
	if _, ok := tools["task_current"]; !ok {
		t.Fatal("task_current should stay visible when only issues layer is disabled")
	}
}

func TestConfigToolsRegistered(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	tools := srv.ListTools()
	for _, name := range []string{"task_registry_init", "config_schema", "config_read", "config_reload", "config_option_set", "config_option_reset", "feature_list", "feature_get", "feature_enable", "feature_disable", "feature_reset"} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("%s tool is not registered", name)
		}
	}
	if _, ok := tools["config_replace"]; ok {
		t.Fatal("config_replace tool must stay hidden; full config replacement is disabled")
	}
}

func TestTaskRegistryInitDryRunDoesNotWrite(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	result, err := initTaskRegistry(taskRegistryInitRequest{RepoPath: repo, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "would_initialize" || result["next_call"] != "task_current" {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(repo, "obsidian-tasks")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created registry dir, stat err = %v", err)
	}
}

func TestTaskRegistryInitCreatesObsidianRegistryAndRepoConfig(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	result, err := initTaskRegistry(taskRegistryInitRequest{RepoPath: repo})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "ok" || result["next_call"] != "task_current" {
		t.Fatalf("unexpected init result: %#v", result)
	}
	if info, err := os.Stat(filepath.Join(repo, "obsidian-tasks")); err != nil || !info.IsDir() {
		t.Fatalf("registry dir was not created: info=%#v err=%v", info, err)
	}
	data, err := os.ReadFile(filepath.Join(repo, ".mcp-ai-helper.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"task_registry:", "backend: obsidian", "path: obsidian-tasks"} {
		if !strings.Contains(text, want) {
			t.Fatalf("repo config missing %q: %s", want, text)
		}
	}
	backend, err := (&Server{cfg: &config.Config{TaskRegistry: config.TaskRegistryConfig{Backend: "lean"}}}).loadTaskBackendForRepo(repo)
	if err != nil {
		t.Fatalf("load initialized backend: %v", err)
	}
	if _, _, err := backend.ListCurrent(context.Background(), repo); err != nil {
		t.Fatalf("initialized backend ListCurrent: %v", err)
	}
}

func TestTaskRegistryInitRejectsEscapingPath(t *testing.T) {
	t.Parallel()
	_, err := initTaskRegistry(taskRegistryInitRequest{RepoPath: t.TempDir(), Path: "../tasks"})
	if err == nil || !strings.Contains(err.Error(), "escapes repo_path") {
		t.Fatalf("expected escape error, got %v", err)
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

func TestWriteValidatedConfigPreservesRedactedTokenFields(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	_, err := writeValidatedConfig(path, `providers:
  openai:
    type: generic
    base_url: https://api.example.test/v1
    api_key: provider-token-123
secrets:
  GH_TOKEN:
    value: gh-token-123456
    enabled: true
integrations:
  jira:
    url: https://jira.example.test
    api_key: jira-token-123
    api_key_env: JIRA_TOKEN
    enabled: true
  confluence:
    url: https://conf.example.test
    api_key: conf-token-123
    api_key_env: CONF_TOKEN
    enabled: true
`)
	if err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	loaded, err := writeValidatedConfig(path, `providers:
  openai:
    type: generic
    base_url: https://api.example.test/v1
integrations:
  jira:
    url: https://jira.example.test
    enabled: true
  confluence:
    url: https://conf.example.test
    enabled: true
`)
	if err != nil {
		t.Fatalf("replace config: %v", err)
	}
	if got := loaded.Providers["openai"].APIKey; got != "provider-token-123" {
		t.Fatalf("provider api_key = %q, want preserved token", got)
	}
	if got := loaded.Secrets["GH_TOKEN"].Value; got != "gh-token-123456" {
		t.Fatalf("secret value = %q, want preserved token", got)
	}
	if loaded.Integrations.Jira == nil || loaded.Integrations.Jira.APIKey != "jira-token-123" || loaded.Integrations.Jira.APIKeyEnv != "JIRA_TOKEN" {
		t.Fatalf("jira token fields were not preserved: %#v", loaded.Integrations.Jira)
	}
	if loaded.Integrations.Confluence == nil || loaded.Integrations.Confluence.APIKey != "conf-token-123" || loaded.Integrations.Confluence.APIKeyEnv != "CONF_TOKEN" {
		t.Fatalf("confluence token fields were not preserved: %#v", loaded.Integrations.Confluence)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"provider-token-123", "gh-token-123456", "jira-token-123", "conf-token-123"} {
		if !strings.Contains(text, want) {
			t.Fatalf("written config does not preserve %q: %s", want, text)
		}
	}
}

func TestConfigOptionSetAndResetPreservesTokens(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	_, err := writeValidatedConfig(path, `layers:
  tasks:
    enabled: true
providers:
  openai:
    type: generic
    base_url: https://api.example.test/v1
    api_key: provider-token-123
secrets:
  GH_TOKEN:
    value: gh-token-123456
    enabled: true
`)
	if err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	loaded, err := setConfigOption(path, "layers.tasks.enabled", "false")
	if err != nil {
		t.Fatalf("set option: %v", err)
	}
	if loaded.Layers.Tasks.Enabled == nil || *loaded.Layers.Tasks.Enabled {
		t.Fatalf("layers.tasks.enabled = %#v, want false", loaded.Layers.Tasks.Enabled)
	}
	if loaded.Providers["openai"].APIKey != "provider-token-123" || loaded.Secrets["GH_TOKEN"].Value != "gh-token-123456" {
		t.Fatalf("token fields were not preserved: provider=%q secret=%q", loaded.Providers["openai"].APIKey, loaded.Secrets["GH_TOKEN"].Value)
	}
	loaded, err = setConfigOption(path, "command_policy.max_lines", "77")
	if err != nil {
		t.Fatalf("set int option: %v", err)
	}
	if loaded.CommandPolicy.MaxLines != 77 {
		t.Fatalf("max_lines = %d, want 77", loaded.CommandPolicy.MaxLines)
	}
	loaded, err = setConfigOption(path, "web_policy.timeout_seconds", "600")
	if err != nil {
		t.Fatalf("set web timeout option: %v", err)
	}
	if loaded.WebPolicy.TimeoutSeconds != 600 {
		t.Fatalf("web timeout = %d, want 600", loaded.WebPolicy.TimeoutSeconds)
	}
	loaded, err = resetConfigOption(path, "layers.tasks.enabled")
	if err != nil {
		t.Fatalf("reset option: %v", err)
	}
	if loaded.Layers.Tasks.Enabled != nil {
		t.Fatalf("layers.tasks.enabled = %#v, want nil reset", loaded.Layers.Tasks.Enabled)
	}
}

func TestConfigOptionSetRejectsUnsupportedAndInvalidValues(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	_, err := writeValidatedConfig(path, "providers: {}\nmodels: {}\nrouting: {}\n")
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := setConfigOption(path, "providers.openai.api_key", "secret"); err == nil || !strings.Contains(err.Error(), "unsupported config option") {
		t.Fatalf("unsupported path error = %v", err)
	}
	if _, err := setConfigOption(path, "command_policy.max_lines", "zero"); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("invalid int error = %v", err)
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
	if got := currentGuidance(cfg); !strings.Contains(got, "first guidance") || !strings.Contains(got, "command_get") {
		t.Fatalf("guidance = %q", got)
	}
	cfg.AssistantGuidance = "second guidance"
	if got := currentGuidance(cfg); !strings.Contains(got, "second guidance") || !strings.Contains(got, "issue_add") {
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
