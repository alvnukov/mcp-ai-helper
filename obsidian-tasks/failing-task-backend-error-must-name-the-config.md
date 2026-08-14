---
id: failing-task-backend-error-must-name-the-config
title: Ошибка failingTaskBackend должна называть конфиг и ключ для починки
status: done
priority: low
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - tasks
    - config
    - errors
    - remediation
acceptance_criteria:
    - Ошибка отсутствующего obsidian path называет конфиг-файл, куда его вписать
    - Ошибка неподдерживаемого backend называет допустимые значения и конфиг-файл
    - Тест фиксирует ремедиацию в тексте ошибки
created_at: "2026-08-14T22:25:49.139499Z"
updated_at: "2026-08-14T22:29:11.376239Z"
---

## Body

При ошибке конфигурации task registry failingTaskBackend возвращает голый факт ('task_registry.obsidian.path is required') во все task-инструменты. Ошибка fail-closed правильная, но не говорит, какой файл править — модель вынуждена гадать или читать конфиг заново. Ремедиация стоит дёшево: cfg.SourcePath уже доступен в buildTaskBackend (internal/mcp/server.go:123-139).

## Acceptance Criteria

- Ошибка отсутствующего obsidian path называет конфиг-файл, куда его вписать
- Ошибка неподдерживаемого backend называет допустимые значения и конфиг-файл
- Тест фиксирует ремедиацию в тексте ошибки
