# Roadmap

This file tracks concrete improvements needed to make `mcp-ai-helper` a reliable workflow executor for senior LLMs.

## Priority 1: Output Selection

- Add precise filter presets for common tools: `go_test`, `golangci`, `go_vet`, `pytest`, `ruff`, `mypy`, `git_status`, and `build`.
- Add filter controls for `first_match`, `last_match`, `dedupe`, repeated-line collapsing, and grouped context blocks.
- Return only failing packages, tests, files, and diagnostics when a preset can identify them.
- Keep every filtered result traceable to the retained command artifact.

## Priority 2: Command History Search

- Add `search_command_history`.
- Search fields: `repo_path`, `cwd`, command substring, exit code, time range, output hash, evidence keyword, and filter preset.
- Return compact index records, not full logs.
- Allow follow-up `filter_command_history` by `command_id`.

## Priority 3: Log Retention And Archive

- Keep active logs in `~/.mcp-ai-helper/repos/<project>/logs`.
- Maintain `index.jsonl` plus per-command records under `records/`.
- Add daily archives under `archive/YYYY-MM-DD.*`.
- Add `cleanup_logs` with retention by age and max records.
- Test cleanup, compression, archive creation, and index rewrite.

## Priority 4: Project Tasks

- Keep project tasks in `~/.mcp-ai-helper/repos/<project>/tasks` as Lean files.
- Add task lifecycle tools: add, list, current, get, update, and delete.
- Link task entries to command ids, commits, and workflow ids.
- Add compaction rules so old completed tasks can be archived without losing auditability.

## Priority 5: Workflow DSL

- Keep `run_workflow` as the stable MCP entrypoint.
- Expose structured workflow input through MCP so callers can send edits, checks, conditions, and commit in one request.
- Prefer one long workflow whenever intermediate results are not needed by the calling model.
- Support command sequences with per-step output filters so the caller receives only final necessary evidence.
- Extend internal steps instead of adding new MCP tools when possible.
- Add deterministic conditions: `&&`, `||`, `!`, `exit_code == 0`, `changed_files contains`, and `steps.<id>.output.<field>`.
- Add `on_failure` policies: `stop`, `continue`, `rollback_own_changes`, and `diagnose`.
- Keep condition evaluation deterministic and non-Turing-complete.

## Priority 6: Idempotent Edit Tools

- Add `ensure_block`.
- Add `remove_block`.
- Add `replace_between_markers`.
- Add structured JSON/YAML/TOML update steps.
- Add Go-aware edit steps for imports and simple declarations.
- Preserve hash checks and repo-relative path policy for every edit operation.

## Priority 7: Rollback And Ownership

- Record snapshots before every owned edit.
- Support rollback of only workflow-owned changes.
- Never revert unrelated user changes.
- Detect overlap conflicts before rollback.
- Report rollback decisions in structured workflow results.

## Priority 8: Test And Quality Gates

- Keep `go test ./...`, `go vet ./...`, and `golangci-lint run ./...` green.
- Add coverage threshold.
- Add golden tests for workflow results.
- Add fuzz tests for path escaping and output filters.
- Add integration tests for command history search and log retention.

## Priority 9: MCP Stability

- Avoid frequent MCP schema changes.
- Route new capabilities through `run_workflow.steps` where possible.
- Keep low-level tools for diagnostics and bootstrapping.
