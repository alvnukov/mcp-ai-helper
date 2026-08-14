---
id: ci-format-gate-must-pass-v1-0-1
title: Исправить падение CI format gate после релиза v1.0.1
status: done
priority: critical
model_level: low
task_type: ci
tags:
    - ci
    - format
    - gofmt
    - release
    - hotfix
acceptance_criteria:
    - gofmt -l . не выводит файлов
    - Go quality gate проходит
    - Исправление и финальный статус задачи зафиксированы одним owned-files commit
verification_plan:
    - test -z "$(gofmt -l .)"
    - make quality
created_at: "2026-08-01T19:59:32.541909Z"
updated_at: "2026-08-01T20:00:26.580576Z"
---

## Body

Последние CI runs на main падают на Format check: gofmt -l сообщает internal/mcp/guidance.go. Отформатировать файл, проверить CI-equivalent gate и закрыть задачу атомарно.

## Acceptance Criteria

- gofmt -l . не выводит файлов
- Go quality gate проходит
- Исправление и финальный статус задачи зафиксированы одним owned-files commit

## Verification Plan

1. test -z "$(gofmt -l .)"
2. make quality
