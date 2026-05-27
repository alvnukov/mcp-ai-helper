package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExampleConfig(t *testing.T) {
	cfg, err := Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Providers["example"].BaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected example base url: %q", cfg.Providers["example"].BaseURL)
	}
	if _, ok := cfg.Models["routine_summary"]; !ok {
		t.Fatal("routine_summary model is missing")
	}
	if !strings.Contains(cfg.AssistantGuidance, "run_workflow") {
		t.Fatal("assistant guidance is missing")
	}
}

func TestValidateRejectsUnknownProvider(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{},
		Models: map[string]ModelConfig{
			"m": {Provider: "missing", Model: "x"},
		},
	}
	applyDefaults(cfg)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWebPolicyGoogleAPIKeyIsOmittedFromJSON(t *testing.T) {
	cfg := Config{WebPolicy: WebPolicy{GoogleAPIKey: "secret-value", GoogleAPIKeyEnv: "GOOGLE_CSE_API_KEY", GoogleCSEID: "engine-id"}}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "secret-value") {
		t.Fatalf("google api key leaked in JSON: %s", text)
	}
	if !strings.Contains(text, "GOOGLE_CSE_API_KEY") || !strings.Contains(text, "engine-id") {
		t.Fatalf("google public config missing from JSON: %s", text)
	}
}

func TestTaskRegistryBackendDefaultsToLean(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)
	if cfg.TaskRegistry.Backend != "lean" {
		t.Fatalf("backend = %q, want lean", cfg.TaskRegistry.Backend)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestTaskRegistryBackendRejectsInvalidValue(t *testing.T) {
	cfg := &Config{TaskRegistry: TaskRegistryConfig{Backend: "magic"}}
	applyDefaults(cfg)
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "unsupported task_registry.backend") {
		t.Fatalf("expected unsupported backend error, got: %v", err)
	}
}

func TestTaskRegistryBackendRequiresReadableObsidianPath(t *testing.T) {
	cfg := &Config{TaskRegistry: TaskRegistryConfig{Backend: "obsidian"}}
	applyDefaults(cfg)
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "task_registry.obsidian.path is required") {
		t.Fatalf("expected missing path error, got: %v", err)
	}

	cfg.TaskRegistry.Obsidian.Path = filepath.Join(t.TempDir(), "missing")
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "task_registry.obsidian.path not readable") {
		t.Fatalf("expected unreadable path error, got: %v", err)
	}
}

func TestTaskRegistryBackendAcceptsObsidianDirectory(t *testing.T) {
	cfg := &Config{TaskRegistry: TaskRegistryConfig{Backend: "obsidian", Obsidian: ObsidianRegistryConfig{Path: t.TempDir()}}}
	applyDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestLoadCreatesDefaultConfigInHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(home, defaultConfigDir, "config.yaml")
	if cfg.SourcePath != wantPath {
		t.Fatalf("source path = %q, want %q", cfg.SourcePath, wantPath)
	}
	data, err := os.ReadFile(wantPath) // #nosec G304 -- path is under the test temp dir.
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "assistant_guidance: |") {
		t.Fatal("generated config does not include assistant_guidance")
	}
	if !strings.Contains(text, "layers:") {
		t.Fatal("generated config does not include layers")
	}
	if len(cfg.Providers) != 0 || len(cfg.Models) != 0 {
		t.Fatal("generated config should not enable provider or model presets")
	}
	if !strings.Contains(text, "issues:\n    enabled: false") {
		t.Fatal("generated config should disable issues layer by default")
	}
	if !strings.Contains(text, "reasoning_patterns:\n    enabled: true") {
		t.Fatal("generated config should expose reasoning_patterns layer")
	}
	if !strings.Contains(text, "default_timeout_seconds: 300") {
		t.Fatal("generated config should default command timeout to 300 seconds")
	}
	if cfg.CommandPolicy.DefaultTimeoutSeconds != 300 {
		t.Fatalf("default command timeout = %d, want 300", cfg.CommandPolicy.DefaultTimeoutSeconds)
	}
	if !strings.Contains(cfg.AssistantGuidance, "one long run_workflow") {
		t.Fatal("loaded config guidance is missing workflow policy")
	}
	if !strings.Contains(cfg.AssistantGuidance, "no such unified commit means the task is not done") {
		t.Fatal("loaded config guidance is missing atomic commit closeout policy")
	}
	if !strings.Contains(cfg.AssistantGuidance, "post-hoc status commit") {
		t.Fatal("loaded config guidance is missing post-hoc status commit prohibition")
	}
	if !strings.Contains(cfg.AssistantGuidance, "MCP-ONLY") || !strings.Contains(cfg.AssistantGuidance, "Do not use direct filesystem, shell, git") {
		t.Fatal("loaded config guidance is missing MCP-only repo tool policy")
	}
	if !strings.Contains(cfg.AssistantGuidance, "web_search for compact hits") || !strings.Contains(cfg.AssistantGuidance, "fetched_doc_find or fetched_doc_read") {
		t.Fatal("loaded config guidance is missing bounded web workflow")
	}
}

