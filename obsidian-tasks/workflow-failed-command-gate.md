---
id: workflow-failed-command-gate
title: Workflow не должен коммитить после failed command gate
status: done
priority: critical
model_level: high
task_type: bug
tags:
    - workflow
    - atomicity
    - commands
    - tasks
    - git
    - regression
acceptance_criteria:
    - Command step с ненулевым exit code и on_failure=stop прекращает workflow; downstream task transition и commit не выполняются.
    - on_failure=continue сохраняет probe-семантику и позволяет условным downstream steps работать.
    - Несколько guarded_replace одного файла используют актуальный hash после предыдущего шага либо fail-closed до частичного ложного успеха.
    - Регрессионные тесты воспроизводят реальный порядок command → task_transition → git_commit_owned.
verification_plan:
    - Сначала добавить узкий red regression test для failed command gate и подтвердить падение.
    - Запустить targeted internal/pipeline tests, затем make quality и lint из атомарного workflow.
created_at: "2026-08-22T10:07:00.207346Z"
updated_at: "2026-08-22T10:16:38.757155Z"
---

## Body

В реальном run action=workflow command gate вернул failed, однако downstream task_transition и git_commit_owned выполнились. Исправить fail-closed семантику on_failure=stop и проверить same-file guarded_replace hash chaining. Не полагаться на ручные if как единственную защиту.

## Acceptance Criteria

- Command step с ненулевым exit code и on_failure=stop прекращает workflow; downstream task transition и commit не выполняются.
- on_failure=continue сохраняет probe-семантику и позволяет условным downstream steps работать.
- Несколько guarded_replace одного файла используют актуальный hash после предыдущего шага либо fail-closed до частичного ложного успеха.
- Регрессионные тесты воспроизводят реальный порядок command → task_transition → git_commit_owned.

## Verification Plan

1. Сначала добавить узкий red regression test для failed command gate и подтвердить падение.
2. Запустить targeted internal/pipeline tests, затем make quality и lint из атомарного workflow.
