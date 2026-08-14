---
id: config-schema-must-list-task-backend
title: config_schema должен описывать task_registry.backend
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - config
    - schema
    - docs
acceptance_criteria:
    - Schema() содержит task_registry.backend с enum допустимых значений
    - тест на присутствие поля в схеме
created_at: "2026-08-14T21:18:50.497855Z"
updated_at: "2026-08-14T21:25:38.54184Z"
---

## Body

internal/config/schema.go — task_registry.backend (enum по контракту §2.1) отсутствует в Schema(), хотя README обещает «every field» в config_schema.

## Acceptance Criteria

- Schema() содержит task_registry.backend с enum допустимых значений
- тест на присутствие поля в схеме
