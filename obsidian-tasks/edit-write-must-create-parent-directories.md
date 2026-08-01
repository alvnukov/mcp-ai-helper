---
id: edit-write-must-create-parent-directories
title: Исправить расхождение edit.write при создании родительских каталогов
status: todo
priority: high
model_level: medium
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - edit
    - files
    - contract
    - surface
    - reliability
acceptance_criteria:
    - edit action=write создаёт отсутствующие родительские каталоги внутри repo root.
    - Guarded confinement и запрет symlink traversal сохраняются.
    - Ошибка содержит структурированное различие между небезопасным путём и отсутствующим разрешённым parent.
verification_plan:
    - Добавить интеграционный тест записи файла в новый вложенный каталог.
    - Проверить отказ для parent symlink, ведущего наружу, и выполнить make quality.
created_at: "2026-08-01T14:11:14.263176Z"
updated_at: "2026-08-01T14:11:14.263176Z"
---

## Body

Canonical edit.write contract обещает создавать parent directories, однако запись нового файла internal/safefs/root.go завершилась ошибкой lstat родительского каталога. Модель вынуждена обходить штатный edit surface через command, что ухудшает надёжность и безопасность.

## Acceptance Criteria

- edit action=write создаёт отсутствующие родительские каталоги внутри repo root.
- Guarded confinement и запрет symlink traversal сохраняются.
- Ошибка содержит структурированное различие между небезопасным путём и отсутствующим разрешённым parent.

## Verification Plan

1. Добавить интеграционный тест записи файла в новый вложенный каталог.
2. Проверить отказ для parent symlink, ведущего наружу, и выполнить make quality.
