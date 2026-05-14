---
id: task-119
title: Добавить schema/help examples для task_graph и task_context
status: done
priority: high
model_level: medium
task_type: implementation
tags:
    - tasks
    - schema
    - help
    - mcp
    - self-describing
branch: implementation/task-119
worktree_path: .worktrees/task-119
acceptance_criteria:
    - Schema/help output includes compact valid examples for both tools.
    - Examples explain fact/provenance, omitted/truncated metadata and next_call guidance for common errors.
    - The information is available through MCP responses without README/manual registry reads.
    - No broad documentation rewrite is included.
verification_plan:
    - Run targeted tests/snapshots for schema/help output.
    - Check examples remain short and machine-readable.
created_at: "2026-05-14T09:15:25.490387Z"
updated_at: "2026-05-14T10:09:40.376811Z"
---

## Body

Add MCP-facing schema/help examples for task_graph and task_context, including valid calls, field interpretation, limits, truncation handling, fact-vs-inference semantics and structured error examples. Prefer extending existing schema/help surfaces over adding duplicate docs.

## Acceptance Criteria

- Schema/help output includes compact valid examples for both tools.
- Examples explain fact/provenance, omitted/truncated metadata and next_call guidance for common errors.
- The information is available through MCP responses without README/manual registry reads.
- No broad documentation rewrite is included.

## Verification Plan

1. Run targeted tests/snapshots for schema/help output.
2. Check examples remain short and machine-readable.
