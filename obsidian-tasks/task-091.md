---
id: task-091
title: Собрать каталог prompt patterns и пример task-048
status: todo
priority: high
model_level: medium
tags:
  - research
  - design
  - prompt-patterns
  - task-format
worktree_path: .worktrees/task-091
acceptance_criteria:
  - Catalog explains when to apply each pattern and what reasoning steps are mandatory.
  - Precedence/fallback pattern includes source enumeration, mutation category matrix and counterexample check.
  - Rewritten task-048 includes enough structure for medium-level execution without hidden high-level inference.
  - The design distinguishes meta-cognitive trigger behavior from adding new domain knowledge.
verification_plan:
  - Review task-048 rewrite against the known failure mode: choosing changedSet precedence too early.
  - Check at least one example for precedence/fallback, state machine and boundary/invariant classes.
  - Confirm no implementation changes are included.
created_at: 2026-05-14T09:15:25.486907Z
updated_at: 2026-05-14T09:15:25.486907Z
---

## Body

Define the first prompt-pattern catalog for reliable lower-level execution: precedence/fallback matrix, state machine, boundary conditions, invariants, forced enumeration and counterexample-first checks. Include a rewritten task-048 example showing exactly how the medium model should avoid the previous precedence mistake.

## Acceptance Criteria

- Catalog explains when to apply each pattern and what reasoning steps are mandatory.
- Precedence/fallback pattern includes source enumeration, mutation category matrix and counterexample check.
- Rewritten task-048 includes enough structure for medium-level execution without hidden high-level inference.
- The design distinguishes meta-cognitive trigger behavior from adding new domain knowledge.

## Verification Plan

1. Review task-048 rewrite against the known failure mode: choosing changedSet precedence too early.
2. Check at least one example for precedence/fallback, state machine and boundary/invariant classes.
3. Confirm no implementation changes are included.
