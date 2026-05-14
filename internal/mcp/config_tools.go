package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"

	"github.com/zol/mcp-ai-helper/internal/config"
)

type configReloadFunc func(path string) (*config.Config, error)

type configPathRequest struct {
	Path     string `json:"path"`
	RepoPath string `json:"repo_path"`
}

type configReplaceRequest struct {
	Path       string `json:"path"`
	ConfigYAML string `json:"config_yaml"`
	Reload     *bool  `json:"reload"`
	RepoPath   string `json:"repo_path"`
}

type configOptionSetRequest struct {
	Path       string `json:"path"`
	Value      string `json:"value"`
	ConfigPath string `json:"config_path"`
	Reload     *bool  `json:"reload"`
}

type configOptionResetRequest struct {
	Path       string `json:"path"`
	ConfigPath string `json:"config_path"`
	Reload     *bool  `json:"reload"`
}

func registerConfigTools(srv *server.MCPServer, deps *Server, reload configReloadFunc) {
	registerConfigOptionTools(srv, deps, reload)
	srv.AddTool(basemcp.NewTool("config_schema",
		basemcp.WithDescription("Return machine-readable documentation for every mcp-ai-helper config field and the safe model-driven setup workflow."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(config.Schema())
	})

	srv.AddTool(basemcp.NewTool("config_read",
		basemcp.WithDescription("Return the active sanitized config, or validate/read another config path without exposing literal api_key values. Pass repo_path to merge repo-local .mcp-ai-helper.yaml."),
		basemcp.WithString("path", basemcp.Description("Optional config path. Empty means active in-memory config.")),
		basemcp.WithString("repo_path", basemcp.Description("Optional repo root to load and merge .mcp-ai-helper.yaml.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configPathRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		if strings.TrimSpace(args.Path) == "" {
			cfg, _, _, _, _ := deps.loadDeps()
			result := map[string]any{"config": cfg, "config_path": cfg.SourcePath, "source": "memory"}
			if strings.TrimSpace(args.RepoPath) != "" {
				repoCfg, err := config.LoadRepoConfig(args.RepoPath)
				if err != nil {
					return basemcp.NewToolResultError(err.Error()), nil
				}
				if repoCfg != nil {
					merged, err := config.MergeRepoConfig(cfg, repoCfg, args.RepoPath)
					if err != nil {
						return basemcp.NewToolResultError(err.Error()), nil
					}
					result["config"] = merged
					result["repo_config"] = repoCfg
					result["repo_config_path"] = repoCfg.SourcePath
					result["source"] = "memory+repo"
				}
			}
			return structured(result)
		}
		loaded, err := config.Load(args.Path)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"config": loaded, "config_path": loaded.SourcePath, "source": "file"})
	})

	srv.AddTool(basemcp.NewTool("config_replace",
		basemcp.WithDescription("Validate and atomically replace the complete YAML config. Reloads the running helper by default, without restarting Codex. Cannot write repo-local .mcp-ai-helper.yaml files."),
		basemcp.WithString("config_yaml", basemcp.Required(), basemcp.Description("Complete YAML config document.")),
		basemcp.WithString("path", basemcp.Description("Optional config path. Empty means the active config path.")),
		basemcp.WithString("repo_path", basemcp.Description("Repository root from the calling LLM. Used to detect repo-local config writes.")),
		basemcp.WithBoolean("reload", basemcp.Description("Reload runtime after writing. Defaults to true.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configReplaceRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath(args.Path, cfg.SourcePath)

		// Repo-local configs are user-editable only.
		if config.IsRepoConfigPath(path) {
			return basemcp.NewToolResultError("repo config (.mcp-ai-helper.yaml) is user-editable only; use config_read with repo_path to inspect it"), nil
		}

		loaded, err := writeValidatedConfig(path, args.ConfigYAML)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		reloadNow := args.Reload == nil || *args.Reload
		if reloadNow {
			loaded, err = reload(path)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
		}
		return structured(map[string]any{"status": "ok", "reloaded": reloadNow, "config_path": path, "config": loaded})
	})

	srv.AddTool(basemcp.NewTool("config_reload",
		basemcp.WithDescription("Reload the running helper from config YAML without restarting Codex. Tool visibility still changes only on process restart."),
		basemcp.WithString("path", basemcp.Description("Optional config path. Empty means the active config path.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configPathRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath(args.Path, cfg.SourcePath)
		loaded, err := reload(path)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"status": "ok", "config_path": path, "config": loaded})
	})
}

func registerConfigOptionTools(srv *server.MCPServer, deps *Server, reload configReloadFunc) {
	srv.AddTool(basemcp.NewTool("config_option_set",
		basemcp.WithDescription("Set one allowlisted scalar config option without replacing the whole YAML config. Preserves hidden token fields and reloads by default."),
		basemcp.WithString("path", basemcp.Required(), basemcp.Description("Allowlisted config option path, for example layers.tasks.enabled or command_policy.max_lines.")),
		basemcp.WithString("value", basemcp.Required(), basemcp.Description("Scalar value as string. Bool options accept true/false; int options accept decimal integers.")),
		basemcp.WithString("config_path", basemcp.Description("Optional config file path. Empty means the active config path.")),
		basemcp.WithBoolean("reload", basemcp.Description("Reload runtime after writing. Defaults to true.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configOptionSetRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath(args.ConfigPath, cfg.SourcePath)
		loaded, err := setConfigOption(path, args.Path, args.Value)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		reloadNow := args.Reload == nil || *args.Reload
		if reloadNow {
			loaded, err = reload(path)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
		}
		return structured(map[string]any{"status": "ok", "path": args.Path, "value": args.Value, "reloaded": reloadNow, "config_path": path, "config": loaded})
	})

	srv.AddTool(basemcp.NewTool("config_option_reset",
		basemcp.WithDescription("Reset one allowlisted optional config option without replacing the whole YAML config. Currently supports optional bool overrides."),
		basemcp.WithString("path", basemcp.Required(), basemcp.Description("Allowlisted optional config option path, for example layers.tasks.enabled or command_policy.log_enabled.")),
		basemcp.WithString("config_path", basemcp.Description("Optional config file path. Empty means the active config path.")),
		basemcp.WithBoolean("reload", basemcp.Description("Reload runtime after writing. Defaults to true.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configOptionResetRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath(args.ConfigPath, cfg.SourcePath)
		loaded, err := resetConfigOption(path, args.Path)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		reloadNow := args.Reload == nil || *args.Reload
		if reloadNow {
			loaded, err = reload(path)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
		}
		return structured(map[string]any{"status": "ok", "path": args.Path, "reset": true, "reloaded": reloadNow, "config_path": path, "config": loaded})
	})
}

func setConfigOption(path string, optionPath string, value string) (*config.Config, error) {
	if config.IsRepoConfigPath(path) {
		return nil, errors.New("repo config (.mcp-ai-helper.yaml) is user-editable only; use config_read with repo_path to inspect it")
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if err := applyConfigOption(cfg, strings.TrimSpace(optionPath), strings.TrimSpace(value)); err != nil {
		return nil, err
	}
	if err := validateConfigOptionValue(strings.TrimSpace(optionPath), strings.TrimSpace(value)); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return writeValidatedConfig(path, string(data))
}

func resetConfigOption(path string, optionPath string) (*config.Config, error) {
	if config.IsRepoConfigPath(path) {
		return nil, errors.New("repo config (.mcp-ai-helper.yaml) is user-editable only; use config_read with repo_path to inspect it")
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if err := resetOptionalConfigOption(cfg, strings.TrimSpace(optionPath)); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return writeValidatedConfig(path, string(data))
}

func applyConfigOption(cfg *config.Config, optionPath string, value string) error {
	switch optionPath {
	case "layers.logs.enabled":
		cfg.Layers.Logs.Enabled = boolValue(value)
	case "layers.tasks.enabled":
		cfg.Layers.Tasks.Enabled = boolValue(value)
	case "layers.issues.enabled":
		cfg.Layers.Issues.Enabled = boolValue(value)
	case "layers.guidance.enabled":
		cfg.Layers.Guidance.Enabled = boolValue(value)
	case "layers.models.enabled":
		cfg.Layers.Models.Enabled = boolValue(value)
	case "layers.commands.enabled":
		cfg.Layers.Commands.Enabled = boolValue(value)
	case "layers.workflows.enabled":
		cfg.Layers.Workflows.Enabled = boolValue(value)
	case "layers.reasoning_patterns.enabled":
		cfg.Layers.ReasoningPatterns.Enabled = boolValue(value)
	case "command_policy.default_timeout_seconds":
		cfg.CommandPolicy.DefaultTimeoutSeconds = positiveIntValue(optionPath, value)
	case "command_policy.max_output_bytes":
		cfg.CommandPolicy.MaxOutputBytes = positiveIntValue(optionPath, value)
	case "command_policy.max_lines":
		cfg.CommandPolicy.MaxLines = positiveIntValue(optionPath, value)
	case "command_policy.log_dir":
		cfg.CommandPolicy.LogDir = nonEmptyStringValue(optionPath, value)
	case "command_policy.log_enabled":
		cfg.CommandPolicy.LogEnabled = boolValue(value)
	case "command_policy.log_retention_days":
		cfg.CommandPolicy.LogRetentionDays = positiveIntValue(optionPath, value)
	case "command_policy.log_max_records":
		cfg.CommandPolicy.LogMaxRecords = positiveIntValue(optionPath, value)
	case "command_policy.log_compress":
		cfg.CommandPolicy.LogCompress = derefBool(boolValue(value))
	case "pipeline_policy.max_return_chars":
		cfg.PipelinePolicy.MaxReturnChars = positiveIntValue(optionPath, value)
	case "pipeline_policy.require_evidence_for_analysis":
		cfg.PipelinePolicy.RequireEvidenceForAnalysis = derefBool(boolValue(value))
	case "task_registry.backend":
		cfg.TaskRegistry.Backend = registryBackendValue(value)
	case "task_registry.obsidian.path":
		cfg.TaskRegistry.Obsidian.Path = nonEmptyStringValue(optionPath, value)
	case "task_registry.obsidian.vault":
		cfg.TaskRegistry.Obsidian.Vault = nonEmptyStringValue(optionPath, value)
	default:
		return fmt.Errorf("unsupported config option path %q; use config_schema for allowlisted options", optionPath)
	}
	return nil
}

func resetOptionalConfigOption(cfg *config.Config, optionPath string) error {
	switch optionPath {
	case "layers.logs.enabled":
		cfg.Layers.Logs.Enabled = nil
	case "layers.tasks.enabled":
		cfg.Layers.Tasks.Enabled = nil
	case "layers.issues.enabled":
		cfg.Layers.Issues.Enabled = nil
	case "layers.guidance.enabled":
		cfg.Layers.Guidance.Enabled = nil
	case "layers.models.enabled":
		cfg.Layers.Models.Enabled = nil
	case "layers.commands.enabled":
		cfg.Layers.Commands.Enabled = nil
	case "layers.workflows.enabled":
		cfg.Layers.Workflows.Enabled = nil
	case "layers.reasoning_patterns.enabled":
		cfg.Layers.ReasoningPatterns.Enabled = nil
	case "command_policy.log_enabled":
		cfg.CommandPolicy.LogEnabled = nil
	default:
		return fmt.Errorf("config option path %q cannot be reset; set an explicit value instead", optionPath)
	}
	return nil
}

func boolValue(value string) *bool {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func derefBool(value *bool) bool {
	return value != nil && *value
}

func positiveIntValue(optionPath string, value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func nonEmptyStringValue(optionPath string, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

func registryBackendValue(value string) string {
	return strings.TrimSpace(value)
}

func validateConfigOptionValue(optionPath string, value string) error {
	switch optionPath {
	case "layers.logs.enabled", "layers.tasks.enabled", "layers.issues.enabled", "layers.guidance.enabled", "layers.models.enabled", "layers.commands.enabled", "layers.workflows.enabled", "layers.reasoning_patterns.enabled", "command_policy.log_enabled", "command_policy.log_compress", "pipeline_policy.require_evidence_for_analysis":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("config option %s requires a boolean value", optionPath)
		}
	case "command_policy.default_timeout_seconds", "command_policy.max_output_bytes", "command_policy.max_lines", "command_policy.log_retention_days", "command_policy.log_max_records", "pipeline_policy.max_return_chars":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("config option %s requires a positive integer value", optionPath)
		}
	case "command_policy.log_dir", "task_registry.obsidian.path", "task_registry.obsidian.vault":
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("config option %s requires a non-empty string value", optionPath)
		}
	case "task_registry.backend":
		switch value {
		case "", "lean", "obsidian":
		default:
			return fmt.Errorf("config option %s must be lean or obsidian", optionPath)
		}
	}
	return nil
}

func effectiveConfigPath(requested string, active string) string {
	if strings.TrimSpace(requested) != "" {
		return requested
	}
	if strings.TrimSpace(active) != "" {
		return active
	}
	return config.DefaultConfigPath()
}

func writeValidatedConfig(path string, yamlText string) (*config.Config, error) {
	if strings.TrimSpace(yamlText) == "" {
		return nil, errors.New("config_yaml is required")
	}
	if strings.TrimSpace(path) == "" {
		path = config.DefaultConfigPath()
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open config directory root: %w", err)
	}
	defer func() { _ = root.Close() }()
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	tmpBase := filepath.Base(tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.WriteString(yamlText); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp config: %w", err)
	}
	loaded, err := config.Load(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	if existing, existingErr := loadExistingConfig(path); existingErr == nil && preserveRedactedConfigFields(existing, loaded, yamlText) {
		rewritten, err := yaml.Marshal(loaded)
		if err != nil {
			return nil, fmt.Errorf("preserve redacted config fields: %w", err)
		}
		if err := os.WriteFile(tmpPath, rewritten, 0o600); err != nil {
			return nil, fmt.Errorf("rewrite config with preserved redacted fields: %w", err)
		}
		loaded, err = config.Load(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("validate preserved config: %w", err)
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		src, readErr := root.ReadFile(tmpBase)
		if readErr != nil {
			return nil, fmt.Errorf("read temp config for copy: %w", readErr)
		}
		if writeErr := root.WriteFile(base, src, 0o600); writeErr != nil {
			return nil, fmt.Errorf("write config via copy: %w", writeErr)
		}
	}
	loaded.SourcePath = path
	return loaded, nil
}

func loadExistingConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return config.Load(path)
}

func preserveRedactedConfigFields(existing *config.Config, replacement *config.Config, yamlText string) bool {
	changed := false
	if !yamlHasTopLevelKey(yamlText, "secrets") && len(existing.Secrets) > 0 {
		replacement.Secrets = existing.Secrets
		changed = true
	}
	for id, oldProvider := range existing.Providers {
		newProvider, ok := replacement.Providers[id]
		if !ok || oldProvider.APIKey == "" || newProvider.APIKey != "" {
			continue
		}
		newProvider.APIKey = oldProvider.APIKey
		replacement.Providers[id] = newProvider
		changed = true
	}
	if preserveJiraRedactedFields(existing.Integrations.Jira, replacement.Integrations.Jira) {
		changed = true
	}
	if preserveConfluenceRedactedFields(existing.Integrations.Confluence, replacement.Integrations.Confluence) {
		changed = true
	}
	return changed
}

func preserveJiraRedactedFields(existing *config.JiraConfig, replacement *config.JiraConfig) bool {
	if existing == nil || replacement == nil {
		return false
	}
	changed := false
	if existing.APIKey != "" && replacement.APIKey == "" {
		replacement.APIKey = existing.APIKey
		changed = true
	}
	if existing.APIKeyEnv != "" && replacement.APIKeyEnv == "" {
		replacement.APIKeyEnv = existing.APIKeyEnv
		changed = true
	}
	return changed
}

func preserveConfluenceRedactedFields(existing *config.ConfluenceConfig, replacement *config.ConfluenceConfig) bool {
	if existing == nil || replacement == nil {
		return false
	}
	changed := false
	if existing.APIKey != "" && replacement.APIKey == "" {
		replacement.APIKey = existing.APIKey
		changed = true
	}
	if existing.APIKeyEnv != "" && replacement.APIKeyEnv == "" {
		replacement.APIKeyEnv = existing.APIKeyEnv
		changed = true
	}
	return changed
}

func yamlHasTopLevelKey(yamlText string, key string) bool {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &node); err != nil || len(node.Content) == 0 {
		return false
	}
	mapping := node.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return true
		}
	}
	return false
}