func TestLayerEnabledDefaultsAndOverrides(t *testing.T) {
	cfg := &Config{}
	if !cfg.LayerEnabled("tasks") {
		t.Fatal("tasks layer should default to enabled")
	}
	disabled := false
	cfg.Layers.Tasks.Enabled = &disabled
	if cfg.LayerEnabled("tasks") {
		t.Fatal("tasks layer should be disabled")
	}
	if !cfg.LayerEnabled("reasoning_patterns") {
		t.Fatal("reasoning_patterns layer should default to enabled")
	}
	cfg.Layers.ReasoningPatterns.Enabled = &disabled
	if cfg.LayerEnabled("reasoning_patterns") {
		t.Fatal("reasoning_patterns layer should be disabled")
	}
}

func TestApplyDefaultsDisablesLogsLayer(t *testing.T) {
	disabled := false
	cfg := &Config{}
	cfg.Layers.Logs.Enabled = &disabled
	applyDefaults(cfg)
	if cfg.CommandPolicy.LogEnabled == nil || *cfg.CommandPolicy.LogEnabled {
		t.Fatal("logs should be disabled by layer policy")
	}
}

func TestIssuesLayerCanBeDisabled(t *testing.T) {
	disabled := false
	cfg := &Config{}
	if !cfg.LayerEnabled("issues") {
		t.Fatal("issues layer should default to enabled")
	}
	cfg.Layers.Issues.Enabled = &disabled
	if cfg.LayerEnabled("issues") {
		t.Fatal("issues layer should be disabled")
	}
}

func TestLoadCreatesExplicitMissingConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp-ai-helper", "config.yaml")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourcePath != path {
		t.Fatalf("source path = %q, want %q", cfg.SourcePath, path)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is under the test temp dir.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "assistant_guidance: |") {
		t.Fatal("generated explicit config does not include assistant guidance")
	}
}

func TestDefaultConfigPathUsesHomeHelperDir(t *testing.T) {
	path := DefaultConfigPath()
	if !strings.HasSuffix(path, filepath.Join(".mcp-ai-helper", "config.yaml")) {
		t.Fatalf("default config path = %q", path)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("default config path should be absolute: %q", path)
	}
}

