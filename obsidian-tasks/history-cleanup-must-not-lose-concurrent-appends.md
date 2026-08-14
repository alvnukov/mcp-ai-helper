---
id: history-cleanup-must-not-lose-concurrent-appends
title: Cleanup истории не должен терять конкурентные записи (дизайн)
status: todo
priority: medium
model_level: high
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - commands
    - history
    - concurrency
    - design
acceptance_criteria:
    - выбрана и задокументирована схема межпроцессной координации (lock-файл / append-only журнал / однокаталожная очередь)
    - терминальная запись, дописанная вторым процессом между read и rename, не теряется
    - List не показывает завершённую команду running
created_at: "2026-08-14T21:18:50.509846Z"
updated_at: "2026-08-14T21:18:50.509846Z"
---

## Body

history.go:437 — Cleanup переписывает индекс из снапшота без межпроцессной блокировки: вторая копия хелпера, дописавшая terminal-строку между readEntries и rename, теряет её навсегда. Требует дизайн-решения (design-heavy, не фиксится наспех).

## Acceptance Criteria

- выбрана и задокументирована схема межпроцессной координации (lock-файл / append-only журнал / однокаталожная очередь)
- терминальная запись, дописанная вторым процессом между read и rename, не теряется
- List не показывает завершённую команду running
