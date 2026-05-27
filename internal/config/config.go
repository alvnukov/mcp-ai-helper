// Package config loads and validates mcp-ai-helper YAML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/zol/mcp-ai-helper/internal/security"
)

const (
	defaultConfigDir = ".mcp-ai-helper"
	repoConfigFile   = ".mcp-ai-helper.yaml"
)

const defaultAssistantGuidance = `mcp-ai-helper operating guidance:

1. Prefer one long run_workflow or run_pipeline call when intermediate results are not needed by the calling model.
2. Put command execution, output filters, deterministic conditions, guarded edits, checks, and commit into that workflow instead of making many low-level calls.
3. Use low-level tools only for bootstrapping, schema discovery, or when the returned result must change the caller's next decision.
4. Always pass repo_path. Treat cwd and file paths as repo-relative unless a tool explicitly says otherwise.
5. Never use broad staging or destructive git operations. Commit only explicit owned files after relevant checks pass.
6. Keep output compact and evidence-linked. Re-filter retained command history instead of rerunning commands or returning raw logs.
7. For any repo task, first gather complete minimal sufficient context: use task_current for discovery, task_graph for overview/dependencies, task_context for selected-task execution context, task_packet for readiness/owned_files/gates when executing, then relevant read_file/snapshot_file calls and narrow run_pipeline/collect_command_output probes; do not build an execution pipeline before understanding the contract, architecture, integration points, test patterns, and existing changes.
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
	WebPolicy         WebPolicy                 `yaml:"web_policy" json:"web_policy"`
	Integrations      IntegrationsConfig        `yaml:"integrations" json:"integrations"`
	TaskRegistry      TaskRegistryConfig        `yaml:"task_registry" json:"task_registry"`
	Secrets           map[string]SecretConfig   `yaml:"secrets" json:"-"`
}

// SecretConfig holds a single named server-config secret. Never serialized to JSON.
type SecretConfig struct {
	Value   string `yaml:"value" json:"-"`
	Enabled bool   `yaml:"enabled" json:"-"`
}

// validSecretHandle matches allowed secret handle names.
var validSecretHandle = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

// ResolveSecretEnv validates handles and returns env vars ("HELPER_SECRET_<NAME>=<value>")
// and a Mask populated with resolved values. Returns error on first invalid/missing/disabled handle.
func (c Config) ResolveSecretEnv(handles []string) ([]string, *security.Mask, error) {
	if len(handles) == 0 {
		return nil, nil, nil
	}
	envs := make([]string, 0, len(handles))
	mask := security.NewMask()
	for _, h := range handles {
		if !validSecretHandle.MatchString(h) {
			return nil, nil, fmt.Errorf("invalid secret handle: %s", h)
		}
		s, ok := c.Secrets[h]
		if !ok {
			return nil, nil, fmt.Errorf("secret handle not found: %s", h)
		}
		if !s.Enabled {
			return nil, nil, fmt.Errorf("secret is disabled: %s", h)
		}
		if len(s.Value) < 8 {
			return nil, nil, fmt.Errorf("secret value too short for handle: %s", h)
		}
		envs = append(envs, "HELPER_SECRET_"+h+"="+s.Value)
		mask.AddNamed(h, s.Value)
	}
	return envs, mask, nil
}

// SecretMask returns a redaction mask for all configured server-side secrets.
func (c Config) SecretMask() *security.Mask {
	mask := security.NewMask()
	add := func(name string, value string) {
		if len(value) < 8 {
			return
		}
		if name == "" {
			mask.Add(value)
			return
		}
		mask.AddNamed(name, value)
	}
	for handle, secret := range c.Secrets {
		add(handle, secret.Value)
	}
	if c.Integrations.Jira != nil {
		add("", c.Integrations.Jira.APIKey)
		if c.Integrations.Jira.APIKeyEnv != "" {
			add("", os.Getenv(c.Integrations.Jira.APIKeyEnv))
		}
	}
	if c.Integrations.Confluence != nil {
		add("", c.Integrations.Confluence.APIKey)
		if c.Integrations.Confluence.APIKeyEnv != "" {
			add("", os.Getenv(c.Integrations.Confluence.APIKeyEnv))
		}
	}
	for _, provider := range c.Providers {
		add("", provider.APIKey)
		if provider.APIKeyEnv != "" {
			add("", os.Getenv(provider.APIKeyEnv))
		}
	}
	add("", c.WebPolicy.GoogleAPIKey)
	if c.WebPolicy.GoogleAPIKeyEnv != "" {
		add("", os.Getenv(c.WebPolicy.GoogleAPIKeyEnv))
	}
	return mask
}

// RepoPermissions defines per-repository LLM permissions set by the user in .mcp-ai-helper.yaml.
type RepoPermissions struct {
	Tools ToolPermissions `yaml:"tools" json:"tools"`
}

// ToolPermissions controls which MCP tools the LLM may call in this repo.
type ToolPermissions struct {
	Deny []string `yaml:"deny" json:"deny"`
}

// RepoConfig is a repo-local optional config loaded from .mcp-ai-helper.yaml.
// config_replace refuses to write this file; feature tools may update only the features section.
type RepoConfig struct {
	SourcePath    string              `yaml:"-" json:"repo_config_path"`
	Permissions   RepoPermissions     `yaml:"permissions" json:"permissions"`
	TaskRegistry  *TaskRegistryConfig `yaml:"task_registry" json:"task_registry,omitempty"`
	CommandPolicy *struct {
		AllowedCWDs []string `yaml:"allowed_cwds" json:"allowed_cwds"`
	} `yaml:"command_policy" json:"command_policy"`
	Features FeatureState `yaml:"features" json:"features"`
}

// FeatureState stores explicit feature overrides and a compact audit trail.
type FeatureState struct {
	Overrides map[string]FeatureOverride `yaml:"overrides" json:"overrides"`
	Audit     []FeatureAuditEntry        `yaml:"audit" json:"audit,omitempty"`
}

// FeatureOverride stores one explicit feature value.
type FeatureOverride struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Reason    string `yaml:"reason,omitempty" json:"reason,omitempty"`
	UpdatedAt string `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// FeatureAuditEntry records one feature override mutation.
