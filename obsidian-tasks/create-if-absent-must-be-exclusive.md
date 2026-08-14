---
id: create-if-absent-must-be-exclusive
title: create_if_absent должен быть атомарно эксклюзивным
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - fileops
    - edit
    - concurrency
acceptance_criteria:
    - создание через O_CREATE|O_EXCL, файл в окне между проверкой и записью не затирается
    - существующий файл по-прежнему даёт понятную ошибку
    - тест на параллельное создание
created_at: "2026-08-14T21:18:50.500598Z"
updated_at: "2026-08-14T21:38:46.911118Z"
---

## Body

safe_edit.go:644-649 — CreateIfAbsent это Stat-then-Write: файл, созданный между Stat и WriteFile, молча затирается, что defeats exclusive-create.

## Acceptance Criteria

- создание через O_CREATE|O_EXCL, файл в окне между проверкой и записью не затирается
- существующий файл по-прежнему даёт понятную ошибку
- тест на параллельное создание
