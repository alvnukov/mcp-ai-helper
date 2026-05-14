---
id: task-111
title: Определить task_context schema и связь с task_packet
status: todo
priority: high
model_level: medium
task_type: design
tags:
  - tasks
  - context
  - schema
  - task_packet
  - design
branch: design/task-111
worktree_path: .worktrees/task-111
acceptance_criteria:
  - Schema is selected-task-first and explicitly rejects raw full-registry dumps as the primary context.
  - usage_contract states intended use, must-not rules and next call when truncated or insufficient.
  - Design explains whether task_packet wraps task_context or remains separate for backward compatibility.
  - Examples cover executable child task and blocked parent epic.
verification_plan:
  - Review against task-110 graph schema for consistent field names and provenance.
  - Check examples answer what to edit, what not to touch and what gate proves completion.
created_at: 2026-05-14T09:15:25.489352Z
updated_at: 2026-05-14T09:15:25.489352Z
---

## Body

Define the task_context execution packet schema from task-109 and task-110: selected task, goal chain, prerequisites, already done, planned next, blockers, execution boundaries, non-goals, acceptance criteria, verification plan, llm_warnings, usage_contract, truncation behavior and compatibility/coexistence with task_packet. No implementation.

## Acceptance Criteria

- Schema is selected-task-first and explicitly rejects raw full-registry dumps as the primary context.
- usage_contract states intended use, must-not rules and next call when truncated or insufficient.
- Design explains whether task_packet wraps task_context or remains separate for backward compatibility.
- Examples cover executable child task and blocked parent epic.

## Verification Plan

1. Review against task-110 graph schema for consistent field names and provenance.
2. Check examples answer what to edit, what not to touch and what gate proves completion.
