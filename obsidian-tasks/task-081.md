---
id: task-081
title: Определить matrix capability/cost routing policy
status: todo
priority: high
model_level: medium
tags:
  - routing
  - models
  - cost
  - design
worktree_path: .worktrees/task-081
acceptance_criteria:
  - Decision matrix covers task type, model level, capabilities, context budget, cost budget, reliability and safety requirements.
  - Weak-safe summary paths and strong-required analysis paths are explicitly separated.
  - Policy states what data is passed to each model tier and how prompt/schema noise is minimized.
  - Ambiguous or unsupported routing cases fail closed with a compact reason.
verification_plan:
  - Review the matrix against task-019 original scope.
  - Check at least three examples: cheap summary, medium implementation, strong-required security/design analysis.
  - Confirm no provider implementation or broad router code is included.
created_at: 2026-05-14T09:15:25.485383Z
updated_at: 2026-05-14T09:15:25.485383Z
---

## Body

Define deterministic routing inputs and decision matrix for model routing: task type, required model level, model capabilities/limitations, context budget, cost budget, reliability/safety requirements, weak-safe summarization paths, strong-required analysis paths, and prompt/schema-noise minimization. No implementation in this task.

## Acceptance Criteria

- Decision matrix covers task type, model level, capabilities, context budget, cost budget, reliability and safety requirements.
- Weak-safe summary paths and strong-required analysis paths are explicitly separated.
- Policy states what data is passed to each model tier and how prompt/schema noise is minimized.
- Ambiguous or unsupported routing cases fail closed with a compact reason.

## Verification Plan

1. Review the matrix against task-019 original scope.
2. Check at least three examples: cheap summary, medium implementation, strong-required security/design analysis.
3. Confirm no provider implementation or broad router code is included.
