// Package config loads and validates mcp-ai-helper YAML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigDir = ".mcp-ai-helper"
)

const defaultAssistantGuidance = `mcp-ai-helper operating guidance:

1. Prefer one long run_workflow or run_pipeline call when intermediate results are not needed by the calling model.
2. Put command execution, output filters, deterministic conditions, guarded edits, checks, and commit into that workflow instead of making many low-level calls.
3. Use low-level tools only for bootstrapping, schema discovery, or when the returned result must change the caller's next decision.
4. Always pass repo_path. Treat cwd and file paths as repo-relative unless a tool explicitly says otherwise.
5. Never use broad staging or destructive git operations. Commit only explicit owned files after relevant checks pass.
6. Keep output compact and evidence-linked. Re-filter retained command history instead of rerunning commands or returning raw logs.
7. For any repo task, first gather complete minimal sufficient context with task_current, relevant read_file/snapshot_file calls, and narrow run_pipeline/collect_command_output probes; do not build an execution pipeline before understanding the contract, architecture, integration points, test patterns, and existing changes.
8. After context gathering, stop and state the decision: selected tasks, why they fit the current model, exact edits, owned_files, forbidden files, acceptance criteria to close, the minimal gate proving closure, and the end-to-end closeout path.
9. Only after that, build one self-contained run_pipeline or run_workflow for the remaining implementation path: minimal edits, formatting, relevant checks, final task status transition, and explicit owned-files commit only if everything succeeds.
10. For repo tasks with file changes, finalization must be atomic inside one run_workflow: after successful acceptance gates, perform the task-facing transition to the final status, then run git_commit_owned in the same workflow with owned_files covering both changed work files and the canonical task registry mutation. Do not commit code first and then record task status in a separate step or separate commit.
11. Never set a task to done until its acceptance criteria, relevant gate, and required owned-files commit are actually closed; task status transition must also be closed in the same workflow. For a repo task with file changes, no such unified commit means the task is not done; no commit means the task is not done. A partial green test, timeout, evidence-only analysis success, skipped check, failed commit, post-hoc status commit, or unverified MCP/tool-facing path is not enough.
12. If a workflow fails or times out, do not close the task. First inspect actual state, separate confirmed facts from assumptions, and choose the next minimal step with a new hypothesis.
13. For migrated Lean/Lake repos, task_current, task_list, task_get, and task mutations require the Lean registry/exporter; legacy tasks/*.lean JSON-comment files are not fallback storage.
14. Do not implement production task state by parsing or regex-mutating Lean registry source in Go; use Lean-owned lake serve/exporter/task tools and fail closed when that surface cannot express the mutation.
15. For project memory, read task_current or task_list before work, then synchronize the backlog with one task_batch_upsert call whenever you already know the complete authoritative task set; use close_missing only intentionally.
16. Never edit tasks by modifying task registry/source/projection files directly. Do not change MCPAIHelperProject/ActiveTasks.lean, tasks/*.lean, task JSON comments, or legacy task files to update task title/body/status/priority/tags/criteria/verification; use task_upsert, task_batch_upsert, task_set_status, task_delete, or another explicit task-facing helper tool only.
17. If task tools cannot express the needed task mutation, stop with a surface mismatch/blocker; do not bypass the helper with file edits, scripts, guarded_replace, shell, or direct git operations.
18. Use task statuses consistently: todo for planned work, in_progress for the current owner, blocked when external input is needed, and done for completed or intentionally closed work.
19. Prefer task_update or task_set_status for one targeted change; avoid long sequences of task_add calls when batch_upsert can express the desired state.
20. If a workflow cannot express the needed operation, improve the workflow DSL instead of normalizing repeated manual tool calls.

This guidance is configurable in the server config through assistant_guidance; generated default config and configs/config.example.yaml show the same field.`

// Config is the complete server configuration loaded from YAML.
type Config struct {
	SourcePath        string                    `yaml:"-" json:"config_path"`
	AssistantGuidance string                    `yaml:"assistant_guidance" json:"assistant_guidance"`
	Layers            LayerPolicy               `yaml:"layers" json:"layers"`
	Providers         map[string]ProviderConfig `yaml:"providers" json:"providers"`
	Models            map[string]ModelConfig    `yaml:"models" json:"models"`
	Routing           map[string]string         `yaml:"routing" json:"routing"`
	CommandPolicy     CommandPolicy             `yaml:"command_policy" json:"command_policy"`
	PipelinePolicy    PipelinePolicy            `yaml:"pipeline_policy" json:"pipeline_policy"`
}

