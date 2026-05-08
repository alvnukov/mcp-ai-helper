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
