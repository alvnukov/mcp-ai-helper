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
