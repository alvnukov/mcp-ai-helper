// Package config exposes machine-readable configuration documentation.
package config

// FieldDoc describes one configuration field for model-driven setup.
type FieldDoc struct {
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Required    bool     `json:"required"`
	Description string   `json:"description"`
	Examples    []string `json:"examples,omitempty"`
}

// Schema returns compact, machine-readable documentation for every supported config area.
func Schema() map[string]any {
	return map[string]any{
		"config_path": DefaultConfigPath(),
		"workflow": []string{
			"Call config_schema before editing config when field meaning is unclear.",
			"Use config_read to inspect the active sanitized config.",
			"Use config_option_set/config_option_reset for allowlisted scalar option edits; prefer them over full YAML replacement.",
			"Use config_replace only when a full validated YAML replacement is explicitly required; it preserves hidden token fields where possible.",
			"Use config_reload after an external config edit; config_option_set and config_replace reload by default.",
			"Use language_profiles before code edits so format/test/lint commands are selected by language instead of ad hoc shell habits.",
		},
		"fields": []FieldDoc{
			{Path: "assistant_guidance", Type: "string", Default: "built-in workflow-first guidance", Description: "Instructions returned to calling models. Keep it short, mandatory, and focused on one-pipeline operation, task hygiene, output filtering, and safety."},
			{Path: "layers.logs.enabled", Type: "bool", Default: "true", Description: "Enables command history retention and log filtering tools."},
			{Path: "layers.tasks.enabled", Type: "bool", Default: "true", Description: "Enables per-repository task memory tools."},
			{Path: "layers.issues.enabled", Type: "bool", Default: "false in generated config", Description: "Enables cross-repository feedback/issue intake. Keep disabled in production unless this machine is intentionally accepting development feedback."},
			{Path: "layers.guidance.enabled", Type: "bool", Default: "true", Description: "Enables guidance resources and prompts."},
			{Path: "layers.models.enabled", Type: "bool", Default: "true", Description: "Enables model listing and remote model query tools."},
			{Path: "layers.commands.enabled", Type: "bool", Default: "true", Description: "Enables local command execution, output filtering, and command history."},
			{Path: "layers.workflows.enabled", Type: "bool", Default: "true", Description: "Enables multi-step repo workflows with guarded edits, checks, task updates, and owned-file commits."},
			{Path: "layers.reasoning_patterns.enabled", Type: "bool", Default: "false in generated config", Description: "Legacy planning layer flag. Canonical startup tools are task_graph and task_context."},
			{Path: "language_profiles", Type: "built-in registry", Default: "go", Description: "Language-aware guardrails used by callers before code edits: file matching, formatter, targeted tests, broad tests, static checks, and common safety rules."},
			{Path: "providers.<id>.type", Type: "string", Default: "generic", Description: "Provider adapter type: generic for OpenAI-compatible providers, anthropic for Anthropic Messages API.", Examples: []string{"generic", "anthropic"}},
			{Path: "providers.<id>.base_url", Type: "string", Description: "OpenAI-compatible base URL used to derive /chat/completions when completions_url is not set."},
			{Path: "providers.<id>.completions_url", Type: "string", Description: "Explicit chat completions endpoint. Use when the provider does not follow the standard base_url layout."},
			{Path: "providers.<id>.api_key_env", Type: "string", Description: "Environment variable containing the provider API key. Prefer this over literal api_key."},
			{Path: "providers.<id>.api_key", Type: "string", Description: "Literal provider API key. Supported but not recommended; never return it in summaries."},
			{Path: "providers.<id>.app_name", Type: "string", Description: "Optional application name/header value for providers that require attribution."},
			{Path: "providers.<id>.timeout_seconds", Type: "int", Default: "120", Description: "HTTP timeout for remote model calls."},
			{Path: "providers.<id>.max_retries", Type: "int", Default: "0", Description: "Retry count for transient remote model failures."},
			{Path: "models.<id>.provider", Type: "string", Required: true, Description: "Provider id from providers."},
			{Path: "models.<id>.model", Type: "string", Required: true, Description: "Provider model name."},
			{Path: "models.<id>.tier", Type: "string", Description: "Cost/strength tier used by routing policy and humans.", Examples: []string{"low", "medium", "high"}},
			{Path: "models.<id>.roles", Type: "[]string", Description: "Intended roles such as summarizer, classifier, routine_debug, code_reasoning, verification."},
			{Path: "models.<id>.purpose", Type: "string", Description: "Short explanation of when a caller should use this model."},
			{Path: "models.<id>.system_prompt", Type: "string", Description: "Per-model prompt tuned to its capabilities and weaknesses."},
			{Path: "models.<id>.system_prompt_file", Type: "string", Description: "Optional path to a prompt file for larger prompts."},
			{Path: "models.<id>.max_input_chars", Type: "int", Description: "Input budget guard before sending data to this model."},
			{Path: "models.<id>.max_output_tokens", Type: "int", Description: "Maximum remote model output tokens."},
			{Path: "models.<id>.temperature", Type: "float", Default: "0", Description: "Sampling temperature. Use 0 for grounded analysis and verification."},
			{Path: "models.<id>.capabilities", Type: "map", Description: "Machine-readable capability hints, for example json/code/reasoning/context/window/tool_use."},
			{Path: "routing.query_default", Type: "string", Description: "Default model id for general query_model calls."},
			{Path: "routing.log_summary", Type: "string", Description: "Model id for cheap log and command-output summarization."},
			{Path: "routing.evidence_analysis", Type: "string", Description: "Model id for stronger evidence-based analysis."},
			{Path: "command_policy.default_timeout_seconds", Type: "int", Default: "300", Description: "Default timeout for local commands, in seconds."},
			{Path: "command_policy.max_output_bytes", Type: "int", Default: "200000", Description: "Maximum retained command output bytes before truncation/filtering."},
			{Path: "command_policy.max_lines", Type: "int", Default: "400", Description: "Maximum returned output lines."},
			{Path: "command_policy.allowed_cwds", Type: "[]string", Default: ".", Description: "Allowed command working directories. Use repo-relative entries for normal repo work."},
			{Path: "command_policy.log_dir", Type: "string", Default: "~/.mcp-ai-helper/repos", Description: "Root for per-repository logs and task memory."},
			{Path: "command_policy.log_enabled", Type: "bool", Default: "layers.logs.enabled", Description: "Enables command log persistence."},
			{Path: "command_policy.log_retention_days", Type: "int", Default: "30", Description: "Age-based retention for command logs."},
			{Path: "command_policy.log_max_records", Type: "int", Default: "2000", Description: "Count-based retention for command history records."},
			{Path: "command_policy.log_compress", Type: "bool", Default: "true", Description: "Allows old command records to be compressed by retention jobs."},
			{Path: "pipeline_policy.max_return_chars", Type: "int", Default: "4000", Description: "Maximum compact result size returned from pipelines."},
			{Path: "pipeline_policy.require_evidence_for_analysis", Type: "bool", Default: "true", Description: "Requires model conclusions to cite extracted evidence lines."},
			{Path: "web_policy.enabled", Type: "bool", Default: "true", Description: "Enables bounded web_fetch network access. Disable to fail closed."},
			{Path: "web_policy.cache_dir", Type: "string", Default: "~/.mcp-ai-helper/web", Description: "Helper-managed cache for raw source bytes, normalized text, and metadata artifacts."},
			{Path: "web_policy.max_source_bytes", Type: "int", Default: "1048576", Description: "Maximum accepted source body bytes. Larger responses return incomplete diagnostics, not complete artifacts."},
			{Path: "web_policy.timeout_seconds", Type: "int", Default: "600", Description: "HTTP timeout for bounded fetch and search requests, in seconds."},
			{Path: "web_policy.max_redirects", Type: "int", Default: "5", Description: "Maximum redirect count; redirects are validated against the same URL policy."},
			{Path: "web_policy.allowed_schemes", Type: "[]string", Default: "https,http", Description: "Allowed URL schemes for web_fetch."},
			{Path: "web_policy.allowed_hosts", Type: "[]string", Description: "Optional exact host allowlist. Empty allows public hosts subject to deny checks."},
			{Path: "web_policy.denied_hosts", Type: "[]string", Description: "Exact host denylist checked before fetching."},
			{Path: "web_policy.accepted_content_types", Type: "[]string", Default: "text/html,text/plain,application/json,application/xml,text/", Description: "Accepted response content types. Unsupported types fail closed."},
			{Path: "web_policy.user_agent", Type: "string", Default: "mcp-ai-helper/0.1", Description: "User-Agent used for web_fetch requests."},
			{Path: "web_policy.search_provider", Type: "string", Description: "Default explicit web_search provider. Empty keeps web_search fail-closed unless the call passes provider. Supported: duckduckgo_html, google_cse.", Examples: []string{"duckduckgo_html", "google_cse"}},
			{Path: "web_policy.search_url", Type: "string", Default: "https://html.duckduckgo.com/html/", Description: "Provider endpoint for duckduckgo_html. Search calls add q=<query> and return compact hits only."},
			{Path: "web_policy.max_search_results", Type: "int", Default: "10", Description: "Maximum compact web_search hits returned by default. Calls may request fewer results; hard cap is enforced by the provider adapter."},
			{Path: "web_policy.google_cse_id", Type: "string", Description: "Google Programmable Search Engine id (cx) required for provider google_cse."},
			{Path: "web_policy.google_api_key_env", Type: "string", Default: "GOOGLE_CSE_API_KEY in generated config", Description: "Environment variable containing the Google Custom Search JSON API key. Prefer this over literal google_api_key."},
			{Path: "web_policy.google_api_key", Type: "string", Description: "Literal Google API key for provider google_cse. Supported but not recommended; omitted from config JSON output."},
			{Path: "web_policy.google_cse_url", Type: "string", Default: "https://www.googleapis.com/customsearch/v1", Description: "Google Custom Search JSON API endpoint. Override only for tests or compatible proxies."},
			{Path: "secrets.<handle>.value", Type: "string", Description: "Server-side secret value. It is never serialized to model-facing config JSON; models may reference only the handle through secret_handles."},
			{Path: "secrets.<handle>.enabled", Type: "bool", Default: "false", Description: "Enables a named secret handle for run_pipeline/run_workflow injection. Disabled or unknown handles fail closed before command execution."},
			{Path: "features.overrides.<id>.enabled", Type: "bool", Description: "Feature override value. Global overrides live in helper-owned feature state; repo-local overrides live in .mcp-ai-helper.yaml and take precedence over global overrides."},
			{Path: "features.audit", Type: "[]record", Description: "Compact helper-owned audit trail for feature changes: scope, previous value/source, new value/source, timestamp, and optional reason."},
			{Path: "permissions.tools.deny", Type: "[]string", Description: "Repo-local only (.mcp-ai-helper.yaml). MCP tool names denied for LLM in this repo. User-editable; config_replace refuses to write repo configs."},
			{Path: "task_registry.obsidian.path", Type: "string", Description: "Obsidian task registry directory. In repo-local config this should be repo-relative; runtime resolves it against repo_path without exposing the absolute path in config_read."},
			{Path: "repo_config", Type: "file", Default: ".mcp-ai-helper.yaml (optional)", Description: "Per-repository config merged on top of global config. Feature tools may create/update only the features section and ensure this file is gitignored; config_replace refuses to write repo configs."},
		},
	}
}
