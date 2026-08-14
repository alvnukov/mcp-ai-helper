---
id: layers-gates-must-match-documented-surface
title: Гейтинг слоёв должен соответствовать документированному surface
status: done
priority: critical
model_level: high
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - mcp
    - layers
    - surface
    - contract
acceptance_criteria:
    - layers.commands/workflows/guidance/tasks реально гейтят соответствующие группы инструментов при регистрации
    - models.enabled=false не убирает command/run/task/guidance
    - server_setup_guidance больше не обещает неработающие переключатели (или они работают)
    - тест на каждую комбинацию слоёв (wire/tool_names)
    - существующие дефолтные конфиги дают прежний surface без изменений
created_at: "2026-08-14T21:18:50.490552Z"
updated_at: "2026-08-14T21:54:55.354683Z"
---

## Body

server.go:195 — весь повседневный surface (command, run, task, config, guidance) зарегистрирован под layers.models; флаги layers.commands/workflows/guidance не читаются нигде. layers.commands.enabled=false не запрещает shell; models.enabled=false молча убирает command/task/run/config. Класс бага — тот же, что был у conf_* до фикса ef69f28, но в регистраторе.

## Acceptance Criteria

- layers.commands/workflows/guidance/tasks реально гейтят соответствующие группы инструментов при регистрации
- models.enabled=false не убирает command/run/task/guidance
- server_setup_guidance больше не обещает неработающие переключатели (или они работают)
- тест на каждую комбинацию слоёв (wire/tool_names)
- существующие дефолтные конфиги дают прежний surface без изменений
