---
id: integration-clients-must-retry-rate-limits
title: Confluence/Jira клиенты должны ретраить rate limit с backoff и понятной ошибкой
status: done
priority: medium
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - confluence
    - jira
    - retry
    - http
    - reliability
acceptance_criteria:
    - 'Retry-транспорт в общем пакете internal/httpx: 429 ретраится для всех методов, 502/503/504 — только для идемпотентных (GET/HEAD)'
    - Задержка из Retry-After (секунды или HTTP-date) с клампом; ожидание отменяется контекстом запроса
    - 'После исчерпания попыток ошибка называет число попыток и последний статус, а не ''unknown response status: 429'''
    - Confluence и Jira клиенты используют транспорт; тест на 429→200 последовательность для каждого клиента
created_at: "2026-08-14T22:25:49.136767Z"
updated_at: "2026-08-14T22:33:24.88663Z"
---

## Body

Диагноз подтверждён живым вызовом: conf_spaces после перезапуска сервера дважды вернул 'unknown response status: 429 Too Many Requests' с разрывом 15с — ни повтора, ни Retry-After, ни подсказки. В internal/confluence и internal/jira нет ни одного упоминания retry. Воспроизведение: два вызова conf_spaces подряд при rate limit Atlassian Cloud.

Фикс: транспортный RoundTripper в новом internal/httpx, подключённый в confluence apiWithContext и jira NewClient. Политика fail-safe: 429 не исполняет запрос, поэтому ретраится для любого метода; 5xx неоднозначен, поэтому только GET/HEAD.

## Acceptance Criteria

- Retry-транспорт в общем пакете internal/httpx: 429 ретраится для всех методов, 502/503/504 — только для идемпотентных (GET/HEAD)
- Задержка из Retry-After (секунды или HTTP-date) с клампом; ожидание отменяется контекстом запроса
- После исчерпания попыток ошибка называет число попыток и последний статус, а не 'unknown response status: 429'
- Confluence и Jira клиенты используют транспорт; тест на 429→200 последовательность для каждого клиента
