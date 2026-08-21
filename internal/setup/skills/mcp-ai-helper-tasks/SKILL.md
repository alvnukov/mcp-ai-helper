---
name: mcp-ai-helper-tasks
description: Select, execute, and close repository tasks through mcp-ai-helper — one fitting task, bounded reads, guarded edits, and a single workflow that closes checks, task status, and the owned-files commit together. Use when starting or finishing task-backed work, choosing the next task, recording cross-repo feedback, or when a blocker or surface mismatch stops task work.
---

# Task work through mcp-ai-helper

The `task` tool is the only interface to the registry. Whatever backs it —
Lean sources, Obsidian notes, JSON — the files are the server's: hand edits
there corrupt state every tool still trusts.

## Order of work

1. Call `task action=current`, even when a task id was supplied. Choose one
   executable task whose model_level fits the current model; a blocked
   parent or epic is context, never work.
2. Call `task action=get` for the chosen id. When `tool_manifest` exposes
   `task_context`, use it as the bounded packet; otherwise continue with
   task get and narrow reads.
3. Call `git action=status`. Dirty files outside task ownership stay
   untouched.
4. State the plan before editing: selected task, exact owned_files,
   forbidden files, acceptance criteria, minimal gate, and the atomic close
   path.
5. Read only relevant ranges with `file action=read`, `file action=read_many`,
   `file action=search`, and `file action=snapshot`.
6. Call `run action=schema`, then close in one `run action=workflow` call.

## Closing a task

Use only workflow steps the schema returned. The current step set includes
`guarded_replace`, `write_file`, `command`, `task_batch_upsert`,
`task_transition`, `git_commit_owned`, and `git_prepare_task_worktree`.

Order the workflow: edits, formatting, focused checks, risk-based wider
gates, `task_transition`, then `git_commit_owned` over exactly `owned_files`.
Make every check depend on all the edits it covers, so the DAG cannot
validate stale code.

Done means the whole workflow succeeded: acceptance criteria met, gates
green, status transitioned, and the owned-files commit landed in the same
run. A failed or running check, a timeout, a missing result, a skipped gate,
or a separate status commit leaves the task open. When a workflow loses its
result, inspect its command_id and reconcile task and git state once.

## Feedback intake

When the user reports a defect or friction worth keeping, record it with
`issue_add` and take ownership with `issue_accept` before working it. When
the `issue_*` tools are absent from the manifest, the issues layer is off:
follow mcp-ai-helper-surface instead of improvising storage.

## When something is missing

Stop with `surface_mismatch` or `blocked` when a required tool, action,
workflow step, ownership fact, or status is absent. Routing around the
helper with shell, direct git, or manual registry edits turns a visible gap
into silent corruption.

Obey repository instructions that require command execution or polling to
be delegated; when the delegate lacks the helper surface, report the
blocker. On a failed check, inspect actual state once, form a new
hypothesis, and reuse the command_id of what already ran.