func TestTaskRegistryBackendAllowsRelativeObsidianPathWithoutRepo(t *testing.T) {
	cfg := &Config{TaskRegistry: TaskRegistryConfig{Backend: "obsidian", Obsidian: ObsidianRegistryConfig{Path: "obsidian-tasks"}}}
	applyDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestLoadRepoConfigAndMergePolicy(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, repoConfigFile), []byte(`permissions:
  tools:
    deny: [collect_command_output]
command_policy:
  allowed_cwds: [safe]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, "safe"), 0o700); err != nil {
		t.Fatal(err)
	}
	repoCfg, err := LoadRepoConfig(repo)
	if err != nil {
		t.Fatal(err)
	}
	if repoCfg == nil || !repoCfg.ToolDenied("collect_command_output") {
		t.Fatalf("repo permissions were not loaded: %#v", repoCfg)
	}
	base := &Config{CommandPolicy: CommandPolicy{AllowedCWDs: []string{"."}, DefaultTimeoutSeconds: 20}}
	merged, err := MergeRepoConfig(base, repoCfg, repo)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(repo, "safe")
	if len(merged.CommandPolicy.AllowedCWDs) != 1 || merged.CommandPolicy.AllowedCWDs[0] != want {
		t.Fatalf("allowed_cwds = %#v, want %q", merged.CommandPolicy.AllowedCWDs, want)
	}
	if base.CommandPolicy.AllowedCWDs[0] != "." {
		t.Fatalf("base config mutated: %#v", base.CommandPolicy.AllowedCWDs)
	}
}

func TestMergeRepoConfigResolvesGlobalRelativeTaskRegistry(t *testing.T) {
	repo := t.TempDir()
	notesDir := filepath.Join(repo, "obsidian-tasks")
	if err := os.Mkdir(notesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	base := &Config{
		Providers:    map[string]ProviderConfig{},
		Models:       map[string]ModelConfig{},
		Routing:      map[string]string{},
		TaskRegistry: TaskRegistryConfig{Backend: "obsidian", Obsidian: ObsidianRegistryConfig{Path: "obsidian-tasks"}},
	}
	merged, err := MergeRepoConfig(base, nil, repo)
	if err != nil {
		t.Fatal(err)
	}
	if merged.TaskRegistry.Obsidian.Path != "obsidian-tasks" {
		t.Fatalf("obsidian path = %q, want relative global path preserved", merged.TaskRegistry.Obsidian.Path)
	}
	if merged.TaskRegistry.Obsidian.ResolvedPath != notesDir {
		t.Fatalf("resolved obsidian path = %q, want %q", merged.TaskRegistry.Obsidian.ResolvedPath, notesDir)
	}
}

func TestMergeRepoConfigOverlaysTaskRegistry(t *testing.T) {
	repo := t.TempDir()
	notesDir := filepath.Join(repo, "notes")
	if err := os.Mkdir(notesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	repoCfg := &RepoConfig{TaskRegistry: &TaskRegistryConfig{Backend: "obsidian", Obsidian: ObsidianRegistryConfig{Path: "notes"}}}
	merged, err := MergeRepoConfig(&Config{Providers: map[string]ProviderConfig{}, Models: map[string]ModelConfig{}, Routing: map[string]string{}}, repoCfg, repo)
	if err != nil {
		t.Fatal(err)
	}
	if merged.TaskRegistry.Backend != "obsidian" || merged.TaskRegistry.Obsidian.Path != "notes" {
		t.Fatalf("task registry path = %q, want repo-relative path", merged.TaskRegistry.Obsidian.Path)
	}
	if merged.TaskRegistry.Obsidian.ResolvedPath != notesDir {
		t.Fatalf("resolved obsidian path = %q, want %q", merged.TaskRegistry.Obsidian.ResolvedPath, notesDir)
	}
}

func TestMergeRepoConfigRejectsInvalidTaskRegistry(t *testing.T) {
	repoCfg := &RepoConfig{TaskRegistry: &TaskRegistryConfig{Backend: "obsidian"}}
	_, err := MergeRepoConfig(&Config{Providers: map[string]ProviderConfig{}, Models: map[string]ModelConfig{}, Routing: map[string]string{}}, repoCfg, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "task_registry.obsidian.path is required") {
		t.Fatalf("expected missing obsidian path error, got: %v", err)
	}
}

func TestMergeRepoConfigRejectsEscapingAllowedCWD(t *testing.T) {
	repo := t.TempDir()
	repoCfg := &RepoConfig{CommandPolicy: &struct {
		AllowedCWDs []string `yaml:"allowed_cwds" json:"allowed_cwds"`
	}{AllowedCWDs: []string{".."}}}
	_, err := MergeRepoConfig(&Config{}, repoCfg, repo)
	if err == nil || !strings.Contains(err.Error(), "escapes repo_path") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestSchemaDocumentsModelDrivenConfig(t *testing.T) {
	schema := Schema()
	fields, ok := schema["fields"].([]FieldDoc)
	if !ok || len(fields) == 0 {
		t.Fatalf("schema fields missing: %#v", schema["fields"])
	}
	want := map[string]bool{
		"assistant_guidance":                            false,
		"providers.<id>.api_key_env":                    false,
		"models.<id>.system_prompt":                     false,
		"command_policy.log_dir":                        false,
		"pipeline_policy.require_evidence_for_analysis": false,
		"secrets.<handle>.value":                        false,
		"secrets.<handle>.enabled":                      false,
		"features.overrides.<id>.enabled":               false,
	}
	for _, field := range fields {
		if _, ok := want[field.Path]; ok && field.Description != "" {
			want[field.Path] = true
		}
	}
	for path, seen := range want {
		if !seen {
			t.Fatalf("schema does not document %s", path)
		}
	}
}

func TestGuidanceDocumentsLeanTaskRegistry(t *testing.T) {
	guidance := DefaultAssistantGuidance()
	for _, want := range []string{
		"Lean registry/exporter",
		"not fallback storage",
		"Do not implement production task state by parsing or regex-mutating Lean registry source in Go",
	} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance does not document Lean task registry mode %q: %q", want, guidance)
		}
	}
	setup := SetupGuidance("")
	for _, want := range []string{
		"Lean registry/exporter",
		"not fallback storage",
		"Do not implement production task state by parsing or regex-mutating Lean registry source in Go",
	} {
		if !strings.Contains(setup["tasks"], want) {
			t.Fatalf("setup guidance does not document task storage mode %q: %#v", want, setup)
		}
	}
	for _, want := range []string{
		"repair_lean_task_registry_exporter",
		"MCPAIHelperProject/TaskRegistryExport.lean",
		"task_registry_export",
		"lake exe task_registry_export --list-active",
	} {
		if !strings.Contains(setup["lean_task_registry_repair"], want) {
			t.Fatalf("setup guidance does not document repair step %q: %#v", want, setup)
		}
	}
}

func TestGuidanceDocumentsStrictRepoTaskWorkflow(t *testing.T) {
	guidance := DefaultAssistantGuidance()
	for _, want := range []string{
		"first gather complete minimal sufficient context",
		"After context gathering, stop and state the decision",
		"selected tasks, why they fit the current model",
		"task_graph for overview/dependencies",
		"task_context for selected-task execution context",
		"task_packet for readiness/owned_files/gates",
		"one self-contained run_pipeline or run_workflow",
		"Never set a task to done until its acceptance criteria, relevant gate, and required owned-files commit are actually closed",
		"evidence-only analysis success",
		"If a workflow fails or times out, do not close the task",
		"complete authoritative task set",
		"Never edit tasks by modifying task registry/source/projection files directly",
		"use task_upsert, task_batch_upsert, task_set_status, task_delete",
		"stop with a surface mismatch/blocker",
		"configurable in the server config through assistant_guidance",
	} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q in %q", want, guidance)
		}
	}
	setup := SetupGuidance("")
	for _, want := range []string{"task_graph for dependency overview", "task_context for selected-task execution context", "task_packet for readiness/owned_files/gates", "batch task updates", "close_missing only intentionally", "because it can close omitted active tasks", "acceptance criteria, gates, and required owned-files commit pass", "no commit means the task is not done", "evidence-only analysis"} {
		if !strings.Contains(setup["tasks"], want) {
			t.Fatalf("setup guidance missing %q in %#v", want, setup)
		}
	}
}
