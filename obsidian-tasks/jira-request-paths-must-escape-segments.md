---
id: jira-request-paths-must-escape-segments
title: Сегменты путей Jira-запросов должны экранироваться
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - jira
    - security
    - paths
acceptance_criteria:
    - issueKey/propertyKey проходят url.PathEscape перед интерполяцией в path
    - ../ и ? в ключах больше не уводят на другой REST-путь
    - тест с ключом, содержащим спецсимволы
created_at: "2026-08-14T21:18:50.506849Z"
updated_at: "2026-08-14T21:33:00.595705Z"
---

## Body

jira/client.go:130,158 — issueKey/propertyKey интерполируются в путь запроса без эскейпа: ../ или ? в property_key целят другой REST-путь на Jira-хосте.

## Acceptance Criteria

- issueKey/propertyKey проходят url.PathEscape перед интерполяцией в path
- ../ и ? в ключах больше не уводят на другой REST-путь
- тест с ключом, содержащим спецсимволы
