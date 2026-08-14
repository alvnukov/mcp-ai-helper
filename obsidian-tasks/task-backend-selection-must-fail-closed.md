---
id: task-backend-selection-must-fail-closed
title: Выбор task-бэкенда должен fail-closed по контракту
status: done
priority: critical
model_level: high
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - tasks
    - registry
    - contract
    - fail-closed
acceptance_criteria:
    - 'неизвестное значение task_registry.backend → ошибка на старте (unsupported task_registry.backend: <v>), не Lean-fallback'
    - backend=obsidian без пути → ошибка на старте (path is required), не пустой путь
    - пустой backend (дефолт) по-прежнему даёт Lean без ошибок
    - ошибка провайдится через buildDeps/loadTaskBackendForRepo/pipelineRunnerForRepo
    - тесты на все три случая
created_at: "2026-08-14T21:18:50.496237Z"
updated_at: "2026-08-14T21:43:31.282119Z"
---

## Body

server.go:105-116 — buildTaskBackend на любое неизвестное значение возвращает Lean, obsidian без пути молча берёт пустой Obsidian.Path. Нарушение docs/task-registry-backend-contract.md §2.4/§2.5 (fail at startup, no silent fallback): пользователь думает, что на Obsidian, а записи идут в Lean.

## Acceptance Criteria

- неизвестное значение task_registry.backend → ошибка на старте (unsupported task_registry.backend: <v>), не Lean-fallback
- backend=obsidian без пути → ошибка на старте (path is required), не пустой путь
- пустой backend (дефолт) по-прежнему даёт Lean без ошибок
- ошибка провайдится через buildDeps/loadTaskBackendForRepo/pipelineRunnerForRepo
- тесты на все три случая