type FeatureAuditEntry struct {
	Timestamp       string `yaml:"timestamp" json:"timestamp"`
	Scope           string `yaml:"scope" json:"scope"`
	FeatureID       string `yaml:"feature_id" json:"feature_id"`
	PreviousEnabled bool   `yaml:"previous_enabled" json:"previous_enabled"`
	PreviousSource  string `yaml:"previous_source" json:"previous_source"`
	NewEnabled      bool   `yaml:"new_enabled" json:"new_enabled"`
	NewSource       string `yaml:"new_source" json:"new_source"`
	Reason          string `yaml:"reason,omitempty" json:"reason,omitempty"`
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
	ProtectedConfigPath   string   `yaml:"-" json:"-"`
}

// PipelinePolicy defines workflow and result composition limits.
type PipelinePolicy struct {
	MaxReturnChars             int  `yaml:"max_return_chars" json:"max_return_chars"`
	RequireEvidenceForAnalysis bool `yaml:"require_evidence_for_analysis" json:"require_evidence_for_analysis"`
}

// WebPolicy defines bounded network fetch limits for model-facing web tools.
type WebPolicy struct {
	Enabled              *bool    `yaml:"enabled" json:"enabled"`
	CacheDir             string   `yaml:"cache_dir" json:"cache_dir"`
	MaxSourceBytes       int64    `yaml:"max_source_bytes" json:"max_source_bytes"`
	TimeoutSeconds       int      `yaml:"timeout_seconds" json:"timeout_seconds"`
	MaxRedirects         int      `yaml:"max_redirects" json:"max_redirects"`
	AllowedSchemes       []string `yaml:"allowed_schemes" json:"allowed_schemes"`
	AllowedHosts         []string `yaml:"allowed_hosts" json:"allowed_hosts"`
	DeniedHosts          []string `yaml:"denied_hosts" json:"denied_hosts"`
	AcceptedContentTypes []string `yaml:"accepted_content_types" json:"accepted_content_types"`
	UserAgent            string   `yaml:"user_agent" json:"user_agent"`
	SearchProvider       string   `yaml:"search_provider" json:"search_provider"`
	SearchURL            string   `yaml:"search_url" json:"search_url"`
	MaxSearchResults     int      `yaml:"max_search_results" json:"max_search_results"`
	GoogleCSEID          string   `yaml:"google_cse_id" json:"google_cse_id"`
	GoogleAPIKeyEnv      string   `yaml:"google_api_key_env" json:"google_api_key_env"`
	GoogleAPIKey         string   `yaml:"google_api_key" json:"-"`
	GoogleCSEURL         string   `yaml:"google_cse_url" json:"google_cse_url"`
}

