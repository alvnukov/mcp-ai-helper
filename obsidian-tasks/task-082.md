---
id: task-082
title: Реализовать deterministic model router с focused tests
status: todo
priority: high
model_level: medium
tags:
  - routing
  - models
  - cost
  - implementation
  - tests
worktree_path: .worktrees/task-082
acceptance_criteria:
  - Router implements the task-081 matrix without introducing new provider infrastructure.
  - Unsupported or unsafe routing choices return compact blocker diagnostics.
  - Focused tests cover model level selection, cost/context constraints, weak-safe summaries, strong-required paths and fail-closed unknown cases.
verification_plan:
  - Run targeted Go tests for the router package or exact affected tests only.
  - If model profile config changes, run the focused config/profile tests.
  - Do not run broad project tests unless shared routing contracts are changed.
created_at: 2026-05-14T09:15:25.485495Z
updated_at: 2026-05-14T09:15:25.485495Z
---

## Body

Implement the deterministic routing policy from task-081 using existing model/profile surfaces only. Include typed routing decisions, compact denial/blocker reasons and focused tests for model level, context/cost constraints, weak-safe summaries and strong-required paths. Do not add external provider adapters here.

## Acceptance Criteria

- Router implements the task-081 matrix without introducing new provider infrastructure.
- Unsupported or unsafe routing choices return compact blocker diagnostics.
- Focused tests cover model level selection, cost/context constraints, weak-safe summaries, strong-required paths and fail-closed unknown cases.

## Verification Plan

1. Run targeted Go tests for the router package or exact affected tests only.
2. If model profile config changes, run the focused config/profile tests.
3. Do not run broad project tests unless shared routing contracts are changed.
