---
id: command-history-must-survive-long-lines
title: Индекс истории команд должен переживать строки >64KB
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - commands
    - history
    - persistence
acceptance_criteria:
    - чтение индекса не использует 64KB-лимит bufio.Scanner (строка любой длины читается)
    - команда >64KB не отключает персистентность молча
    - деградация durable history (fallback in-memory) пишет предупреждение в stderr, как project store в server.go:125
    - in-memory List считает Total до лимита (консистентная пагинация)
    - 'тест: запись+чтение индекса со строкой >64KB'
created_at: "2026-08-14T21:18:50.495049Z"
updated_at: "2026-08-14T21:40:56.11011Z"
---

## Body

history.go:640-667 — bufio.Scanner с дефолтным 64KB; Put пишет полную строку команды без лимита. Одна команда >64KB → каждое чтение индекса падает, NewRunnerWithMask молча уходит in-memory, List/Cleanup/previous ломаются и не самоисцеляются. Плюс history.go:490 — in-memory Total после лимита.

## Acceptance Criteria

- чтение индекса не использует 64KB-лимит bufio.Scanner (строка любой длины читается)
- команда >64KB не отключает персистентность молча
- деградация durable history (fallback in-memory) пишет предупреждение в stderr, как project store в server.go:125
- in-memory List считает Total до лимита (консистентная пагинация)
- тест: запись+чтение индекса со строкой >64KB
