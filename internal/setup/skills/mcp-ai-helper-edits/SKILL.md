---
name: mcp-ai-helper-edits
description: Inspect, change, verify, and commit repository files exclusively through mcp-ai-helper using bounded reads, hash-guarded edits, explicit ownership, and workflow gates. Use for any file or git change in a repository served by mcp-ai-helper, especially dirty worktrees, conflicting edits, new files, or atomic task finalization.
---

# Editing through mcp-ai-helper

Call `assistant_guidance` and `tool_manifest` before relying on an action. Keep
context bounded and ownership explicit. Never substitute shell, direct file APIs,
or direct git when the helper surface is required.

## Read

- Use `file action=read` with offset and limit for one range.
- Use `file action=read_many` for up to eight known files.
- Use `file action=search` with a narrow path and capped matches.
- Use `file action=list` for structured directory discovery.
- Use `file action=snapshot` immediately before a guarded change.

## Change

For an existing file:

1. Read the intended region.
2. Snapshot the file.
3. Call `edit action=replace` with expected_hash and an old span that occurs once.

Use old_b64 and new_b64 when transport escaping is risky. If the hash conflicts,
re-read and re-snapshot; never remove the guard.

Use `edit action=write` for a whole or new file, with expected_hash when it
already exists. Before a new-file task that requires atomic completion, verify
`run action=schema` exposes a write/create step. If it does not, report
surface_mismatch; do not create outside the workflow and claim atomic closure.

## Verify

Start a narrow check with `command action=run`. Retain its command_id. Use
`command action=get` and `command action=filter` rather than rerunning it. Run
wider gates only for a concrete regression risk.

## Commit

Inspect with `git action=status` and `git action=diff`. Use `git action=commit`
only over explicit owned files. For task-backed changes, prefer
`run action=workflow` with `git_commit_owned` after every gate and the task
transition; include exactly owned_files and preserve unrelated dirty files.

`git action=log`, `git action=blame`, and `git action=prepare_task_worktree`
provide history and isolation without shell.

