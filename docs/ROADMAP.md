# Roadmap

This file tracks concrete improvements needed to make `mcp-ai-helper` a reliable workflow executor for senior LLMs.

## Workflow Control Order

Next executable task: `task-045` hardens final task status semantics first, because every later workflow feature depends on `run_pipeline` and `run_workflow` failing closed instead of marking work done after a failed gate, skipped check, evidence-only analysis, workflow conflict, or failed commit.

| Order | Task | Owner Level | Owned Files | Acceptance Focus | Minimum Gate | Depends On |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `task-045` finalization semantics | strong | `internal/pipeline`, `internal/mcp`, focused tests | failed commands, invalid evidence, workflow conflicts, and failed commits never mark done | `go test ./internal/pipeline ./internal/mcp` plus one failing MCP fixture | `task-043` cleanup done |
| 2 | `task-012` conditional workflow execution | strong | workflow engine, MCP schema, workflow tests | deterministic branching on exit code, output, file state, task state, validation, and changed files | targeted workflow/MCP tests plus MCP preview fixture | `task-045` |
| 3 | `task-044` execution packet contract | strong | task/planning tools, MCP schema, tests | packet includes context, acceptance criteria, owned/forbidden files, risks, gates, and readiness level | targeted planning/task tests plus MCP packet fixture | `task-045`, `task-012` if packet emits conditional workflow plans |
| 4 | `task-014` idempotent edit tools | strong | fileops package, workflow edit steps, tests | structured patch/block/JSON/YAML edits are guarded and dry-runnable | targeted fileops/workflow tests for conflict and dry-run behavior | `task-045`, `task-044` |
| 5 | `task-015` git ownership hardening | strong | git workflow step, dirty-tree preflight, tests | owned-file commits fail closed on unrelated staged/untracked/changed files and patch conflicts | targeted git/workflow tests with dirty fixture repos | `task-045`, `task-014` |
| 6 | `task-023` audit trail | standard after semantics are stable | workflow result/history records and tests | audit records context ids, branch choices, guard outcomes, command/evidence ids, final status reason | targeted workflow tests for success, skipped branch, failure, and conflict | `task-045`, `task-012` |
| 7 | `task-022` guidance defaults | standard | `internal/config`, `internal/mcp` schema tests | default and setup guidance require context-first, decision-second, one-pipeline-third, batching, close_missing caution, and final-status gating | `go test ./internal/config ./internal/mcp` | `task-045` semantics defined enough to document |
| 8 | `task-024` workflow examples | standard | `README.md`, docs | examples show success, blocked failure, conditional probe, no premature done, compact logs | docs content check plus `lake build` | `task-012`, `task-022` |
| 9 | `task-021` enterprise gates | strong | quality gate config, CI/test fixtures | first-class go test/vet/lint/race gates and workflow failure fixtures | targeted gate tests, then broader suite only with concrete regression risk | `task-045`, `task-012`, `task-015` |

No implementation task in this order is ready while cleanup task `task-043` is unresolved. Since `task-043` is now done, the first ready implementation is `task-045`; tasks below it remain blocked by the dependencies shown here, even if their local code looks small.

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

- Treat the repo-local Lean/Lake registry as canonical task state for migrated repositories.
- Remove legacy JSON-comment `tasks/*.lean` fallback paths after migration; stale projections must not be read as task state.
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
