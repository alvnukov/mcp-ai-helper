---
id: allow-current-repository-through-config-tool
title: Разрешать текущий проект для Jira/Confluence через config tool
status: done
priority: high
model_level: high
task_type: feature
tags:
    - config
    - jira
    - confluence
    - repository-scope
    - mcp
    - security
acceptance_criteria:
    - config_allow_repository доступен даже когда integration tools скрыты fail-closed политикой.
    - Tool принимает только jira, confluence или both и использует нормализованный startup repository path, а не произвольный путь от модели.
    - Операция идемпотентно добавляет путь в глобальный allowed_repositories, сохраняет secrets и не создаёт отсутствующую integration config.
    - Результат сообщает added/reloaded/restart_required; schema/annotations/manifest и targeted tests отражают новый tool.
verification_plan:
    - Запустить targeted Go tests для config_allow_repository, registration и schema/annotations.
    - Запустить make quality и happ diagnostics на изменённых Go-файлах.
created_at: "2026-08-22T15:26:47.661734Z"
updated_at: "2026-08-22T15:35:34.139601Z"
---

## Body

Добавить отдельный model-facing config_allow_repository, который по явному запросу пользователя идемпотентно добавляет startup repository path в глобальный allowed_repositories для Jira, Confluence или обеих интеграций. Tool не принимает произвольный repository path, не редактирует repo-local config и сообщает о необходимости restart для обновления tool surface.

## Acceptance Criteria

- config_allow_repository доступен даже когда integration tools скрыты fail-closed политикой.
- Tool принимает только jira, confluence или both и использует нормализованный startup repository path, а не произвольный путь от модели.
- Операция идемпотентно добавляет путь в глобальный allowed_repositories, сохраняет secrets и не создаёт отсутствующую integration config.
- Результат сообщает added/reloaded/restart_required; schema/annotations/manifest и targeted tests отражают новый tool.

## Verification Plan

1. Запустить targeted Go tests для config_allow_repository, registration и schema/annotations.
2. Запустить make quality и happ diagnostics на изменённых Go-файлах.
