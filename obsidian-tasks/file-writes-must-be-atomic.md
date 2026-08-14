---
id: file-writes-must-be-atomic
title: Запись файлов должна быть атомарной (temp+rename)
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - fileops
    - safefs
    - integrity
    - contract
acceptance_criteria:
    - единый helper атомарной записи (temp в том же каталоге + fsync + rename) в safefs/fileops
    - все точки записи safe_edit.go (replace, write, delete-block и др.) и setup.go конфиг-записи используют его
    - крах посреди записи не оставляет обрезанный целевой файл (тест с падающим write)
    - права файла сохраняются/задаются корректно
created_at: "2026-08-14T21:18:50.497041Z"
updated_at: "2026-08-14T21:51:55.76615Z"
---

## Body

safe_edit.go:294,719,782,861 и setup/setup.go:501 — os.WriteFile in-place (truncate+write). docs/file-edit-contracts.md Implementation Notes #2 требуют temp+rename: крах посреди записи обрезает исходник или ~/.codex/config.toml при том, что hash-guard продаётся как защита целостности.

## Acceptance Criteria

- единый helper атомарной записи (temp в том же каталоге + fsync + rename) в safefs/fileops
- все точки записи safe_edit.go (replace, write, delete-block и др.) и setup.go конфиг-записи используют его
- крах посреди записи не оставляет обрезанный целевой файл (тест с падающим write)
- права файла сохраняются/задаются корректно
