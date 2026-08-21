---
name: mcp-ai-helper-edits
description: Inspect, change, verify, and commit repository files through mcp-ai-helper — search-then-read, hash-guarded replace, and commits over explicitly owned files. Use for any file or git change in a repository served by mcp-ai-helper, especially new files, dirty worktrees, conflicting edits, and atomic task finalization.
---

# Editing through mcp-ai-helper

Every read is bounded and every write is guarded: the helper holds the hash
a write must still match, so a file that moved underneath you fails the
edit instead of being overwritten.

## Read

- `file action=search` first: a narrow pattern with context lines points at
  the ranges worth reading.
- `file action=read` with offset and limit for one range.
- `file action=read_many` for up to eight known files.
- `file action=list` for structured directory discovery.
- `file action=snapshot` immediately before a guarded change.

## Change

For an existing file:

1. Read the intended region.
2. Snapshot the file.
3. Call `edit action=replace` with expected_hash and an old span that
   occurs exactly once.

Use old_b64 and new_b64 when transport escaping is risky; a
`guarded_replace` workflow step takes the same arguments. On a hash
conflict, re-read and re-snapshot: the conflict is the guard working, and
bypassing it is how somebody else's edit gets silently dropped.

Use `edit action=write` for a whole or new file, with expected_hash when
the file already exists. A new file that must land atomically with its
checks belongs in `run action=workflow` as a `write_file` step. Call
`run action=schema` first, and report surface_mismatch when it exposes no
write step, rather than creating the file outside the workflow and losing
the atomic close.

## Verify

Start with the narrowest check that can fail — one package, one test by
name — through `command action=run`, and retain its command_id. Follow it
with `command action=get` and `command action=filter`. Wider gates run
once, before the commit, and only for concrete regression risk.

## Commit

Inspect with `git action=status` and `git action=diff`. `git action=commit`
takes explicit `owned_files` only. For task-backed changes, close through
`run action=workflow` with `git_commit_owned` after every gate and the task
transition; unrelated dirty files stay untouched.

`git action=log`, `git action=blame`, and `git action=prepare_task_worktree`
provide history and isolation without leaving the helper.
