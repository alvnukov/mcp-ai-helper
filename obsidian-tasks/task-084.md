---
id: task-084
title: Реализовать command policy enforcement MVP
status: todo
priority: high
model_level: medium
tags:
  - security
  - commands
  - policy
  - implementation
  - tests
worktree_path: .worktrees/task-084
acceptance_criteria:
  - Current command surfaces share one policy path or equivalent consistent checks.
  - Denied commands fail before execution with compact diagnostics and no partial unsafe output.
  - Focused tests cover destructive patterns, cwd escape, timeout/output limits and supported env/network controls.
  - Existing allowed targeted checks still work.
verification_plan:
  - Run targeted Go tests for command policy and affected execution surfaces.
  - Run one focused happy-path command test and denial tests only.
  - Escalate instead of broadening implementation if policy requires unsupported network/env isolation.
created_at: 2026-05-14T09:15:25.485957Z
updated_at: 2026-05-14T09:15:25.485957Z
---

## Body

Implement the policy from task-083 for the current command execution surfaces, including run_pipeline, collect_command_output and workflow command steps where applicable. Enforce fail-closed denial for destructive patterns, cwd escape, timeout/output caps and supported env/network controls. Add focused tests; do not relax policy limits to make tests pass.

## Acceptance Criteria

- Current command surfaces share one policy path or equivalent consistent checks.
- Denied commands fail before execution with compact diagnostics and no partial unsafe output.
- Focused tests cover destructive patterns, cwd escape, timeout/output limits and supported env/network controls.
- Existing allowed targeted checks still work.

## Verification Plan

1. Run targeted Go tests for command policy and affected execution surfaces.
2. Run one focused happy-path command test and denial tests only.
3. Escalate instead of broadening implementation if policy requires unsupported network/env isolation.
