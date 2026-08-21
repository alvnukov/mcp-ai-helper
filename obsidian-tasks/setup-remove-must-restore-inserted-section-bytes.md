---
id: setup-remove-must-restore-inserted-section-bytes
title: remove должен восстанавливать байты, когда mcp-секцию вставил сам setup
status: done
priority: high
model_level: medium
task_type: bug
tags:
    - setup
    - json
    - surgical-edit
    - round-trip
    - release
created_at: "2026-08-21T17:07:37.991021Z"
updated_at: "2026-08-21T17:10:18.845249Z"
---

## Body

E2E релизного артефакта v1.2.3 нашёл асимметрию insert/delete в internal/setup/jsonedit.go: jsonInsertMember ставит ",\n<indent>member" непосредственно перед закрывающей "}", т.е. запятая оказывается ПОСЛЕ исходного "\n", а jsonDeleteMember для последнего члена режет от prevValEnd и съедает этот "\n". Итог: setup→remove на конфиге без mcp-секции не восстанавливает байты ("value"} вместо "value"\n}); репро: /tmp/rel123/mini. Фикс минимальный — только jsonDeleteMember (case i>0): скипать whitespace после prevValEnd до разделяющей запятой и резать с неё (fallback keyStart); insert не трогать (его форма валидна в любом gap, включая комментарии). Угол JSONC: dangling comma поглощается как сепаратор и уходит при remove — закрепить ожиданием в тесте. Регресс-тесты: insert-then-remove байт-точен для pretty/двухчленного/single-line/claude/секция-с-чужим-entry. Гейт: gofmt+vet+test setup+golangci; далее push main и тег v1.2.4, E2E повтоить на артефакте.
