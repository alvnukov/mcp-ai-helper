---
id: tools-deny-must-cover-all-local-tools
title: tools.deny должен покрывать все локальные инструменты
status: done
priority: high
model_level: high
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - security
    - policy
    - deny
    - dispatch
acceptance_criteria:
    - repo-local tools.deny проверяется в одной точке (dispatcher) для file/edit/git/task/issue/config/command/lake/pipeline/web
    - сообщение об ошибке единообразно
    - существующие разрешения не меняются
    - тесты на deny для каждой группы
created_at: "2026-08-14T21:18:50.504181Z"
updated_at: "2026-08-14T22:00:25.028864Z"
---

## Body

server.go:143,162 + web_tools.go — repo-local deny проверяют только command/lake/pipeline/web (opt-in в трёх runner'ах); file/edit/git/task/issue/config игнорируют запрет. Запрет git/edit в repo-конфиге не работает.

## Acceptance Criteria

- repo-local tools.deny проверяется в одной точке (dispatcher) для file/edit/git/task/issue/config/command/lake/pipeline/web
- сообщение об ошибке единообразно
- существующие разрешения не меняются
- тесты на deny для каждой группы