// IsEnabled returns true unless web access is explicitly disabled.
func (p WebPolicy) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }

// IntegrationsConfig holds third-party integration settings.
type IntegrationsConfig struct {
	Jira       *JiraConfig       `yaml:"jira" json:"jira"`
	Confluence *ConfluenceConfig `yaml:"confluence" json:"confluence"`
}

// ConfluenceConfig holds Confluence connection settings.
type ConfluenceConfig struct {
	URL           string   `yaml:"url" json:"url"`
	Username      string   `yaml:"username" json:"username"`
	APIKey        string   `yaml:"api_key" json:"-"`
	APIKeyEnv     string   `yaml:"api_key_env" json:"-"`
	AllowedSpaces []string `yaml:"allowed_spaces" json:"allowed_spaces"`
	ReadOnly      *bool    `yaml:"read_only" json:"read_only"`
	Enabled       *bool    `yaml:"enabled" json:"enabled"`
}

// TaskRegistryConfig controls which task registry backend is used.
type TaskRegistryConfig struct {
	Backend  string                 `yaml:"backend" json:"backend"`
	Obsidian ObsidianRegistryConfig `yaml:"obsidian" json:"obsidian"`
}

// ObsidianRegistryConfig holds Obsidian-backed registry settings.
type ObsidianRegistryConfig struct {
	Path         string `yaml:"path" json:"path"`
	Vault        string `yaml:"vault" json:"vault,omitempty"`
	ResolvedPath string `yaml:"-" json:"-"`
}

// IsEnabled returns true when the integration is enabled (default true when non-nil).
func (c *ConfluenceConfig) IsEnabled() bool {
	if c == nil {
		return false
	}
	return c.Enabled == nil || *c.Enabled
}

// ResolvedAPIKey returns the API key: direct value first, then env fallback.
func (c ConfluenceConfig) ResolvedAPIKey() string {
	if c.APIKey != "" {
		return c.APIKey
	}
	if c.APIKeyEnv != "" {
		return os.Getenv(c.APIKeyEnv)
	}
	return ""
}

// CanMutate returns false when read_only is explicitly true.
func (c *ConfluenceConfig) CanMutate() bool { return c.ReadOnly == nil || !*c.ReadOnly }

// IsSpaceAllowed checks if a space key is in the allowlist.
func (c *ConfluenceConfig) IsSpaceAllowed(spaceKey string) bool {
	if len(c.AllowedSpaces) == 0 {
		return true
	}
	for _, s := range c.AllowedSpaces {
		if s == spaceKey {
			return true
		}
	}
	return false
}

// JiraConfig holds Jira connection settings.
type JiraConfig struct {
	URL             string   `yaml:"url" json:"url"`
	Username        string   `yaml:"username" json:"username"`
	APIKey          string   `yaml:"api_key" json:"-"`
	APIKeyEnv       string   `yaml:"api_key_env" json:"-"`
	AllowedProjects []string `yaml:"allowed_projects" json:"allowed_projects"`
	ReadOnly        *bool    `yaml:"read_only" json:"read_only"`
	Enabled         *bool    `yaml:"enabled" json:"enabled"`
}

// CanMutate returns false when read_only is explicitly true.
func (j *JiraConfig) CanMutate() bool { return j.ReadOnly == nil || !*j.ReadOnly }

// IsProjectAllowed checks if an issue key's project is in the allowlist.
func (j *JiraConfig) IsProjectAllowed(issueKey string) bool {
	if len(j.AllowedProjects) == 0 {
		return true
	}
	parts := strings.SplitN(issueKey, "-", 2)
	if len(parts) == 0 {
		return false
	}
	for _, p := range j.AllowedProjects {
		if p == parts[0] {
			return true
		}
	}
	return false
}

// IsEnabled returns true when the integration is enabled (default true when non-nil).
func (j *JiraConfig) IsEnabled() bool {
	if j == nil {
		return false
	}
	return j.Enabled == nil || *j.Enabled
}

// ResolvedAPIKey returns the API key: direct value first, then env fallback.
func (j JiraConfig) ResolvedAPIKey() string {
	if j.APIKey != "" {
		return j.APIKey
	}
	if j.APIKeyEnv != "" {
		return os.Getenv(j.APIKeyEnv)
	}
	return ""
}