// LayerPolicy controls optional server capability layers.
type LayerPolicy struct {
	Logs              LayerConfig `yaml:"logs" json:"logs"`
	Tasks             LayerConfig `yaml:"tasks" json:"tasks"`
	Issues            LayerConfig `yaml:"issues" json:"issues"`
	Guidance          LayerConfig `yaml:"guidance" json:"guidance"`
	Models            LayerConfig `yaml:"models" json:"models"`
	Commands          LayerConfig `yaml:"commands" json:"commands"`
	Workflows         LayerConfig `yaml:"workflows" json:"workflows"`
	ReasoningPatterns LayerConfig `yaml:"reasoning_patterns" json:"reasoning_patterns"`
}

// LayerConfig controls one optional server layer.
type LayerConfig struct {
	Enabled *bool `yaml:"enabled" json:"enabled"`
}

// ProviderConfig describes one OpenAI-compatible model provider.
type ProviderConfig struct {
	Type           string `yaml:"type" json:"type"`
	BaseURL        string `yaml:"base_url" json:"base_url"`
	CompletionsURL string `yaml:"completions_url" json:"completions_url"`
	APIKey         string `yaml:"api_key" json:"-"`
	APIKeyEnv      string `yaml:"api_key_env" json:"api_key_env"`
	AppName        string `yaml:"app_name" json:"app_name"`
	TimeoutSeconds int    `yaml:"timeout_seconds" json:"timeout_seconds"`
	MaxRetries     int    `yaml:"max_retries" json:"max_retries"`
}

// ModelConfig describes one named model profile and its prompt policy.
type ModelConfig struct {
	Provider         string         `yaml:"provider" json:"provider"`
	Model            string         `yaml:"model" json:"model"`
	Tier             string         `yaml:"tier" json:"tier"`
	Roles            []string       `yaml:"roles" json:"roles"`
	Purpose          string         `yaml:"purpose" json:"purpose"`
	SystemPrompt     string         `yaml:"system_prompt" json:"system_prompt"`
	SystemPromptFile string         `yaml:"system_prompt_file" json:"system_prompt_file"`
	MaxInputChars    int            `yaml:"max_input_chars" json:"max_input_chars"`
	MaxOutputTokens  int            `yaml:"max_output_tokens" json:"max_output_tokens"`
	Temperature      float64        `yaml:"temperature" json:"temperature"`
	Capabilities     map[string]any `yaml:"capabilities" json:"capabilities"`
}

// CommandPolicy defines local command execution limits.
type CommandPolicy struct {
	DefaultTimeoutSeconds int      `yaml:"default_timeout_seconds" json:"default_timeout_seconds"`
	MaxOutputBytes        int      `yaml:"max_output_bytes" json:"max_output_bytes"`
	MaxLines              int      `yaml:"max_lines" json:"max_lines"`
	AllowedCWDs           []string `yaml:"allowed_cwds" json:"allowed_cwds"`
	LogDir                string   `yaml:"log_dir" json:"log_dir"`
	LogEnabled            *bool    `yaml:"log_enabled" json:"log_enabled"`
	LogRetentionDays      int      `yaml:"log_retention_days" json:"log_retention_days"`
	LogMaxRecords         int      `yaml:"log_max_records" json:"log_max_records"`
	LogCompress           bool     `yaml:"log_compress" json:"log_compress"`
}

// PipelinePolicy defines workflow and result composition limits.
type PipelinePolicy struct {
	MaxReturnChars             int  `yaml:"max_return_chars" json:"max_return_chars"`
	RequireEvidenceForAnalysis bool `yaml:"require_evidence_for_analysis" json:"require_evidence_for_analysis"`
}

// Load reads a YAML config file and applies safe defaults.
func ensureDefaultConfigFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultConfigYAML()), 0o600)
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("MCP_AI_HELPER_CONFIG")
	}
	if path == "" {
		path = DefaultConfigPath()
	}
	if err := ensureDefaultConfigFile(path); err != nil {
		return nil, err
	}

	// #nosec G304,G703 -- path is the explicit user-selected config path for this local CLI/server.
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && filepath.Base(path) == "config.yaml" {
			return defaultConfig(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg.SourcePath = path
	return &cfg, nil
}

// DefaultConfigPath returns the per-user default config path.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(defaultConfigDir, "config.yaml")
	}
	return filepath.Join(home, defaultConfigDir, "config.yaml")
}

