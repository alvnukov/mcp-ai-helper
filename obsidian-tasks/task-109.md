---
id: task-109
title: Инвентаризировать текущие task registry поля и MCP task surface
status: todo
priority: high
model_level: low
task_type: design
tags:
  - tasks
  - registry
  - mcp
  - inventory
  - design
branch: design/task-109
worktree_path: .worktrees/task-109
acceptance_criteria:
  - Inventory lists available factual fields and explicitly marks missing or inferred relationship sources.
  - Existing task_current and task_packet behavior is summarized enough to avoid breaking callers.
  - No manual registry edits, product code changes or speculative relationship rules are introduced.
verification_plan:
  - Use narrow MCP/helper-supported reads or focused commands only.
  - Review inventory against at least one decomposed epic and one executable child task.
created_at: 2026-05-14T09:15:25.4891Z
updated_at: 2026-05-14T09:15:25.4891Z
---

## Body

Gather the minimal implementation facts needed for task graph/context design: existing task fields, parent/status/model_level/tags behavior, task_current/task_packet contracts, registry provenance and missing relationship data. Output a compact design note or test fixture reference; no implementation behavior changes.

## Acceptance Criteria

- Inventory lists available factual fields and explicitly marks missing or inferred relationship sources.
- Existing task_current and task_packet behavior is summarized enough to avoid breaking callers.
- No manual registry edits, product code changes or speculative relationship rules are introduced.

## Verification Plan

1. Use narrow MCP/helper-supported reads or focused commands only.
2. Review inventory against at least one decomposed epic and one executable child task.
