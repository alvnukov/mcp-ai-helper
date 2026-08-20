---
id: rg-search-gitignore-types
title: Gitignore-каскад, типы и режимы матчей в rg-подобном поиске
status: done
priority: high
task_type: feature
tags:
    - fileops
    - search
    - rg
    - gitignore
    - implementation
created_at: "2026-08-20T10:58:59.831033Z"
updated_at: "2026-08-20T11:07:53.706605Z"
---

## Body

Расширение file action=search до полного rg-подобия: (1) каскад .gitignore/.ignore/.rgignore с семантикой git (negation last-match-wins, dir-only, anchoring, **, prune ignored-директорий; no_ignore отключает; явно указанный файл обходит правила); (2) типы -t/-T; (3) NUL-сниф бинарности; (4) word_match/-w, line_regexp/-x, count_only/-c, only_matching/-o + replace/-r. Своя реализация парсера gitignore без новых зависимостей.
