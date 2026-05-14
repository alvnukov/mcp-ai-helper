---
id: task-050
title: Спроектировать формат техзадания и паттерн-промпты для надёжного исполнения младшей моделью
status: blocked
priority: high
model_level: very_high
tags:
  - research
  - design
  - task-format
  - prompt-patterns
  - model-routing
  - safety
  - spec-quality
  - epic
  - decomposed
worktree_path: .worktrees/task-050
acceptance_criteria:
  - Parent remains research/design-only until child outputs define pattern catalog, rewritten task-048 example, sufficiency checklist, author/executor contract and delivery recommendation.
  - Child outputs distinguish new knowledge from meta-cognitive triggers and include counterexample checks.
  - Design must explain how a model chooses a task class before applying a pattern.
verification_plan:
  - Review task-091 for pattern catalog and task-048 rewrite.
  - Review task-092 for sufficiency checklist, author/executor contract and delivery strategy.
created_at: 2026-05-14T09:15:25.479674Z
updated_at: 2026-05-14T09:15:25.479674Z
---

## Body

Parent/epic for task-spec format and prompt-pattern research. Original scope: test the hypothesis that a correctly structured task brief plus pattern prompt acts as a meta-cognitive trigger for lower-level models, forcing complete reasoning paths such as precedence/fallback matrices, state machines, boundary conditions and invariant checks. Execute through child tasks task-091 and task-092; do not implement product changes from the parent.

## Acceptance Criteria

- Parent remains research/design-only until child outputs define pattern catalog, rewritten task-048 example, sufficiency checklist, author/executor contract and delivery recommendation.
- Child outputs distinguish new knowledge from meta-cognitive triggers and include counterexample checks.
- Design must explain how a model chooses a task class before applying a pattern.

## Verification Plan

1. Review task-091 for pattern catalog and task-048 rewrite.
2. Review task-092 for sufficiency checklist, author/executor contract and delivery strategy.
