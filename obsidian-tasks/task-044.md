---
id: task-044
title: Добавить task execution packet и readiness contract
status: done
priority: critical
tags:
  - tasks
  - workflow
  - planning
  - llm-ergonomics
worktree_path: .worktrees/task-044
created_at: 2026-05-14T09:15:25.478659Z
updated_at: 2026-05-14T09:15:25.478659Z
---

## Body

Добавить server-supported способ сформировать compact execution packet для одной задачи перед началом implementation. Packet должен содержать minimal required context, acceptance criteria, owned files, forbidden files, known risks, required gates и указание, готова ли задача для standard model или требует strong model. Это формализует желаемый context-first и decision-before-execution workflow.
