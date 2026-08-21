---
id: setup-must-edit-json-configs-surgically
title: setup/remove/status должны править JSON-конфиги клиентов хирургически (JSONC, порядок, чужие ключи и argv)
status: done
priority: high
model_level: medium
task_type: bug
tags:
    - setup
    - opencode
    - claude
    - json
    - jsonc
    - config
    - surgical-edit
acceptance_criteria:
    - 'merge на существующем JSON/JSONC конфиге opencode/claude меняет байты только собственного entry: комментарии, порядок ключей, форматирование и чужие записи остаются байт-в-байт'
    - 'argv entry мёржится: бинарник и пара --config управляются setup, пользовательские флаги (--repo) и ключи (timeout) сохраняются; повторный setup — фикспойнт (apply outcomeCurrent)'
    - JSONC-конфиг (комментарии, trailing comma) не ломает setup/status/remove; remove восстанавливает исходные байты конфига точно
    - для opencode выбирается существующий opencode.jsonc вместо создания второго файла opencode.json
    - go vet + go test ./internal/setup/... -race -short зелёные
verification_plan:
    - go vet ./internal/setup/...
    - go test ./internal/setup/... -count=1 -race -short
    - 'ручная проверка: go build && HOME=/tmp/sandbox bin/mcp-ai-helper setup -c opencode --global на копии живого конфига — diff только внутри entry'
created_at: "2026-08-21T10:58:33.858448Z"
updated_at: "2026-08-21T11:10:03.392494Z"
---

## Body

Диагноз (воспроизведено на песочнице): setup декодирует весь файл через encoding/json и перекодирует его целиком. Для OpenCode это (1) жёсткая ошибка на легальном JSONC-конфиге с комментариями, (2) молчаливая потеря пользовательских args (--repo) в command через mergeEntry, (3) пересортировка всех ключей файла по алфавиту и полная переформатировка (490-строчный конфиг пользователя перетасовывается), (4) opencode.jsonc игнорируется — создаётся конкурирующий opencode.json. Решение: хирургический span-редактор (как instructions-блок в markdown): JSONC-толерантный сканер с байтовыми спанами, splice только собственного entry; managed-ключи type/command/enabled/args и пара --config; пользовательское — нетронуто. TOML-путь без изменений.

## Acceptance Criteria

- merge на существующем JSON/JSONC конфиге opencode/claude меняет байты только собственного entry: комментарии, порядок ключей, форматирование и чужие записи остаются байт-в-байт
- argv entry мёржится: бинарник и пара --config управляются setup, пользовательские флаги (--repo) и ключи (timeout) сохраняются; повторный setup — фикспойнт (apply outcomeCurrent)
- JSONC-конфиг (комментарии, trailing comma) не ломает setup/status/remove; remove восстанавливает исходные байты конфига точно
- для opencode выбирается существующий opencode.jsonc вместо создания второго файла opencode.json
- go vet + go test ./internal/setup/... -race -short зелёные

## Verification Plan

1. go vet ./internal/setup/...
2. go test ./internal/setup/... -count=1 -race -short
3. ручная проверка: go build && HOME=/tmp/sandbox bin/mcp-ai-helper setup -c opencode --global на копии живого конфига — diff только внутри entry
