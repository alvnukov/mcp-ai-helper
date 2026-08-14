---
id: confluence-client-must-be-built-at-startup
title: Собирать Confluence клиент при старте сервера, а не только на config reload
status: done
priority: high
model_level: medium
task_type: bug
tags:
    - confluence
    - integrations
    - startup
    - config-reload
    - mcp
    - bug
verification_plan:
    - 'Красный прогон нового теста до фикса: ожидается ''confluence: not configured'''
    - Зелёный прогон после фикса, go vet, затем task transition в done и git_commit_owned в одном run action=workflow
created_at: "2026-08-14T20:45:47.185666Z"
updated_at: "2026-08-14T20:49:52.304251Z"
---

## Body

Диагноз подтверждён на живом сервере: conf_* инструменты регистрируются из стартового конфига (internal/mcp/server.go:247), но клиент собирается только внутри замыкания reloadConfig (internal/mcp/server.go:214) — то есть на config_reload / config_option_set. Конструктор New() (internal/mcp/server.go:184) собирает jiraClient, но не confluenceClient/confluenceClientErr, поэтому каждый вызов на свежем сервере получает "confluence: not configured" до первого релоада. Воспроизведено: conf_spaces до релоада → "confluence: not configured"; после config_reload → запрос доходит до реального API.

Фикс: строить confluence клиент в New() рядом со сборкой jiraClient. Регрессионный тест через New(cfg) + in-process вызов conf_spaces против httptest-стаба Confluence.

Out of scope (отдельное наблюдение, не фиксить здесь): reloadConfig регистрируется только под слоем layers.models, а conf_* инструменты — безусловно; при выключенном models слое оживить Confluence нечем.

## Verification Plan

1. Красный прогон нового теста до фикса: ожидается 'confluence: not configured'
2. Зелёный прогон после фикса, go vet, затем task transition в done и git_commit_owned в одном run action=workflow
