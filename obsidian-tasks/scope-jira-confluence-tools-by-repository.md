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
    - fail-closed
acceptance_criteria:
    - Глобальные Jira и Confluence config поддерживают документированный allowed_repositories с нормализацией путей.
    - Startup repo context передаётся в MCP server до регистрации tools; tools видны только для явно разрешённого repository.
    - Пустой или отсутствующий allowed_repositories запрещает Jira/Confluence tools во всех проектах, включая неизвестный repository context.
    - allowed_projects и allowed_spaces продолжают независимо ограничивать удалённые сущности.
    - Config schema/example и targeted tests покрывают explicit allow, denied, unknown и empty-list deny cases.
verification_plan:
    - Запустить targeted Go tests для internal/config и internal/mcp по новым fail-closed сценариям.
    - Запустить обязательный make quality и проверить happ diagnostics на изменённых Go-файлах.
created_at: "2026-08-22T14:52:04.900736Z"
updated_at: "2026-08-22T15:19:34.775818Z"
---

## Body

Добавить глобальную repository allowlist-политику для Jira и Confluence. Сервер получает нормализованный startup repo context до построения MCP tool surface и регистрирует integration tools только когда integration enabled и текущий локальный repository явно указан в allowed_repositories. Пустой или отсутствующий allowlist запрещает integration tools во всех проектах. Не смешивать локальный repository scope с Jira allowed_projects и Confluence allowed_spaces.

## Acceptance Criteria

- Глобальные Jira и Confluence config поддерживают документированный allowed_repositories с нормализацией путей.
- Startup repo context передаётся в MCP server до регистрации tools; tools видны только для явно разрешённого repository.
- Пустой или отсутствующий allowed_repositories запрещает Jira/Confluence tools во всех проектах, включая неизвестный repository context.
- allowed_projects и allowed_spaces продолжают независимо ограничивать удалённые сущности.
- Config schema/example и targeted tests покрывают explicit allow, denied, unknown и empty-list deny cases.

## Verification Plan

1. Запустить targeted Go tests для internal/config и internal/mcp по новым fail-closed сценариям.
2. Запустить обязательный make quality и проверить happ diagnostics на изменённых Go-файлах.
