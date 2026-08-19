---
id: obsidian-task-registry-by-default
title: 'Obsidian task registry по умолчанию: авто-создание при первом обращении и дефолтный конфиг'
status: done
priority: high
model_level: medium
task_type: feature
tags:
    - tasks
    - obsidian
    - config
    - defaults
    - registry
created_at: "2026-08-19T20:39:43.116165Z"
updated_at: "2026-08-19T20:54:30.580701Z"
---

## Body

Требования пользователя: (1) task_registry.backend по умолчанию obsidian c дефолтным путём obsidian-tasks, резолвимым относительно repo_path; (2) при первом обращении к таскам реестр создаётся молча, а ответ модели сообщает registry_created/registry_path/registry_note (реестр создан и пуст); (3) дефолтный конфиг в ~/.mcp-ai-helper/config.yaml создаётся при первом запуске (уже есть через ensureDefaultConfigFile) и содержит явный блок task_registry: backend obsidian. Изменения: internal/config (applyDefaults, defaultConfigYAML, schema, тесты), internal/mcp (taskListMetadata + registry_created поля, obsidian backend repo-relative dirFor/forRepo, taskListResponse note, тесты).
