package config

import (
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
	if !strings.Contains(cfg.AssistantGuidance, "one long run_workflow") {
		t.Fatal("loaded config guidance is missing workflow policy")
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
