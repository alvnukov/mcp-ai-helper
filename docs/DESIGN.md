# Design

`mcp-ai-helper` is an execution and context broker for senior models.

It is not an autonomous agent. The caller sends a bounded workflow; the server performs deterministic collection, filtering, model routing, grounding validation, and guarded side effects.

## Invariants

1. Long raw output is not returned by default.
2. Model analysis is separate from evidence.
3. Analysis must cite evidence ids.
4. Failed grounding returns `INSUFFICIENT_DATA`.
5. Commands run under cwd, timeout, and output limits.
6. File edits are guarded by content hash and unique spans.
7. Git commits stage only explicit owned files.
8. Secrets are redacted before persistence or model calls.
9. Local workflow tools receive an explicit `repo_path` from the caller; file paths and working directories are repo-relative unless a tool explicitly documents otherwise.

## MVP workflow

```text
collect_command_output
  -> deterministic evidence extraction
  -> optional model summary
  -> optional model analysis
  -> evidence-link validation
  -> compact handoff
```

Edit and commit workflows should normally use one high-level call:

```text
run_workflow
  -> guarded edits
  -> checks
  -> owned-files commit when checks pass
```

Low-level tools (`snapshot_file`, `apply_guarded_replace`, `collect_command_output`, `git_commit_owned`) remain available for diagnostics and for bootstrapping new workflow capabilities.

The pipeline engine should grow toward DAG/state-machine execution, but all branching must remain deterministic over structured step results.

`run_workflow.steps` is the stable extension point. New local capabilities should normally be added as internal step tools rather than new MCP tools, so Codex does not need to rediscover schema for every workflow improvement.

## Lean Registry Server Boundary

Research on the local toolchain shows that the Lake-facing server entry point is `lake serve`, not `lake --server`. `lake serve` runs `lean --server` inside the package environment. A raw `lake --server` invocation is not a valid Lake flag in the current toolchain.

The transport exposed by `lake serve` is the Lean language server over stdio using LSP `Content-Length` framing. A minimal `initialize` request returns `Lean 4 Server` capabilities, including the Lean-specific RPC surface. The relevant server methods are `$/lean/rpc/connect`, `$/lean/rpc/keepAlive`, `$/lean/rpc/release`, and `$/lean/rpc/call`. Custom callable methods are possible through Lean server RPC methods tagged with `@[server_rpc_method]`, but this is a language-server RPC mechanism, not a generic Lake command dispatcher.

Registry work must respect that boundary. Go may manage process lifecycle, command policy, request framing, evidence, and owned commits. Go must not parse or mutate `MCPAIHelperProject/ActiveTasks.lean` as production task-registry behavior. Registry reads and writes should be Lean-owned server operations with typed request/response payloads and typed diagnostics.

The next safe path is staged:

1. Design the Lean-owned RPC contract before production integration.
2. Prove one read-only query through `lake serve` / `lean --server` before translating the full task read surface.
3. Prove one mutation transaction with Lean-side validation and fail-closed semantics before translating batch/status/delete tools.
4. Add hardening that prevents Go-side source parsing or regex mutation from returning as a production path.

Open risks remain around long-running server lifecycle, RPC session keepalive/release, how project modules are loaded into the file worker context, and whether mutation should be exposed through language-server RPC or a separate Lean-owned transaction executable coordinated by the server protocol.

## Task Registry Server Contract

The canonical registry service is Lean-owned. The Go side is only a client and workflow orchestrator. It starts or reuses `lake serve`, performs LSP framing, opens or loads the registry module as required by Lean server RPC, sends typed requests, validates the response envelope, records evidence, and commits explicit owned files. It does not inspect Lean source text for task state and does not mutate `ActiveTasks.lean` with string or regex replacement.

The transport contract is:

1. Start `lake serve` in the repository root under the command policy.
2. Send LSP `initialize` and complete Lean RPC session setup with `$/lean/rpc/connect`.
3. Call one project-defined registry RPC method through `$/lean/rpc/call`. The method name is owned by Lean code and must be registered with `@[server_rpc_method]` or an equivalent Lean server RPC hook.
4. Keep the session alive for multi-call workflows with `$/lean/rpc/keepAlive`; release it with `$/lean/rpc/release` before shutdown.
5. Treat malformed JSON, missing fields, unexpected schema versions, server exit, timeout, and Lean diagnostics as fail-closed blockers.

Every registry response uses one envelope:

```json
{
  "schema_version": 1,
  "ok": true,
  "operation": "task.get",
  "data": {},
  "diagnostics": [],
  "changed_files": [],
  "validation": {"checked": true, "summary": "registry invariants passed"}
}
```

Failures use the same envelope with `ok: false`, no partial success, and diagnostics carrying `code`, `message`, `severity`, and optional `field`, `task_id`, and `source_range`. Go may summarize diagnostics, but it must preserve enough structured data to explain why a workflow did not mark a task done.

The minimum command ADT is:

- `task.list`: list `active` or `all`, optionally filter by exact status and query.
- `task.get`: return one task by id.
- `task.transition`: transition one or more task ids from an expected status to a target status.
- `task.upsert`: create or replace one canonical task.
- `task.batch_upsert`: synchronize an explicit task set, with intentional `close_missing` semantics.
- `task.delete`: delete or archive one canonical task, depending on registry policy.
- `registry.validate`: return invariant diagnostics without mutating state.

A task payload is first-class data, not parsed text: `id`, `parent_id`, `status`, `title`, `body`, `priority`, `tags`, `acceptance_criteria`, `verification_plan`, `created_at`, `updated_at`, `projection_source`, and relation fields such as `depends_on` and `blocks` when supported.

Mutation semantics are stricter than read semantics. Lean owns normalization, ID validation, status validation, timestamp preservation, structured field serialization, relation validation, and registry invariant checks. A mutation is atomic from the Go caller's point of view: if validation fails, the response is `ok: false`, `changed_files` is empty, and no registry source change may remain. If validation passes, the response lists the repo-relative files that Go is allowed to include in `git_commit_owned`.

Compatibility with current MCP tools is by adapter, not by duplicated logic. `task_current`, `task_get`, `task_list`, and `task_search` adapt to `task.list`/`task.get`. `task_set_status` and workflow `task_transition` adapt to `task.transition`. `task_upsert`, `task_batch_upsert`, and `task_delete` adapt to the matching Lean-owned mutation commands. Legacy task storage is not a fallback for migrated Lean repos; server or protocol failure is a blocker.

This contract leaves two implementation gates open: first prove one read-only query end to end, then prove one mutation transaction with rollback/fail-closed evidence. Only after both gates should `task-047` be closed.