// Load reads a YAML config file and applies safe defaults.
// #nosec G703 -- path from validated config, local fs operations only
func ensureDefaultConfigFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
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

// LoadRepoConfig reads an optional .mcp-ai-helper.yaml from repoPath.
// Returns nil, nil when no repo config exists.
func LoadRepoConfig(repoPath string) (*RepoConfig, error) {
	if strings.TrimSpace(repoPath) == "" {
		return nil, nil
	}
	path := filepath.Join(repoPath, repoConfigFile)
	data, err := os.ReadFile(path) // #nosec G304 -- path is repo-local helper config under the caller-selected repo.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid repo config %s: %w", path, err)
	}
	cfg.SourcePath = path
	return &cfg, nil
}

// ToolDenied reports whether the repo-local policy denies a tool name exactly.
func (c *RepoConfig) ToolDenied(toolName string) bool {
	if c == nil {
		return false
	}
	want := strings.TrimSpace(toolName)
	if want == "" {
		return false
	}
	for _, denied := range c.Permissions.Tools.Deny {
		if strings.TrimSpace(denied) == want {
			return true
		}
	}
	return false
}

// MergeRepoConfig overlays the supported user-owned repo-local policy on top of the global config.
func MergeRepoConfig(base *Config, repoCfg *RepoConfig, repoPath string) (*Config, error) {
	if base == nil {
		return nil, errors.New("base config is required")
	}
	merged := *base
	if repoCfg != nil {
		if repoCfg.CommandPolicy != nil && len(repoCfg.CommandPolicy.AllowedCWDs) > 0 {
			allowed, err := resolveRepoAllowedCWDs(repoPath, repoCfg.CommandPolicy.AllowedCWDs)
			if err != nil {
				return nil, err
			}
			merged.CommandPolicy = base.CommandPolicy
			merged.CommandPolicy.AllowedCWDs = allowed
		}
		if repoCfg.TaskRegistry != nil {
			merged.TaskRegistry = *repoCfg.TaskRegistry
		}
	}
	applyDefaults(&merged)
	registry, err := resolveRepoTaskRegistry(repoPath, merged.TaskRegistry)
	if err != nil {
		return nil, err
	}
	merged.TaskRegistry = registry
	if err := merged.Validate(); err != nil {
		return nil, err
	}
	return &merged, nil
}

func resolveRepoTaskRegistry(repoPath string, registry TaskRegistryConfig) (TaskRegistryConfig, error) {
	registry.Backend = strings.TrimSpace(registry.Backend)
	registry.Obsidian.Path = strings.TrimSpace(registry.Obsidian.Path)
	registry.Obsidian.ResolvedPath = strings.TrimSpace(registry.Obsidian.ResolvedPath)
	if registry.Backend == "obsidian" && registry.Obsidian.Path != "" {
		if filepath.IsAbs(registry.Obsidian.Path) {
			registry.Obsidian.ResolvedPath = registry.Obsidian.Path
			return registry, nil
		}
		repo, err := filepath.Abs(strings.TrimSpace(repoPath))
		if err != nil {
			return TaskRegistryConfig{}, fmt.Errorf("resolve repo_path: %w", err)
		}
		if repo == "" {
			return TaskRegistryConfig{}, errors.New("repo_path is required for repo-local task_registry.obsidian.path")
		}
		resolved := filepath.Join(repo, registry.Obsidian.Path)
		if !insideDir(repo, resolved) {
			return TaskRegistryConfig{}, fmt.Errorf("repo-local task_registry.obsidian.path %q escapes repo_path", registry.Obsidian.Path)
		}
		registry.Obsidian.ResolvedPath = resolved
	}
	return registry, nil
}

func resolveRepoAllowedCWDs(repoPath string, values []string) ([]string, error) {
	repo, err := filepath.Abs(strings.TrimSpace(repoPath))
	if err != nil {
		return nil, fmt.Errorf("resolve repo_path: %w", err)
	}
	if repo == "" {
		return nil, errors.New("repo_path is required for repo-local allowed_cwds")
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		candidate := trimmed
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(repo, candidate)
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			return nil, fmt.Errorf("resolve allowed_cwds entry %q: %w", trimmed, err)
		}
		if !insideDir(repo, abs) {
			return nil, fmt.Errorf("repo-local allowed_cwds entry %q escapes repo_path", trimmed)
		}
		out = append(out, abs)
	}
	if len(out) == 0 {
		return nil, errors.New("repo-local allowed_cwds must contain at least one non-empty path")
	}
	return out, nil
}

