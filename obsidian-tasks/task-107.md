---
id: task-107
title: Реализовать task_context execution packet поверх task graph
status: blocked
priority: high
model_level: medium
task_type: implementation_epic
parent_id: task-104
tags:
  - tasks
  - context
  - mcp
  - llm-tools
  - implementation
  - tests
  - decomposed
worktree_path: .worktrees/task-107
acceptance_criteria:
  - task_context returns compact execution-oriented context for a selected task, not a raw registry dump.
  - Context includes goal chain, boundaries, non-goals, acceptance criteria, verification gates and LLM warnings.
  - Output distinguishes confirmed facts from inferred or unavailable relationships.
verification_plan:
  - Review task-115, task-116 and task-117 outputs together.
  - Run focused context builder and MCP handler tests only.
created_at: 2026-05-14T09:15:25.488849Z
updated_at: 2026-05-14T09:15:25.488849Z
---

## Body

Coordination parent for selected-task execution context. Decomposed into task-115, task-116 and task-117. Do not execute directly; close after context builder, usage/error behavior and focused scenario tests are complete.

## Acceptance Criteria

- task_context returns compact execution-oriented context for a selected task, not a raw registry dump.
- Context includes goal chain, boundaries, non-goals, acceptance criteria, verification gates and LLM warnings.
- Output distinguishes confirmed facts from inferred or unavailable relationships.

## Verification Plan

1. Review task-115, task-116 and task-117 outputs together.
2. Run focused context builder and MCP handler tests only.
