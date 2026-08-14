---
id: jira-client-must-retain-startup-error
title: jira-клиент должен сохранять ошибку старта (parity с confluence)
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - jira
    - diagnostics
    - integrations
acceptance_criteria:
    - buildJiraClient возвращает (client, error), ошибка сохраняется в поле как confluenceClientErr
    - битый URL даёт осмысленную диагностику из реальной ошибки, не «not configured or connection failed»
    - reload пересобирает с сохранением той же семантики
    - тест на битый URL
created_at: "2026-08-14T21:18:50.498585Z"
updated_at: "2026-08-14T21:24:18.698272Z"
---

## Body

server.go:38-47,77-84 — buildJiraClient глотает ошибку NewClient; jiraClientErr нет. Двойник исправленного confluence-бага (ef69f28): битый URL → вечное «not configured or connection failed» без диагностики даже после reload.

## Acceptance Criteria

- buildJiraClient возвращает (client, error), ошибка сохраняется в поле как confluenceClientErr
- битый URL даёт осмысленную диагностику из реальной ошибки, не «not configured or connection failed»
- reload пересобирает с сохранением той же семантики
- тест на битый URL