// DefaultAssistantGuidance returns the built-in guidance text used when config omits it.
func DefaultAssistantGuidance() string {
	return defaultAssistantGuidance
}

// SetupGuidance describes how a caller should configure the server.
func SetupGuidance(configPath string) map[string]string {
	if strings.TrimSpace(configPath) == "" {
		configPath = DefaultConfigPath()
	}
	return map[string]string{
		"config_path": configPath,
		"summary":     "Keep server guidance and base policy in ~/.mcp-ai-helper/config.yaml; edit assistant_guidance there to tune the MCP assistant_guidance/server_setup_guidance behavior without code changes.",
		"first_run":   "When the config file is missing, mcp-ai-helper creates ~/.mcp-ai-helper/config.yaml with safe local-command defaults.",
		"layers":      "Use layers.<name>.enabled to toggle logs, tasks, guidance, models, commands, and workflows without editing code.",
		"models":      "Add providers/models only when remote model calls are needed; local pipelines and tasks work without provider credentials.",
		"workflows":   "Prefer one long run_workflow or run_pipeline call when intermediate results are not needed by the calling model. Include deterministic conditions, guarded edits, relevant checks, task status transitions, and owned-files commits in that workflow when they do not require caller-side decisions.",
		"tasks":       "Read task_current before work, keep statuses current, batch task updates only from a complete authoritative task set, and use close_missing only intentionally because it can close omitted active tasks. A task is done only when acceptance criteria, gates, and required owned-files commit pass; no commit means the task is not done. For repo tasks with file changes, finalization must be atomic inside one run_workflow: after acceptance gates pass, transition task status, then run git_commit_owned in the same workflow with owned_files covering changed work files and the canonical task registry mutation. Missing commit, failed commit, missing task registry mutation, separate post-hoc status commit, partial green tests, evidence-only analysis, skipped checks, and stale task reads are blockers; none is enough to mark done. Migrated Lean/Lake repos use the Lean registry/exporter as canonical task state; legacy tasks/*.lean JSON-comment files are not fallback storage. Do not implement production task state by parsing or regex-mutating Lean registry source in Go; use Lean-owned lake serve/exporter/task tools and fail closed when that surface cannot express the mutation.",
	}
}

func defaultConfigYAML() string {
	return `# mcp-ai-helper default server config.
# This file is created on first run at ~/.mcp-ai-helper/config.yaml.
# Keep model/provider credentials out of git. Add them here only when remote model calls are needed.

assistant_guidance: |
  ` + strings.ReplaceAll(defaultAssistantGuidance, "\n", "\n  ") + `

layers:
  logs:
    enabled: true
  tasks:
    enabled: true
  issues:
    enabled: false
  guidance:
    enabled: true
  models:
    enabled: true
  commands:
    enabled: true
  workflows:
    enabled: true
  reasoning_patterns:
    enabled: true

providers:
  # example:
  #   type: generic
  #   base_url: https://api.example.com/v1
  #   api_key_env: EXAMPLE_API_KEY
  #   timeout_seconds: 120
  #   max_retries: 2
models:
  # example_routine:
  #   provider: example
  #   model: provider/model-name
  #   tier: low
  #   roles: [summarizer, classifier, routine_debug]
  #   purpose: cheap grounded summaries from logs and command output
  #   system_prompt: |
  #     Extract only grounded facts from command output.
  #     Return concise JSON when requested. Do not invent causes.
  #     Every non-trivial claim must cite evidence ids like [E1].
  #   max_input_chars: 30000
  #   max_output_tokens: 1800
  #   temperature: 0
routing: {}

command_policy:
  default_timeout_seconds: 20
  max_output_bytes: 200000
  max_lines: 400
  log_dir: ~/.mcp-ai-helper/repos
  log_enabled: true
  log_retention_days: 30
  log_max_records: 2000
  log_compress: true
  allowed_cwds:
    - .

pipeline_policy:
  max_return_chars: 4000
  require_evidence_for_analysis: true
`
}

