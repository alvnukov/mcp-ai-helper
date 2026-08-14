---
id: guarded-replace-must-support-crlf-files
title: guarded_replace должен работать на CRLF-файлах
status: done
created_at: "2026-08-14T21:18:50.49929Z"
updated_at: "2026-08-14T21:47:51.622872Z"
---

## Body

safe_edit.go:340,379 vs 286 — чтение показывает LF-нормализованные строки, ApplyGuardedReplace матчит сырые байты: на CRLF-файле каждый эдит «конфликтует» при визуально идентичном тексте. Ядро инструмента (guarded edit) неработоспособно на классе файлов.
