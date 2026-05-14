---
id: task-051
title: Fix git_commit_owned handling for deleted owned files
status: done
priority: critical
tags:
  - issue
  - feedback
  - git
  - workflow
  - commit
  - deletions
  - surface-mismatch
worktree_path: .worktrees/task-051
created_at: 2026-05-14T09:15:25.479806Z
updated_at: 2026-05-14T09:15:25.479806Z
---

## Body

Observed on 2026-05-10: git_commit_owned failed when explicitly owned files included deleted tracked paths such as tasks/task-001.lean. The tool runs `git add -- <files>` for every owned file, so Git exits with `fatal: pathspec 'tasks/task-001.lean' did not match any files` even when the deletion is already staged and is inside the owned set. Expected behavior: git_commit_owned must support committing explicitly owned tracked deletions, likely by using a deletion-aware staging path such as `git add -u -- <files>` for tracked deleted files or by trusting already-staged deletions after validating they are within owned_files. This blocks the required closeout rule that repo tasks with changes end in an owned-files commit.
