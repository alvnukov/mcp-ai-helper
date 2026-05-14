---
id: task-120
title: Добавить discovery-flow tests для self-describing task context surface
status: todo
priority: medium
model_level: low
task_type: tests
tags:
  - tasks
  - mcp
  - self-describing
  - tests
  - discovery
branch: tests/task-120
worktree_path: .worktrees/task-120
acceptance_criteria:
  - Test or snapshot covers the intended call order without external docs.
  - Test fails if task_context/task_graph descriptions omit truncation or fact-vs-inference semantics.
  - Test verifies structured errors provide actionable next_call guidance where implemented.
  - No unrelated MCP schema snapshots are churned.
verification_plan:
  - Run only the focused discovery/schema/guidance tests.
  - Do not run broad tests unless shared MCP metadata generation changed.
created_at: 2026-05-14T09:15:25.49053Z
updated_at: 2026-05-14T09:15:25.49053Z
---

## Body

Add a minimal discovery-flow test proving a model can infer the correct usage path from MCP responses alone: assistant_guidance -> task_current -> task_context, with task_graph used for overview/dependency inspection. Keep fixtures compact.

## Acceptance Criteria

- Test or snapshot covers the intended call order without external docs.
- Test fails if task_context/task_graph descriptions omit truncation or fact-vs-inference semantics.
- Test verifies structured errors provide actionable next_call guidance where implemented.
- No unrelated MCP schema snapshots are churned.

## Verification Plan

1. Run only the focused discovery/schema/guidance tests.
2. Do not run broad tests unless shared MCP metadata generation changed.
