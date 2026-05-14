---
id: task-020
title: Добавить политику безопасности команд и allow/deny controls
status: blocked
priority: high
model_level: very_high
tags:
  - security
  - commands
  - policy
  - epic
  - decomposed
worktree_path: .worktrees/task-020
acceptance_criteria:
  - Parent remains non-executable until policy design and enforcement MVP are complete.
  - Security behavior is fail-closed and does not hide denials by weakening checks or increasing limits.
  - Policy covers shell, destructive command patterns, cwd/workspace boundaries, env/network stance, duration/output limits and compact risk diagnostics.
verification_plan:
  - Check task-083 produces a concrete policy matrix and failure semantics.
  - Check task-084 includes focused denial tests for destructive patterns, cwd escape, unsafe env/network behavior where applicable, timeout and output caps.
created_at: 2026-05-14T09:15:25.477302Z
updated_at: 2026-05-14T09:15:25.477302Z
---

## Body

Parent/epic for fail-closed command execution policy. Original scope: allowed shells, destructive pattern denial, environment controls, network policy, max duration, max output, workspace boundary checks and explicit risk reporting. Execute through child tasks task-083 and task-084; do not implement directly from the parent.

## Acceptance Criteria

- Parent remains non-executable until policy design and enforcement MVP are complete.
- Security behavior is fail-closed and does not hide denials by weakening checks or increasing limits.
- Policy covers shell, destructive command patterns, cwd/workspace boundaries, env/network stance, duration/output limits and compact risk diagnostics.

## Verification Plan

1. Check task-083 produces a concrete policy matrix and failure semantics.
2. Check task-084 includes focused denial tests for destructive patterns, cwd escape, unsafe env/network behavior where applicable, timeout and output caps.
