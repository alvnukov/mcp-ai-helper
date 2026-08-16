# mcp-ai-helper

Go MCP server for delegating bounded work to third-party models and local deterministic tools.

The design goal is token economy without losing grounding:

- collect and filter large command output before model calls;
- route work to configured OpenAI-compatible models such as generic provider;
- keep model prompts and capabilities per model;
- validate that analysis points back to evidence;
- edit files only through guarded, idempotent operations;
- commit only explicitly owned files after checks pass.

## Install

The supported package install uses the project Homebrew tap:

```sh
brew install alvnukov/tap/mcp-ai-helper
mcp-ai-helper --version
```

Release archives and checksums for Linux, macOS, and Windows are attached to
each [GitHub Release](https://github.com/alvnukov/mcp-ai-helper/releases).
See [RELEASE.md](RELEASE.md) for the tag and Homebrew publication contract.

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

### Self-install

The binary registers itself, so the per-client stanzas below are reference rather than work:

```sh
bin/mcp-ai-helper setup -c claude,codex,opencode
bin/mcp-ai-helper status -c claude,codex,opencode   # exits 1 if anything is missing or stale
bin/mcp-ai-helper remove -c claude,codex,opencode   # alias: uninstall
```

Each client gets three things: the MCP server entry, an `mcp-ai-helper` block in the file it reads for instructions (`CLAUDE.md` for Claude Code, `AGENTS.md` for Codex and opencode), and five focused skills: `mcp-ai-helper-tasks`, `mcp-ai-helper-edits`, `mcp-ai-helper-commands`, `mcp-ai-helper-web`, and `mcp-ai-helper-surface`. Their text lives in `internal/setup/skills/` and is embedded into the binary, so the markdown a model reads can be reviewed as markdown; adding a skill is a directory plus one line in `skillNames`. Every skill includes Codex UI metadata. Codex installs them user-wide under `~/.codex/skills`; Claude Code and OpenCode use their documented project/global skill roots. `remove` takes back exactly those helper-owned files, leaving other servers, skills, and instructions alone.

| Flag | Effect |
|---|---|
| `-c`, `--clients` | Clients to act on: `claude`, `codex`, `opencode`. Comma-separated or repeated. Required. |
| `--global` | Write the user-wide config instead of the project one. Codex is user-wide either way. |
| `--dry-run` | Report what would change without writing it. |
| `--no-instructions` | Leave `CLAUDE.md` / `AGENTS.md` alone. |
| `--no-skills` | Leave the skill files alone. |
| `--config PATH` | Pin `--config PATH` in the command line the client runs. Omit it to let the server fall back to `~/.mcp-ai-helper/config.yaml`. |

Both commands are idempotent: every write is compared against what is on disk first, so a re-run reports `already up to date` rather than reformatting a config or duplicating a block. The command registered is the absolute path of the binary you invoked, which is what makes it work from a client launched by a GUI with a thin `PATH`. For repo development, run `setup` from `bin/mcp-ai-helper-dev` — it registers itself the same way, wrapper arguments included.

`status` writes nothing. It re-derives what `setup` would write, compares that against what is on disk, and exits 1 when anything is missing or out of date — including a server entry naming a binary that has since moved, which from inside the client is indistinguishable from one that works. It takes the same `-c`, `--global`, `--no-instructions` and `--no-skills` flags, which makes it usable as a check rather than as something somebody remembers to read.

Two caveats worth knowing before the first run. Re-registering preserves per-entry settings you added yourself, such as Codex `approval_mode` under `[mcp_servers.mcp-ai-helper.tools.*]`, but the TOML and JSON files are rewritten through a parser, so comments and key order in them do not survive. And a config left holding nothing but the helper is deleted rather than left as an empty husk.

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

Six action-dispatch tools carry the everyday surface. Each takes an `action`, and
the tool's own schema lists the actions it accepts — that list is generated from
the handlers, so it cannot drift from what the server will answer.

| Tool | Actions |
| --- | --- |
| `file` | `read`, `read_many`, `list`, `search`, `snapshot` |
| `edit` | `replace`, `write` |
| `command` | `run`, `get`, `filter`, `list`, `abort`, `cleanup`, `health` |
| `git` | `status`, `diff`, `commit`; with the `git_advanced` layer also `log`, `log_diff`, `blame`, `stash_list`, `branch_list`, `remote_list`, `tag_list`, `prepare_task_worktree` |
| `task` | `current`, `get`, `list`, `search`, `upsert`, `set_status`, `batch_upsert`, `delete` |
| `run` | `pipeline`, `workflow`, `schema` |

Alongside them:

- guidance — `assistant_guidance`, `server_setup_guidance`, `tool_manifest`
- config — `config_schema`, `config_read`, `config_option_set`, `config_option_reset`, `config_reload`
- models — `list_models`, `query_model`
- language — `language_profiles`, `language_detect`
- planning — `plan_task_execution`, `task_packet`, `reasoning_patterns`
- health — `health`
- task registry — `task_registry_init`

And behind opt-in layers:

- `task_advanced` — `task_graph`, `task_context`, `task_export`
- `task_ui` — `task_ui_start`, `task_ui_stop`
- `web` — `web_search`, `web_fetch`, `fetched_doc_find`, `fetched_doc_read`
- `issues` — `issue_add`, `issue_list`, `issue_accept`
- `lake` — `lake_smoke`, `lake_init`
- `config_advanced` — `feature_list`, `feature_get`, `feature_enable`, `feature_disable`, `feature_reset`
- `jira` / Confluence integrations — `jira_*`, `conf_*`

Call `tool_manifest` for the surface the running server actually exposes; a
hardcoded list in a prompt goes stale, this one does not.

The server is intentionally policy-first. Local tools require `repo_path` from the caller; command `cwd` and file `path` are interpreted as repo-relative where applicable. It refuses unsafe command working directories, hash-mismatched file edits, repo path escapes, and broad git staging.

`task action=upsert` requires a title and accepts an optional ID. When ID is omitted, the backend derives a normalized filesystem-safe ID from the title. For an existing task, omitted optional fields preserve their stored values; an explicit empty array clears a list field. Empty scalar strings are treated as omitted, so scalar clearing needs a dedicated mutation rather than a partial upsert.

On discovery, clients should read `assistant_guidance`, the `mcp-ai-helper://guidance` resource, or the `mcp-ai-helper-guidance` prompt. They publish the workflow-first operating rules from `~/.mcp-ai-helper/config.yaml`. Use `server_setup_guidance` to learn how to configure the server.

When `layers.issues.enabled` is changed from false to true via `config_option_set`, runtime config is reloaded immediately, but newly visible MCP tools such as `issue_add` require MCP client rediscovery or restart if they were hidden at process startup. Keep issues enabled in dev config when feedback intake is expected.

Models can configure the helper without a restart: call `config_schema` to understand every field, `config_read` to inspect the sanitized active config, `config_option_set` and `config_option_reset` to change one option at a time, and `config_reload` after external edits. Tool visibility still changes on process restart because MCP clients discover tools at session startup.

Language profiles give callers deterministic guardrails before code edits. The built-in Go profile tells the model to run `gofmt` only on files whose extension is exactly `.go`, prefer targeted `go test <affected_packages>` before `go test ./...`, run `go vet ./...`, and treat missing imports or undefined symbols as compile blockers. Use `language_detect` with owned files when constructing a workflow.

`run action=pipeline` collapses successful command output by default: callers get only `status`, `command_id`, `exit_code`, and a short handoff. Set `compact_output=false` or use `command action=filter` with `command_id` when details are needed. Failed commands keep relevant error details.

`run action=workflow` is the preferred tool for code work. The caller sends the whole task in one request: guarded text edits, checks, task transitions, and optional commit. The workflow stops before commit on edit conflicts or failed checks.

`run action=workflow` also accepts a stable `steps` DSL so future workflow improvements do not require changing the MCP schema. Supported step tools today include `guarded_replace`, `write_file`, `command`, `task_transition`, `task_batch_upsert`, and `git_commit_owned`. `write_file` creates or replaces a whole file with the same content/base64 and optional hash-guard semantics as `edit action=write`. Supported deterministic conditions include `always`, command status or exit code checks such as `steps.check.status == ok`, output checks such as `steps.probe.output_contains text`, file state checks such as `file_exists path`, task status checks such as `tasks.task-024.status == todo`, and changed-file checks.

Callers should use one long workflow when intermediate results are not needed by the calling model. A single workflow should include command execution, output filters, conditional branches, file edits, focused checks, task status transitions, and commit. Low-level tools are for bootstrapping and cases where a result must change the caller's next decision.

### Canonical workflow examples

Before an implementation workflow, gather only the context that can change the decision: `task action=current`, targeted `file action=read` ranges, `file action=snapshot` for owned files, and narrow probes such as `rg` or a focused test. Then state the decision in the calling turn: selected task, owned files, forbidden files, acceptance criteria, and the gate that proves closure. Do not build an editing workflow while the contract or owned files are still unclear.

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
        "expected_hash": "<hash from file action=snapshot>",
        "old": "old unique span",
        "new": "new unique span"
      }
    },
    {
      "id": "check",
      "tool": "command",
      "depends_on": ["edit"],
      "args": {
        "command": "go test ./internal/example",
        "cwd": "."
      }
    },
    {
      "id": "done",
      "tool": "task_transition",
      "depends_on": ["check"],
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
      "depends_on": ["done"],
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
    { "id": "check", "tool": "command", "depends_on": ["edit"], "args": { "command": "go test ./internal/example", "cwd": "." } },
    {
      "id": "block",
      "tool": "task_transition",
      "depends_on": ["check"],
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

Command output is retained under `~/.mcp-ai-helper/repos/<project>/logs` by default. Each execution gets a `command_id`, an index entry, and a bounded record file so callers can later use `command action=filter` with a more precise filter instead of rerunning the command or flooding context. Retention is controlled by `command_policy.log_retention_days`, `log_max_records`, and `log_compress`.

Three fields exist to keep a caller from spending turns on work it has already done or on a result it should not trust:

- `previous` appears when the same command already ran in that repository within the hour, and carries the earlier `command_id`, its exit code, its age, and `same_output` — set when the two runs produced byte-identical output.
- `failure_markers` appears when the output reports a failure the exit code does not. A command exits with the status of the last stage of its pipeline, so `go test ./... | tail -40` reports success whatever the tests did.
- `command action=get` takes `wait_seconds` and blocks until a running command finishes, so waiting does not mean sleeping inside a shell command. An exhausted wait returns the running record rather than an error.

Writing repository source through a shell command — `apply_patch`, a redirect or `tee` into a source file, `sed -i` — is refused, because it bypasses the snapshot and `expected_hash` that make an edit safe to retry. Use `file action=snapshot` with `edit action=replace`, or `edit action=write` for a new file.

Task state is scoped by the caller's `repo_path`. The helper merges the global config with that repository's `.mcp-ai-helper.yaml`, then opens the configured canonical backend. This checkout uses the Obsidian backend at `obsidian-tasks/`; another repository may select its own supported backend and path. Backend diagnostics are returned as `source`/`projection_source` metadata and must not be bypassed with direct file edits.

For local development, point MCP clients at the stable wrapper instead of the raw server:

```sh
bin/mcp-ai-helper-dev --repo /path/to/default/repository --config ~/.mcp-ai-helper/config.yaml
```

The wrapper keeps stdio alive while it rebuilds or restarts the child server through `dev_rebuild_server` and `dev_restart_server`. The startup `--repo` value is only the default working repository; every repository-scoped tool call still uses its explicit `repo_path`.

## Repository-scoped task workflow

Initialize a repository once with `task_registry_init` when it has no configured registry. Thereafter, MCP callers inspect work with `task action=current` and `task action=get`, and mutate it with `task action=set_status`, `task action=upsert`, `task action=batch_upsert`, or `task action=delete`. Read and mutation calls always resolve the registry from the supplied `repo_path`; missing or invalid registry configuration is a blocker, not permission to fall back to stale task files.

## Production usage

Build and run the server binary directly:

```sh
go build -o bin/mcp-ai-helper ./cmd/mcp-ai-helper
bin/mcp-ai-helper
```

By default the server creates and reads `~/.mcp-ai-helper/config.yaml`. Use `--config` only for an explicit override.
