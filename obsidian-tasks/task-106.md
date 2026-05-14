---
id: task-106
title: Реализовать bounded task_graph MCP tool
status: blocked
priority: high
model_level: medium
task_type: implementation_epic
parent_id: task-104
tags:
  - tasks
  - graph
  - mcp
  - implementation
  - tests
  - decomposed
worktree_path: .worktrees/task-106
acceptance_criteria:
  - task_graph returns bounded nodes/edges with explicit provenance and stable field names.
  - Output includes truncation/omission metadata when limits are hit and never silently drops relevant focus context.
  - Existing task_current/task_packet behavior remains backward compatible.
verification_plan:
  - Review task-112, task-113 and task-114 outputs together.
  - Run only focused graph/MCP registration tests unless shared task registry parsing changes.
created_at: 2026-05-14T09:15:25.488719Z
updated_at: 2026-05-14T09:15:25.488719Z
---

## Body

Coordination parent for graph implementation. Decomposed into task-112, task-113 and task-114. Do not execute directly; close after graph builder, MCP handler/schema and focused tests are complete.

## Acceptance Criteria

- task_graph returns bounded nodes/edges with explicit provenance and stable field names.
- Output includes truncation/omission metadata when limits are hit and never silently drops relevant focus context.
- Existing task_current/task_packet behavior remains backward compatible.

## Verification Plan

1. Review task-112, task-113 and task-114 outputs together.
2. Run only focused graph/MCP registration tests unless shared task registry parsing changes.
