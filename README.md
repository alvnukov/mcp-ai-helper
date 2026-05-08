# mcp-ai-helper

Go MCP server for delegating bounded work to third-party models and local deterministic tools.

The design goal is token economy without losing grounding:

- collect and filter large command output before model calls;
- route work to configured OpenAI-compatible models such as generic provider;
- keep model prompts and capabilities per model;
- validate that analysis points back to evidence;
- edit files only through guarded, idempotent operations;
- commit only explicitly owned files after checks pass.

## Run

```sh
go run ./cmd/mcp-ai-helper
```

On first run the server creates `~/.mcp-ai-helper/config.yaml` with safe local-command defaults, assistant guidance, retention settings, disabled production issues, and commented provider/model placeholders. For real model calls, add providers/models there or run with `--config ./configs/config.example.yaml`.

## MCP tools

- `config_schema`
- `config_read`
- `config_replace`
- `config_reload`
- `language_profiles`
- `language_detect`
- `list_models`
- `assistant_guidance`
- `server_setup_guidance`
- `query_model`
- `collect_command_output`
- `filter_command_history`
- `run_pipeline`
- `run_workflow`
- `snapshot_file`
- `apply_guarded_replace`
- `git_commit_owned`
- `task_add`
- `task_update`
- `task_set_status`
- `task_batch_upsert`
- `task_search`
- `task_list`
- `task_current`
- `task_get`
- `task_delete`

The server is intentionally policy-first. Local tools require `repo_path` from the caller; command `cwd` and file `path` are interpreted as repo-relative where applicable. It refuses unsafe command working directories, hash-mismatched file edits, repo path escapes, and broad git staging.

On discovery, clients should read `assistant_guidance`, the `mcp-ai-helper://guidance` resource, or the `mcp-ai-helper-guidance` prompt. They publish the workflow-first operating rules from `~/.mcp-ai-helper/config.yaml`. Use `server_setup_guidance` to learn how to configure the server.

Models can configure the helper without a restart: call `config_schema` to understand every field, `config_read` to inspect the sanitized active config, `config_replace` to validate and atomically write a complete YAML config, and `config_reload` after external edits. `config_replace` reloads runtime clients by default. Tool visibility still changes on process restart because MCP clients discover tools at session startup.

Language profiles give callers deterministic guardrails before code edits. The built-in Go profile tells the model to run `gofmt` only on `.go` files, prefer targeted `go test <affected_packages>` before `go test ./...`, run `go vet ./...`, and treat missing imports or undefined symbols as compile blockers. Use `language_detect` with owned files when constructing a workflow.

`run_pipeline` collapses successful command output by default: callers get only `status`, `command_id`, `exit_code`, and a short handoff. Set `compact_output=false` or use `filter_command_history` with `command_id` when details are needed. Failed commands keep relevant error details.

`run_workflow` is the preferred tool for code work. The caller sends the whole task in one request: guarded text edits, checks, and optional commit. The workflow stops before commit on edit conflicts or failed checks.

`run_workflow` also accepts a stable `steps` DSL so future workflow improvements do not require changing the MCP schema. Supported step tools today: `guarded_replace`, `run_command`, `git_commit_owned`. Supported deterministic conditions today: `always`, `changed_files_count > 0`, and `steps.<id>.status == <status>`.

Callers should use one long workflow when intermediate results are not needed by the calling model. A single workflow should include command execution, output filters, conditional branches, file edits, checks, and commit. Low-level tools are for bootstrapping and cases where a result must change the caller's next decision.

Command output is retained under `~/.mcp-ai-helper/repos/<project>/logs` by default. Each execution gets a `command_id`, an index entry, and a bounded record file so callers can later use `filter_command_history` with a more precise filter instead of rerunning the command or flooding context. Retention is controlled by `command_policy.log_retention_days`, `log_max_records`, and `log_compress`.

Project tasks are retained under `~/.mcp-ai-helper/repos/<project>/tasks` as Lean files with structured metadata in a Lean comment. Agents can add, list, read, delete, and request current tasks without scanning repo files or replaying previous logs.

## Production usage

Build and run the server binary directly:

```sh
go build -o bin/mcp-ai-helper ./cmd/mcp-ai-helper
bin/mcp-ai-helper
```

By default the server creates and reads `~/.mcp-ai-helper/config.yaml`. Use `--config` only for an explicit override.
