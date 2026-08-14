---
id: workflow-task-transitions-must-roll-back
title: task_transition в workflow должен откатывать частичный сбой
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - pipeline
    - tasks
    - workflow
    - transactions
acceptance_criteria:
    - при сбое SetStatus на k-й задаче ранее переведённые 1..k-1 откатываются в исходный статус
    - ошибка шага содержит и сбой, и результат отката
    - retry того же transition после отката проходит From-guard
    - тест на частичный сбой
created_at: "2026-08-14T21:18:50.502369Z"
updated_at: "2026-08-14T21:46:18.510107Z"
---

## Body

pipeline.go:1001 — transitionTasks валидирует все From, применяет SetStatus последовательно без отката: сбой на k-й оставляет 1..k-1 переведёнными, retry клинит об From-guard («status is done, want todo»).

## Acceptance Criteria

- при сбое SetStatus на k-й задаче ранее переведённые 1..k-1 откатываются в исходный статус
- ошибка шага содержит и сбой, и результат отката
- retry того же transition после отката проходит From-guard
- тест на частичный сбой
