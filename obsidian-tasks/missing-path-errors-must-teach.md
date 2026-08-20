---
id: missing-path-errors-must-teach
title: Ошибки несуществующего path в search/list должны подсказывать ближайший существующий каталог
status: done
priority: high
task_type: feature
tags:
    - fileops
    - errors
    - llm-reliability
    - ux
created_at: "2026-08-20T12:15:21.581744Z"
updated_at: "2026-08-20T12:20:10.80881Z"
---

## Body

Модель на несуществующем path в file action=search получает молчаливый 0 матчей (fs.WalkDir walkErr→nil), а list — сырой ENOENT; после чего выдумывает теории про repo_path. Сделать ошибки обучающими: (1) search path не существует → ошибка с ближайшим существующим каталогом и подсказкой list; (2) list ENOENT → та же подсказка; (3) описание repo_path в file/edit тулзах: любой читаемый каталог, git-корень не нужен.
