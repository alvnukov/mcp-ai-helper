---
id: delete-exact-block-must-not-touch-unrelated-newlines
title: delete_block не должен схлопывать чужие переводы строк
status: done
priority: high
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - fileops
    - edit
    - integrity
acceptance_criteria:
    - нормализация \n\n\n→\n\n применяется только к месту удаления блока, не ко всему файлу
    - тройной перевод строки в невредимой части файла сохраняется
    - тест на файл с двумя независимыми местами
created_at: "2026-08-14T21:18:50.499941Z"
updated_at: "2026-08-14T21:47:51.833979Z"
---

## Body

safe_edit.go:780 — DeleteExactBlock глобально схлопывает \n\n\n→\n\n по всему файлу, молча переписывая чужой контент (например, внутри строкового литерала) при удалении одного блока.

## Acceptance Criteria

- нормализация \n\n\n→\n\n применяется только к месту удаления блока, не ко всему файлу
- тройной перевод строки в невредимой части файла сохраняется
- тест на файл с двумя независимыми местами
