---
id: task-012
title: Добавить декларативное условное выполнение workflow
status: done
priority: critical
tags:
  - workflow
  - conditionals
  - reliability
  - tasks
worktree_path: .worktrees/task-012
created_at: 2026-05-14T09:15:25.476434Z
updated_at: 2026-05-14T09:15:25.476434Z
---

## Body

Реализовать декларативное ветвление workflow, чтобы вызывающий мог выражать if/then/else решения внутри одного run_workflow: ветвиться по коду выхода команды, совпадению в выводе, состоянию файла, состоянию задачи, результату валидации и состоянию изменённых файлов. Это нужно для workflow senior-моделей, где правки, проверки, task transitions и commit должны выполняться сервером детерминированно, а не через повторные ручные вызовы инструментов.
