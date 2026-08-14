---
id: git-advanced-gate-must-not-race-config-reload
title: Проверка git_advanced слоя не должна гоняться с config_reload
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - concurrency
    - git
    - layers
acceptance_criteria:
    - gitAdvancedAction читает cfg только через loadDeps() под RLock
    - тест с параллельной записью cfg под Lock и вызовом advanced-action проходит под -race
created_at: "2026-08-14T21:18:50.491352Z"
updated_at: "2026-08-14T21:21:53.15792Z"
---

## Body

git_tools.go:50 читает deps.cfg.LayerEnabled без deps.mu, тогда как reloadConfig (server.go:209-219) пишет deps.cfg под lock. Data race по Go memory model; ловится -race на параллельных git action=log + config_reload.

## Acceptance Criteria

- gitAdvancedAction читает cfg только через loadDeps() под RLock
- тест с параллельной записью cfg под Lock и вызовом advanced-action проходит под -race
