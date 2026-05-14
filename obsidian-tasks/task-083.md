---
id: task-083
title: Спроектировать fail-closed command execution policy
status: todo
priority: high
model_level: medium
tags:
  - security
  - commands
  - policy
  - design
worktree_path: .worktrees/task-083
acceptance_criteria:
  - Policy matrix covers shell, command patterns, cwd/workspace, env, network, duration, output and diagnostics.
  - Each denial mode has a compact machine-readable error contract.
  - Policy explicitly states what is configurable and what is hard-denied.
  - No tests or code are weakened to make risky commands pass.
verification_plan:
  - Review policy against task-020 original scope.
  - Check examples for allowed targeted test, denied destructive command, cwd escape, oversized output and timeout.
  - Confirm no enforcement implementation is mixed into design-only scope.
created_at: 2026-05-14T09:15:25.485639Z
updated_at: 2026-05-14T09:15:25.485639Z
---

## Body

Specify the command execution policy matrix before implementation: allowed shells, denied destructive patterns, cwd/workspace boundaries, environment controls, network stance, max duration, max output, risk levels, compact diagnostics, and fail-closed behavior. No enforcement code in this task.

## Acceptance Criteria

- Policy matrix covers shell, command patterns, cwd/workspace, env, network, duration, output and diagnostics.
- Each denial mode has a compact machine-readable error contract.
- Policy explicitly states what is configurable and what is hard-denied.
- No tests or code are weakened to make risky commands pass.

## Verification Plan

1. Review policy against task-020 original scope.
2. Check examples for allowed targeted test, denied destructive command, cwd escape, oversized output and timeout.
3. Confirm no enforcement implementation is mixed into design-only scope.
