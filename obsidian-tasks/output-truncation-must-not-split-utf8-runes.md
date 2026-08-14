---
id: output-truncation-must-not-split-utf8-runes
title: Обрезка вывода не должна рвать UTF-8 руны
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - commands
    - output
    - utf8
    - token-economy
acceptance_criteria:
    - общий UTF-8-безопасный truncation helper
    - limitBuffer.Write и composeHandoff режут по границе руны
    - тест на multi-byte вывод в конце буфера
created_at: "2026-08-14T21:18:50.508194Z"
updated_at: "2026-08-14T21:31:49.14074Z"
---

## Body

command/runner.go:793 и pipeline.go:1553 режут по байтовой границе: multi-byte руна в конце рвётся, JSON-кодирует U+FFFD — испорченная последняя строка вместо чистой границы.

## Acceptance Criteria

- общий UTF-8-безопасный truncation helper
- limitBuffer.Write и composeHandoff режут по границе руны
- тест на multi-byte вывод в конце буфера