func defaultConfig() *Config {
	cfg := &Config{
		Providers:         map[string]ProviderConfig{},
		Models:            map[string]ModelConfig{},
		Routing:           map[string]string{},
		AssistantGuidance: defaultAssistantGuidance,
		CommandPolicy: CommandPolicy{
			DefaultTimeoutSeconds: 20,
			MaxOutputBytes:        200000,
			MaxLines:              400,
			AllowedCWDs:           []string{"."},
		},
		PipelinePolicy: PipelinePolicy{
			MaxReturnChars:             4000,
			RequireEvidenceForAnalysis: true,
		},
	}
	return cfg
}

// LayerEnabled reports whether an optional server layer is enabled. Unknown layers default to true.
func (c *Config) LayerEnabled(name string) bool {
	switch name {
	case "logs":
		return layerEnabled(c.Layers.Logs)
	case "tasks":
		return layerEnabled(c.Layers.Tasks)
	case "issues":
		return layerEnabled(c.Layers.Issues)
	case "guidance":
		return layerEnabled(c.Layers.Guidance)
	case "models":
		return layerEnabled(c.Layers.Models)
	case "commands":
		return layerEnabled(c.Layers.Commands)
	case "workflows":
		return layerEnabled(c.Layers.Workflows)
	case "reasoning_patterns":
		return layerEnabled(c.Layers.ReasoningPatterns)
	default:
		return true
	}
}

func layerEnabled(layer LayerConfig) bool {
	return layer.Enabled == nil || *layer.Enabled
}

func applyDefaults(cfg *Config) {
	if cfg.CommandPolicy.LogEnabled == nil {
		value := cfg.LayerEnabled("logs")
		cfg.CommandPolicy.LogEnabled = &value
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}
	if cfg.Models == nil {
		cfg.Models = map[string]ModelConfig{}
	}
	if cfg.Routing == nil {
		cfg.Routing = map[string]string{}
	}
	if strings.TrimSpace(cfg.AssistantGuidance) == "" {
		cfg.AssistantGuidance = defaultAssistantGuidance
	}
	if cfg.CommandPolicy.DefaultTimeoutSeconds <= 0 {
		cfg.CommandPolicy.DefaultTimeoutSeconds = 20
	}
	if cfg.CommandPolicy.MaxOutputBytes <= 0 {
		cfg.CommandPolicy.MaxOutputBytes = 200000
	}
	if cfg.CommandPolicy.MaxLines <= 0 {
		cfg.CommandPolicy.MaxLines = 400
	}
	if len(cfg.CommandPolicy.AllowedCWDs) == 0 {
		cfg.CommandPolicy.AllowedCWDs = []string{"."}
	}
	if cfg.PipelinePolicy.MaxReturnChars <= 0 {
		cfg.PipelinePolicy.MaxReturnChars = 4000
	}
}

// Validate checks cross-references and provider/model invariants.
func (c *Config) Validate() error {
	for id, provider := range c.Providers {
		if provider.Type == "" {
			provider.Type = "generic"
		}
		switch provider.Type {
		case "generic", "anthropic":
		default:
			return fmt.Errorf("provider %q: unsupported type %q", id, provider.Type)
		}
		if provider.Type != "anthropic" && provider.BaseURL == "" && provider.CompletionsURL == "" {
			return fmt.Errorf("provider %q: base_url or completions_url is required", id)
		}
	}
	for id, model := range c.Models {
		if model.Provider == "" {
			return fmt.Errorf("model %q: provider is required", id)
		}
		if _, ok := c.Providers[model.Provider]; !ok {
			return fmt.Errorf("model %q: provider %q is not configured", id, model.Provider)
		}
		if model.Model == "" {
			return fmt.Errorf("model %q: model name is required", id)
		}
	}
	return nil
}

// ResolvedAPIKey returns the literal API key or the value of APIKeyEnv.
func (p ProviderConfig) ResolvedAPIKey() string {
	if p.APIKey != "" {
		return p.APIKey
	}
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}

// Timeout returns the provider request timeout.
func (p ProviderConfig) Timeout() time.Duration {
	if p.TimeoutSeconds <= 0 {
		return 120 * time.Second
	}
	return time.Duration(p.TimeoutSeconds) * time.Second
}

// RetryCount returns the non-negative provider retry count.
func (p ProviderConfig) RetryCount() int {
	if p.MaxRetries < 0 {
		return 0
	}
	return p.MaxRetries
}

// Prompt returns the trimmed model system prompt.
func (m ModelConfig) Prompt() string {
	return strings.TrimSpace(m.SystemPrompt)
}
