---
id: scope-jira-confluence-tools-by-repository
title: Показывать Jira/Confluence tools только в разрешённых локальных проектах
status: done
priority: high
model_level: high
task_type: feature
tags:
    - jira
    - confluence
    - config
    - tool-surface
    - repository-scope
    - security
acceptance_criteria:
    - Глобальные Jira и Confluence config поддерживают документированный allowed_repositories с нормализацией путей.
    - Startup repo context передаётся в MCP server до регистрации tools; непустой allowlist скрывает tools для неизвестного и неразрешённого repository.
    - Пустой allowed_repositories сохраняет прежнее enabled-поведение; allowed_projects и allowed_spaces продолжают ограничивать удалённые сущности независимо.
    - Config schema/example и targeted tests покрывают allowed, denied, unknown и backward-compatible cases.
verification_plan:
    - Запустить targeted Go tests для internal/config, internal/mcp и cmd/mcp-ai-helper по новым сценариям.
    - Запустить обязательный make quality и проверить happ diagnostics на изменённых Go-файлах.
created_at: "2026-08-22T14:52:04.900736Z"
updated_at: "2026-08-22T15:03:29.249413Z"
---

## Body

Добавить глобальную repository allowlist-политику для Jira и Confluence. Сервер должен получить нормализованный startup repo context до построения MCP tool surface и регистрировать integration tools только когда integration enabled и текущий локальный repository разрешён. Пустой allowlist сохраняет обратную совместимость; непустой список при неизвестном repository работает fail-closed. Не смешивать локальный repository scope с Jira allowed_projects и Confluence allowed_spaces.

## Acceptance Criteria

- Глобальные Jira и Confluence config поддерживают документированный allowed_repositories с нормализацией путей.
- Startup repo context передаётся в MCP server до регистрации tools; непустой allowlist скрывает tools для неизвестного и неразрешённого repository.
- Пустой allowed_repositories сохраняет прежнее enabled-поведение; allowed_projects и allowed_spaces продолжают ограничивать удалённые сущности независимо.
- Config schema/example и targeted tests покрывают allowed, denied, unknown и backward-compatible cases.

## Verification Plan

1. Запустить targeted Go tests для internal/config, internal/mcp и cmd/mcp-ai-helper по новым сценариям.
2. Запустить обязательный make quality и проверить happ diagnostics на изменённых Go-файлах.
