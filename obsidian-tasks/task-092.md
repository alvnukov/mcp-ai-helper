---
id: task-092
title: Определить checklist достаточности ТЗ и delivery strategy для model_level
status: todo
priority: high
model_level: medium
tags:
  - research
  - design
  - task-format
  - model-routing
  - safety
worktree_path: .worktrees/task-092
acceptance_criteria:
  - Checklist states required fields for task brief, reasoning pattern, counterexamples, acceptance criteria and verification plan.
  - Contract separates what the task author must provide from what the executor/model must infer.
  - Delivery recommendation compares per-task, per-class and system-wide pattern prompts.
  - Checklist can return insufficient_spec/blocker instead of forcing execution.
verification_plan:
  - Apply checklist mentally to task-048 and one security/policy task.
  - Confirm it would have blocked or corrected the known task-048 failure mode.
  - Confirm no product implementation is mixed into design scope.
created_at: 2026-05-14T09:15:25.487036Z
updated_at: 2026-05-14T09:15:25.487036Z
---

## Body

Design the sufficiency checklist/validator contract for deciding whether a task can be executed by the target model level. Include task author vs executor responsibilities, required task structure, task-class self-classification, pattern selection, delivery mode recommendation (per-task, per-class, system-wide), and failure/blocker criteria when the spec is insufficient.

## Acceptance Criteria

- Checklist states required fields for task brief, reasoning pattern, counterexamples, acceptance criteria and verification plan.
- Contract separates what the task author must provide from what the executor/model must infer.
- Delivery recommendation compares per-task, per-class and system-wide pattern prompts.
- Checklist can return insufficient_spec/blocker instead of forcing execution.

## Verification Plan

1. Apply checklist mentally to task-048 and one security/policy task.
2. Confirm it would have blocked or corrected the known task-048 failure mode.
3. Confirm no product implementation is mixed into design scope.
