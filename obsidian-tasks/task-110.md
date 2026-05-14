---
id: task-110
title: Определить bounded task_graph schema и failure semantics
status: todo
priority: high
model_level: medium
task_type: design
tags:
  - tasks
  - graph
  - schema
  - design
  - errors
branch: design/task-110
worktree_path: .worktrees/task-110
acceptance_criteria:
  - Schema is implementation-ready and bounded by default.
  - Edge kinds and provenance rules prevent inferred relationships from being presented as facts.
  - Failure semantics cover missing repo, missing focus task, invalid limits and insufficient data.
  - Examples show a parent epic and a focused child-task graph.
verification_plan:
  - Review schema against task-109 inventory and task-104 acceptance criteria.
  - Check examples are short enough for routine LLM use.
created_at: 2026-05-14T09:15:25.489238Z
updated_at: 2026-05-14T09:15:25.489238Z
---

## Body

Define the task_graph input/output schema from task-109 inventory: node fields, edge fields, edge kinds, explicit vs inferred provenance, focus_task_id behavior, max_nodes/max_bytes limits, omitted/truncated metadata and stable error codes. No implementation.

## Acceptance Criteria

- Schema is implementation-ready and bounded by default.
- Edge kinds and provenance rules prevent inferred relationships from being presented as facts.
- Failure semantics cover missing repo, missing focus task, invalid limits and insufficient data.
- Examples show a parent epic and a focused child-task graph.

## Verification Plan

1. Review schema against task-109 inventory and task-104 acceptance criteria.
2. Check examples are short enough for routine LLM use.
