---
id: task-057
title: Добавить task worktree workflow
status: done
priority: high
task_type: feature
tags:
  - git
  - workflow
  - worktree
  - tasks
branch: feature/task-057
worktree_path: .worktrees/task-057
created_at: 2026-05-14T09:15:25.480712Z
updated_at: 2026-05-14T09:15:25.480712Z
---

## Body

Реализовать helper workflow, где каждая задача получает явный кодовый контекст: task_type, branch вида <task_type>/<task_id>, worktree_path .worktrees/<task_id> и абсолютный code_path в task_current/task_get. Добавить git/workflow tool для создания или переиспользования worktree по этому контракту. Acceptance: .worktrees/ игнорируется git; task_current показывает .worktrees/<task_id> и code_path; branch не выдумывается без task_type; git_prepare_task_worktree создаёт branch <task_type>/<task_id> и checkout в .worktrees/<task_id>; targeted Go tests и lake build проходят.
