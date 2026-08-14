---
id: task-ui-must-reject-cross-origin-writes
title: task_ui должен отвергать кросс-доменные записи (CSRF)
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - security
    - csrf
    - task-ui
    - http
acceptance_criteria:
    - мутирующие эндпоинты требуют application/json Content-Type
    - Origin при наличии должен совпадать с Host (loopback), иначе 403
    - 'тест: POST text/plain с чужим Origin отвергается; нормальный клиент работает'
created_at: "2026-08-14T21:18:50.503299Z"
updated_at: "2026-08-14T21:49:59.72701Z"
---

## Body

task_ui.go:223-226 — decodeTaskUIJSON не проверяет Content-Type/Origin/Host: злая страница шлёт кросс-доменный POST text/plain (no-preflight) на 127.0.0.1:18067 и мутирует task registry. Смягчено opt-in-ностью task_ui.

## Acceptance Criteria

- мутирующие эндпоинты требуют application/json Content-Type
- Origin при наличии должен совпадать с Host (loopback), иначе 403
- тест: POST text/plain с чужим Origin отвергается; нормальный клиент работает
