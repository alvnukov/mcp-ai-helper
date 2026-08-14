---
id: review-2026-08-helper-hardening
title: 'Эпик: устранение находок полного ревью хелпера 2026-08'
status: in_progress
priority: high
model_level: medium
task_type: epic
tags:
    - review
    - hardening
    - bugs
    - epic
created_at: "2026-08-14T21:18:50.488231Z"
updated_at: "2026-08-14T21:18:50.488231Z"
---

## Body

Полное ревью кодовой базы (code-review skill, 2026-08-14) нашло 5 high-багов, 3 нарушения документированных контрактов, ~9 medium, ~12 low. Каждый дочерний билет — вертикальный срез: красный тест → фикс → gate → transition → атомарный commit (код + файл реестра в одном коммите). Цель сервера — token economy без потери grounding, policy-first fail-closed: каждый фикс усиливает эти свойства.
