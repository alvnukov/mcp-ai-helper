---
id: config-reload-must-follow-integration-consumers
title: 'Config-инструменты должны следовать за потребителями: models ИЛИ интеграции'
status: done
priority: medium
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - mcp
    - layers
    - config
    - integrations
    - seam
acceptance_criteria:
    - При layers.models=false и включённой интеграции jira или confluence config_reload/config_read зарегистрированы
    - При layers.models=false без интеграций config-инструменты не регистрируются (текущее поведение сохранено)
    - list_models/query_model остаются строго за слоем models
    - Расширенный layer-gates тест фиксирует обе стороны
created_at: "2026-08-14T22:25:49.140685Z"
updated_at: "2026-08-14T22:30:38.977826Z"
---

## Body

reloadConfig и registerConfigTools живут только под layers.models (internal/mcp/server.go:232-257), а их потребители conf_*/jira_* регистрируются по флагам интеграций (server.go:293-298). При выключенном models-слое и включённом Confluence клиент собирается при старте (фикс ef69f28), но оживить его релоадом конфигурации нельзя. Наблюдение зафиксировано как out-of-scope в билете confluence-client-must-be-built-at-startup; config-шов должен следовать за своими потребителями.

## Acceptance Criteria

- При layers.models=false и включённой интеграции jira или confluence config_reload/config_read зарегистрированы
- При layers.models=false без интеграций config-инструменты не регистрируются (текущее поведение сохранено)
- list_models/query_model остаются строго за слоем models
- Расширенный layer-gates тест фиксирует обе стороны
