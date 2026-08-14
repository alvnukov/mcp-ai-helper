---
id: integrations-must-enforce-http-timeouts
title: HTTP-клиенты интеграций должны иметь таймауты
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - confluence
    - jira
    - http
    - reliability
acceptance_criteria:
    - confluence и jira клиенты используют http.Client с явным таймаутом (не http.DefaultClient без ограничений)
    - зависший хост не держит вызов дольше внешнего ctx
    - существующие тесты с кастомными транспортерами не ломаются
created_at: "2026-08-14T21:18:50.507434Z"
updated_at: "2026-08-14T21:33:27.279902Z"
---

## Body

confluence/client.go:49-63 — fallback на http.DefaultClient без Timeout; jira/client.go:35-45 аналогично. Зависший хост держит вызов до ctx-deadline.

## Acceptance Criteria

- confluence и jira клиенты используют http.Client с явным таймаутом (не http.DefaultClient без ограничений)
- зависший хост не держит вызов дольше внешнего ctx
- существующие тесты с кастомными транспортерами не ломаются
