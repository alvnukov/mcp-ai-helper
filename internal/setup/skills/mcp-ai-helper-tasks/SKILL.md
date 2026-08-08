---
name: mcp-ai-helper-tasks
description: Select, execute, and close repository tasks through mcp-ai-helper with live surface discovery, bounded context, explicit ownership, acceptance gates, and one atomic run action=workflow finalization. Use when starting or finishing task-backed work, choosing the next task, handling a blocker or surface mismatch, or working in a dirty repository served by mcp-ai-helper.
---

# Task work through mcp-ai-helper

Treat the task registry behind the `task` tool as canonical. Never parse or mutate
task registry files directly.

Call `assistant_guidance` and `tool_manifest` once per session. Guidance is the
current policy; the manifest is the current surface. Never infer hidden tools.

## Order of work

1. Call `task action=current`, even when a task id was supplied. Select only an
   executable task whose model_level matches the current model; never execute a
   blocked parent or epic.
2. Call `task action=get` for the chosen id. If the manifest exposes
   `task_context`, use it as an additional bounded packet. Otherwise continue
   with task get and narrow reads; do not invent the tool.
3. Call `git action=status`. Preserve dirty files outside task ownership.
4. State the selected task, exact owned_files, forbidden files, acceptance
   criteria, minimal gate, and atomic close path before editing.
5. Read only relevant ranges with `file action=read`, `file action=read_many`,
   `file action=search`, and `file action=snapshot`.
6. Call `run action=schema`, then close in one `run action=workflow` call.

## Closing a task

Use only workflow steps returned by the schema. The current step set includes
`guarded_replace`, `write_file`, `command`, `task_batch_upsert`, `task_transition`,
`git_commit_owned`, and `git_prepare_task_worktree`.

Explicitly order edits, formatting, focused checks, risk-based wider gates,
`task_transition`, then `git_commit_owned` over exactly `owned_files`. Make every
check depend on all relevant edits so a DAG cannot validate stale code.

The done transition and owned-files commit belong in the same workflow. A failed
or running check, timeout, missing result, skipped gate, or separate status
commit is not completion. If a workflow loses its result, inspect its command_id
and reconcile task and git state once; never replay mutations blindly.

## When something is missing

Stop with `surface_mismatch` or `blocked` when a required tool, action, workflow
step, ownership fact, or status is absent. Do not route around the helper with
shell, direct git, generic file tools, or manual task-file edits.

Obey repository instructions that require command execution or polling to be
delegated. If the required delegate lacks the helper surface, report the blocker.
On a failed check, inspect actual state once, form a new hypothesis, and reuse
its command_id instead of rerunning unchanged work.

