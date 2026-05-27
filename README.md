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

## MCP client setup

`mcp-ai-helper` is a stdio MCP server. Configure your client to start the helper command directly; do not run it as a long-lived HTTP service.

Build the production binary first:

```sh
go build -o bin/mcp-ai-helper ./cmd/mcp-ai-helper
```

Use `CONFIG_PATH` below for the helper config file described in the `Run` section.

Production command:

```sh
/path/to/mcp-ai-helper/bin/mcp-ai-helper --config CONFIG_PATH
```

For local development of this repository, use the stable wrapper instead:

```sh
/path/to/mcp-ai-helper/bin/mcp-ai-helper-dev --repo /path/to/mcp-ai-helper --config CONFIG_PATH
```

After connecting, restart or rediscover MCP tools in the client and call `server_setup_guidance`, then `assistant_guidance`, then `list_models`. If a tool layer is enabled or disabled in config, restart the MCP client because tool visibility is discovered at session startup.

### opencode

Add the server to `opencode.json` or the opencode config file you use for the project:

```json
{
  "mcp": {
    "mcp-ai-helper": {
      "type": "local",
      "enabled": true,
      "command": [
        "/path/to/mcp-ai-helper/bin/mcp-ai-helper",
        "--config",
        "CONFIG_PATH"
      ]
    }
  }
}
```

For repo development, replace `command` with:

```json
[
  "/path/to/mcp-ai-helper/bin/mcp-ai-helper-dev",
  "--repo",
  "/path/to/mcp-ai-helper",
  "--config",
  "CONFIG_PATH"
]
```

### Codex

Add the server to `~/.codex/config.toml`:

```toml
[mcp_servers.mcp-ai-helper]
command = "/path/to/mcp-ai-helper/bin/mcp-ai-helper"
args = ["--config", "CONFIG_PATH"]
```

For repo development, use the wrapper command:

```toml
[mcp_servers.mcp-ai-helper]
command = "/path/to/mcp-ai-helper/bin/mcp-ai-helper-dev"
args = ["--repo", "/path/to/mcp-ai-helper", "--config", "CONFIG_PATH"]
```

### Claude Code

Register the server with the Claude Code MCP command:

```sh
claude mcp add mcp-ai-helper /path/to/mcp-ai-helper/bin/mcp-ai-helper --config CONFIG_PATH
```

For repo development, register the wrapper instead:

```sh
claude mcp add mcp-ai-helper /path/to/mcp-ai-helper/bin/mcp-ai-helper-dev --repo /path/to/mcp-ai-helper --config CONFIG_PATH
```

If your Claude Code version requires a separator before server args, put `--` before the helper command arguments. Example:

```sh
claude mcp add mcp-ai-helper /path/to/mcp-ai-helper/bin/mcp-ai-helper -- --config CONFIG_PATH
```

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

When `layers.issues.enabled` is changed from false to true via `config_replace`, runtime config is reloaded immediately, but newly visible MCP tools such as `issue_add` require MCP client rediscovery/restart if they were hidden at process startup. Keep issues enabled in dev config when feedback intake is expected.

Models can configure the helper without a restart: call `config_schema` to understand every field, `config_read` to inspect the sanitized active config, `config_replace` to validate and atomically write a complete YAML config, and `config_reload` after external edits. `config_replace` reloads runtime clients by default. Tool visibility still changes on process restart because MCP clients discover tools at session startup.

Language profiles give callers deterministic guardrails before code edits. The built-in Go profile tells the model to run `gofmt` only on files whose extension is exactly `.go`, prefer targeted `go test <affected_packages>` before `go test ./...`, run `go vet ./...`, and treat missing imports or undefined symbols as compile blockers. Use `language_detect` with owned files when constructing a workflow.

`run_pipeline` collapses successful command output by default: callers get only `status`, `command_id`, `exit_code`, and a short handoff. Set `compact_output=false` or use `filter_command_history` with `command_id` when details are needed. Failed commands keep relevant error details.

`run_workflow` is the preferred tool for code work. The caller sends the whole task in one request: guarded text edits, checks, task transitions, and optional commit. The workflow stops before commit on edit conflicts or failed checks.

`run_workflow` also accepts a stable `steps` DSL so future workflow improvements do not require changing the MCP schema. Supported step tools today include `guarded_replace`, `command`, `task_transition`, `task_batch_upsert`, and `git_commit_owned`. Supported deterministic conditions include `always`, command status or exit code checks such as `steps.check.status == ok`, output checks such as `steps.probe.output_contains text`, file state checks such as `file_exists path`, task status checks such as `tasks.task-024.status == todo`, and changed-file checks.

Callers should use one long workflow when intermediate results are not needed by the calling model. A single workflow should include command execution, output filters, conditional branches, file edits, focused checks, task status transitions, and commit. Low-level tools are for bootstrapping and cases where a result must change the caller's next decision.

### Canonical workflow examples

