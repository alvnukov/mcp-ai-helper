---
id: task-019
title: Добавить маршрутизацию по возможностям моделей и cost-aware delegation
status: blocked
priority: high
model_level: very_high
tags:
  - routing
  - models
  - cost
  - epic
  - decomposed
worktree_path: .worktrees/task-019
acceptance_criteria:
  - Parent remains non-executable until child tasks cover routing policy and deterministic implementation.
  - Child tasks preserve the weak-safe summarization and strong-required analysis requirements.
  - Child implementation must not expand provider support beyond existing model/profile surfaces unless a separate provider task requires it.
verification_plan:
  - Check task-081 defines routing inputs and decision matrix before task-082 starts.
  - Check task-082 has focused tests for model level, context/cost constraints, weak-safe and strong-required paths.
created_at: 2026-05-14T09:15:25.477158Z
updated_at: 2026-05-14T09:15:25.477158Z
---

## Body

Parent/epic for model capability and cost-aware routing. Original scope: route requests by task type, model strengths, context budget, reliability requirements and cost; support weak-safe summarization and strong-required analysis paths without leaking prompt/schema noise. Execute through child tasks task-081 and task-082; do not implement directly from the parent.

## Acceptance Criteria

- Parent remains non-executable until child tasks cover routing policy and deterministic implementation.
- Child tasks preserve the weak-safe summarization and strong-required analysis requirements.
- Child implementation must not expand provider support beyond existing model/profile surfaces unless a separate provider task requires it.

## Verification Plan

1. Check task-081 defines routing inputs and decision matrix before task-082 starts.
2. Check task-082 has focused tests for model level, context/cost constraints, weak-safe and strong-required paths.
