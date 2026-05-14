---
id: task-108
title: Сделать task graph/context surface self-describing для LLM
status: blocked
priority: high
model_level: medium
task_type: implementation_epic
parent_id: task-104
tags:
  - tasks
  - context
  - mcp
  - schema
  - guidance
  - self-describing
  - tests
  - decomposed
worktree_path: .worktrees/task-108
acceptance_criteria:
  - assistant_guidance states the recommended call order and when to use each tool.
  - Tool/schema/help output explains limits, truncation and fact-vs-inference semantics.
  - Structured errors guide the next MCP call where possible.
verification_plan:
  - Review task-118, task-119 and task-120 outputs together.
  - Run targeted guidance/schema/discovery tests only.
created_at: 2026-05-14T09:15:25.488978Z
updated_at: 2026-05-14T09:15:25.488978Z
---

## Body

Coordination parent for discoverability. Decomposed into task-118, task-119 and task-120. Do not execute directly; close after assistant guidance, schema/help examples and discovery tests prove models can use the surface without README/manual registry reads.

## Acceptance Criteria

- assistant_guidance states the recommended call order and when to use each tool.
- Tool/schema/help output explains limits, truncation and fact-vs-inference semantics.
- Structured errors guide the next MCP call where possible.

## Verification Plan

1. Review task-118, task-119 and task-120 outputs together.
2. Run targeted guidance/schema/discovery tests only.
