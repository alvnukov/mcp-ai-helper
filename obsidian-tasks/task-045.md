---
id: task-045
title: Усилить семантику финализации задач в pipeline
status: done
priority: critical
tags:
  - tasks
  - pipeline
  - reliability
  - status
worktree_path: .worktrees/task-045
created_at: 2026-05-14T09:15:25.478843Z
updated_at: 2026-05-14T09:15:25.478843Z
---

## Body

Гарантировать, что run_pipeline и run_workflow никогда не помечают задачу done, если actual gate semantics не успешны. Evidence analysis success не равен command success. Non-zero command exits, invalid evidence, workflow conflicts, skipped required checks или failed commits должны оставлять задачу blocked или unchanged согласно explicit policy.