Before an implementation workflow, gather only the context that can change the decision: `task_current`, targeted `read_file` ranges, `snapshot_file` for owned files, and narrow probes such as `rg` or a focused test. Then state the decision in the calling turn: selected task, owned files, forbidden files, acceptance criteria, and the gate that proves closure. Do not build an editing workflow while the contract or owned files are still unclear.

Successful edit-check-task-done flow:

```json
{
  "repo_path": "/repo",
  "owned_files": ["internal/example.go"],
  "steps": [
    {
      "id": "edit",
      "tool": "guarded_replace",
      "args": {
        "path": "internal/example.go",
        "expected_hash": "<snapshot_file hash>",
        "old": "old unique span",
        "new": "new unique span"
      }
    },
    {
      "id": "check",
      "tool": "command",
      "args": {
        "command": "go test ./internal/example",
        "cwd": "."
      }
    },
    {
      "id": "done",
      "tool": "task_transition",
      "if": "steps.check.status == ok",
      "args": {
        "task_ids": ["task-123"],
        "from": "in_progress",
        "to": "done"
      }
    },
    {
      "id": "commit",
      "tool": "git_commit_owned",
      "if": "steps.done.status == ok",
      "args": {
        "files": ["internal/example.go"],
        "message": "Fix example task"
      }
    }
  ]
}
```

Failed-check path:

```json
{
  "repo_path": "/repo",
  "owned_files": ["internal/example.go"],
  "steps": [
    { "id": "edit", "tool": "guarded_replace", "args": { "path": "internal/example.go", "expected_hash": "<hash>", "old": "old", "new": "new" } },
    { "id": "check", "tool": "command", "args": { "command": "go test ./internal/example", "cwd": "." } },
    {
      "id": "block",
      "tool": "task_transition",
      "if": "steps.check.status != ok",
      "args": {
        "task_ids": ["task-123"],
        "from": "in_progress",
        "to": "blocked"
      }
    }
  ]
}
```

The failed path intentionally has no commit step. A repo task is not `done` until the acceptance criteria, the relevant gate, and the required owned-files commit have all passed.

Conditional probe with expected failure:

```json
{
  "repo_path": "/repo",
  "steps": [
    {
      "id": "probe",
      "tool": "command",
      "on_failure": "continue",
      "args": {
        "command": "rg -n \"featureFlag\" internal config | sed -n '1,40p'",
        "cwd": "."
      }
    },
    {
      "id": "fallback-check",
      "tool": "command",
      "if": "steps.probe.exit_code != 0",
      "args": {
        "command": "go test ./internal/config",
        "cwd": "."
      }
    }
  ]
}
```

Use `on_failure=continue` only for probes where a non-zero exit is part of the decision tree. Required gates should fail the workflow normally.

Do not use `close_missing` in task batches unless the caller already has the complete authoritative task set for that repository. Do not set a task to `done` from a documentation-only review, partial green test, skipped check, missing commit, failed commit, or fallback read from stale task storage. For repo tasks with file changes, no owned-files commit means the task is not done. Keep command output compact: prefer focused tests and filtered probes over whole-project tests or raw logs unless the changed surface creates a concrete regression risk.

Command output is retained under `~/.mcp-ai-helper/repos/<project>/logs` by default. Each execution gets a `command_id`, an index entry, and a bounded record file so callers can later use `filter_command_history` with a more precise filter instead of rerunning the command or flooding context. Retention is controlled by `command_policy.log_retention_days`, `log_max_records`, and `log_compress`.

For this repository, project task state is canonical in the Lean/Lake registry under `MCPAIHelperProject/`. The task read and mutation tools require the Lean exporter and expose `source`/`projection_source` diagnostics. Legacy `tasks/*.lean` JSON-comment files are not fallback storage and must not be treated as active state.

For local development in this repository, point MCP clients at the stable wrapper instead of the raw server:

```sh
bin/mcp-ai-helper-dev --repo /path/to/mcp-ai-helper --config ~/.mcp-ai-helper/config.yaml
```

The wrapper keeps stdio alive while it rebuilds or restarts the child server through `dev_rebuild_server` and `dev_restart_server`.

## Lean-backed task workflow

For this repository, the canonical task state is the Lean/Lake registry in `MCPAIHelperProject/`. A new contributor should verify the task layer before changing backlog state:

```sh
lake build
lake exe task_registry_export --list-active
lake exe task_registry_export --get task-042
```

MCP callers should inspect work with `task_current`/`task_get`, update work with `task_set_status`, `task_upsert`, `task_batch_upsert`, or `task_delete`, then rerun `lake build`. These tools use the Lean registry and expose `source`/`projection_source` diagnostics. Exporter or validation failures are blockers, not permission to read stale legacy task files.

## Production usage

Build and run the server binary directly:

```sh
go build -o bin/mcp-ai-helper ./cmd/mcp-ai-helper
bin/mcp-ai-helper
```

By default the server creates and reads `~/.mcp-ai-helper/config.yaml`. Use `--config` only for an explicit override.