func insideDir(root string, child string) bool {
	return child == root || strings.HasPrefix(child, root+string(os.PathSeparator))
}

// IsRepoConfigPath returns true when path points to a repo-local config file.
func IsRepoConfigPath(path string) bool {
	return filepath.Base(path) == repoConfigFile
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
		"config_path":               configPath,
		"summary":                   "Keep server guidance and base policy in ~/.mcp-ai-helper/config.yaml; edit assistant_guidance there to tune the MCP assistant_guidance/server_setup_guidance behavior without code changes.",
		"first_run":                 "When the config file is missing, mcp-ai-helper creates ~/.mcp-ai-helper/config.yaml with safe local-command defaults.",
		"layers":                    "Use layers.<name>.enabled to toggle logs, tasks, guidance, models, commands, and workflows without editing code.",
		"models":                    "Add providers/models only when remote model calls are needed; local pipelines and tasks work without provider credentials.",
		"workflows":                 "Prefer one long run_workflow or run_pipeline call when intermediate results are not needed by the calling model. Include deterministic conditions, guarded edits, relevant checks, task status transitions, and owned-files commits in that workflow when they do not require caller-side decisions.",
		"tasks":                     "Read task_current before work for discovery, use task_graph for dependency overview, task_context for selected-task execution context, and task_packet for readiness/owned_files/gates before file reads and execution. Keep statuses current, batch task updates only from a complete authoritative task set, and use close_missing only intentionally because it can close omitted active tasks. A task is done only when acceptance criteria, gates, and required owned-files commit pass; no commit means the task is not done. For repo tasks with file changes, finalization must be atomic inside one run_workflow: after acceptance gates pass, transition task status, then run git_commit_owned in the same workflow with owned_files covering changed work files and the canonical task registry mutation. Missing commit, failed commit, missing task registry mutation, separate post-hoc status commit, partial green tests, evidence-only analysis, skipped checks, and stale task reads are blockers; none is enough to mark done. Migrated Lean/Lake repos use the Lean registry/exporter as canonical task state; legacy tasks/*.lean JSON-comment files are not fallback storage. Do not implement production task state by parsing or regex-mutating Lean registry source in Go; use Lean-owned lake serve/exporter/task tools and fail closed when that surface cannot express the mutation.",
		"lean_task_registry_repair": "If task_current reports repair_required/action=repair_lean_task_registry_exporter, the repo has MCPAIHelperProject/ActiveTasks.lean but lacks the exporter surface. Repair by adding MCPAIHelperProject/TaskRegistryExport.lean from the mcp-ai-helper canonical exporter module, declaring a task_registry_export executable in lakefile.lean or lakefile.toml with root MCPAIHelperProject.TaskRegistryExport, then verifying with lake build, lake exe task_registry_export --list-active, and task_current. Do not fall back to legacy tasks/*.lean files.",
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
  default_timeout_seconds: 300
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

web_policy:
  enabled: true
  cache_dir: ~/.mcp-ai-helper/web
  max_source_bytes: 1048576
  timeout_seconds: 20
  max_redirects: 5
  allowed_schemes: [https, http]
  accepted_content_types: [text/html, text/plain, application/json, application/xml, text/]
  user_agent: mcp-ai-helper/0.1
  # Leave empty to require an explicit web_search provider argument per call.
  # Set to duckduckgo_html to make that provider the default.
  search_provider: ""
  search_url: https://html.duckduckgo.com/html/
  max_search_results: 10
  # Google Custom Search JSON API provider (provider: google_cse).
  google_cse_id: ""
  google_api_key_env: GOOGLE_CSE_API_KEY
  google_cse_url: https://www.googleapis.com/customsearch/v1

integrations:
  jira:
    # url: https://your-domain.atlassian.net
    # username: bot@example.com
    # api_key_env: JIRA_API_KEY
    enabled: false
`
}

func defaultConfig() *Config {
	cfg := &Config{
		Providers:         map[string]ProviderConfig{},
		Models:            map[string]ModelConfig{},
		Routing:           map[string]string{},
		AssistantGuidance: defaultAssistantGuidance,
		CommandPolicy: CommandPolicy{
			DefaultTimeoutSeconds: 300,
			MaxOutputBytes:        200000,
			MaxLines:              400,
			AllowedCWDs:           []string{"."},
		},
		PipelinePolicy: PipelinePolicy{
			MaxReturnChars:             4000,
			RequireEvidenceForAnalysis: true,
		},
		WebPolicy: defaultWebPolicy(),
	}
	return cfg
}

func boolPtr(value bool) *bool { return &value }

func defaultWebPolicy() WebPolicy {
	return WebPolicy{
		Enabled:              boolPtr(true),
		CacheDir:             "~/.mcp-ai-helper/web",
		MaxSourceBytes:       1048576,
		TimeoutSeconds:       20,
		MaxRedirects:         5,
		AllowedSchemes:       []string{"https", "http"},
		AcceptedContentTypes: []string{"text/html", "text/plain", "application/json", "application/xml", "text/"},
		UserAgent:            "mcp-ai-helper/0.1",
		SearchURL:            "https://html.duckduckgo.com/html/",
		MaxSearchResults:     10,
		GoogleCSEURL:         "https://www.googleapis.com/customsearch/v1",
	}
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
	case "jira":
		return c.Integrations.Jira != nil && c.Integrations.Jira.IsEnabled()
	default:
		return true
	}
}

func layerEnabled(layer LayerConfig) bool {
	return layer.Enabled == nil || *layer.Enabled
}

func applyWebPolicyDefaults(policy *WebPolicy) {
	defaults := defaultWebPolicy()
	if policy.Enabled == nil {
		policy.Enabled = defaults.Enabled
	}
	if strings.TrimSpace(policy.CacheDir) == "" {
		policy.CacheDir = defaults.CacheDir
	}
	if policy.MaxSourceBytes <= 0 {
		policy.MaxSourceBytes = defaults.MaxSourceBytes
	}
	if policy.TimeoutSeconds <= 0 {
		policy.TimeoutSeconds = defaults.TimeoutSeconds
	}
	if policy.MaxRedirects <= 0 {
		policy.MaxRedirects = defaults.MaxRedirects
	}
	if len(policy.AllowedSchemes) == 0 {
		policy.AllowedSchemes = defaults.AllowedSchemes
	}
	if len(policy.AcceptedContentTypes) == 0 {
		policy.AcceptedContentTypes = defaults.AcceptedContentTypes
	}
	if strings.TrimSpace(policy.UserAgent) == "" {
		policy.UserAgent = defaults.UserAgent
	}
	policy.SearchProvider = strings.TrimSpace(policy.SearchProvider)
	if strings.TrimSpace(policy.SearchURL) == "" {
		policy.SearchURL = defaults.SearchURL
	}
	if policy.MaxSearchResults <= 0 {
		policy.MaxSearchResults = defaults.MaxSearchResults
	}
	policy.GoogleCSEID = strings.TrimSpace(policy.GoogleCSEID)
	policy.GoogleAPIKeyEnv = strings.TrimSpace(policy.GoogleAPIKeyEnv)
	if strings.TrimSpace(policy.GoogleCSEURL) == "" {
		policy.GoogleCSEURL = defaults.GoogleCSEURL
	}
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
		cfg.CommandPolicy.DefaultTimeoutSeconds = 300
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
	applyWebPolicyDefaults(&cfg.WebPolicy)
	cfg.TaskRegistry.Backend = strings.TrimSpace(cfg.TaskRegistry.Backend)
	if cfg.TaskRegistry.Backend == "" {
		cfg.TaskRegistry.Backend = "lean"
	}
	cfg.TaskRegistry.Obsidian.Path = strings.TrimSpace(cfg.TaskRegistry.Obsidian.Path)
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
	switch c.TaskRegistry.Backend {
	case "lean":
	case "obsidian":
		path := strings.TrimSpace(c.TaskRegistry.Obsidian.Path)
		if path == "" {
			return errors.New("task_registry.obsidian.path is required")
		}
		checkPath := strings.TrimSpace(c.TaskRegistry.Obsidian.ResolvedPath)
		if checkPath == "" {
			if !filepath.IsAbs(path) {
				return nil
			}
			checkPath = path
		}
		info, err := os.Stat(checkPath)
		if err != nil {
			return fmt.Errorf("task_registry.obsidian.path not readable: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("task_registry.obsidian.path is not a directory: %s", path)
		}
	default:
		return fmt.Errorf("unsupported task_registry.backend: %s", c.TaskRegistry.Backend)
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
