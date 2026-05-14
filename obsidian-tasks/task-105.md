---
id: task-105
title: Спроектировать contract для task_graph и task_context
status: blocked
priority: high
model_level: medium
task_type: design_epic
parent_id: task-104
tags:
  - tasks
  - context
  - graph
  - mcp
  - design
  - schema
  - decomposed
worktree_path: .worktrees/task-105
acceptance_criteria:
  - Current task registry data and existing task tools are inventoried before schema decisions.
  - task_graph and task_context contracts are defined as separate implementation-ready artifacts.
  - Contracts include structured errors, truncation behavior and relationship to task_current/task_packet.
verification_plan:
  - Review task-109, task-110 and task-111 outputs together for consistency.
  - Confirm no product implementation is mixed into design scope.
created_at: 2026-05-14T09:15:25.488574Z
updated_at: 2026-05-14T09:15:25.488574Z
---

## Body

Coordination parent for design. Decomposed into task-109, task-110 and task-111. Do not execute directly; close after child tasks define current data inventory, task_graph schema and task_context schema.

## Acceptance Criteria

- Current task registry data and existing task tools are inventoried before schema decisions.
- task_graph and task_context contracts are defined as separate implementation-ready artifacts.
- Contracts include structured errors, truncation behavior and relationship to task_current/task_packet.

## Verification Plan

1. Review task-109, task-110 and task-111 outputs together for consistency.
2. Confirm no product implementation is mixed into design scope.
