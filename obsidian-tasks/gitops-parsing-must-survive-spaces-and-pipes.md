---
id: gitops-parsing-must-survive-spaces-and-pipes
title: Парсинг gitops должен переживать пробелы и |
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - gitops
    - parsing
acceptance_criteria:
    - porcelain v2 unmerged (u) записи парсят пути с пробелами (tab-override как в case 1/2)
    - git log не использует | как сепаратор (или экранирует), имя автора с | не сдвигает поля
    - тесты на оба случая
created_at: "2026-08-14T21:18:50.505607Z"
updated_at: "2026-08-14T21:35:46.109614Z"
---

## Body

gitops/git_status.go:108-112 — у записей u путь берётся из strings.Fields(parts[10]) без tab-override; git_log.go:51,80 — %an|%ai|%s рвётся на | в имени автора.

## Acceptance Criteria

- porcelain v2 unmerged (u) записи парсят пути с пробелами (tab-override как в case 1/2)
- git log не использует | как сепаратор (или экранирует), имя автора с | не сдвигает поля
- тесты на оба случая
