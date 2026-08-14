---
id: ci-govulncheck-require-patched-go-1-26-5
title: Обновить CI на Go 1.26.5 для устранения stdlib vulnerabilities
status: done
priority: critical
model_level: low
task_type: ci
tags:
    - ci
    - security
    - govulncheck
    - go
    - hotfix
acceptance_criteria:
    - go.mod требует Go 1.26.5 или новее
    - govulncheck не сообщает достижимых уязвимостей
    - make quality проходит
    - Фикс и финальный статус задачи зафиксированы одним owned-files commit
verification_plan:
    - go version подтверждает Go 1.26.5 toolchain после переключения
    - govulncheck ./...
    - make quality
created_at: "2026-08-01T20:10:49.538365Z"
updated_at: "2026-08-01T20:12:03.178488Z"
---

## Body

GitHub CI run 30716035941 проходит format/test/vet/build, затем govulncheck на Go 1.26.3 находит четыре достижимые уязвимости stdlib, исправленные в Go 1.26.5. Поднять минимальный patch-level toolchain без ослабления security gate.

## Acceptance Criteria

- go.mod требует Go 1.26.5 или новее
- govulncheck не сообщает достижимых уязвимостей
- make quality проходит
- Фикс и финальный статус задачи зафиксированы одним owned-files commit

## Verification Plan

1. go version подтверждает Go 1.26.5 toolchain после переключения
2. govulncheck ./...
3. make quality
