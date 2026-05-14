---
id: task-022
title: Make task workflow guidance explicit in config and defaults
status: done
priority: high
tags:
  - tasks
  - guidance
  - workflow
worktree_path: .worktrees/task-022
created_at: 2026-05-14T09:15:25.477551Z
updated_at: 2026-05-14T09:15:25.477551Z
---

## Body

Ensure generated config guidance and setup guidance tell agents to read task_current first, batch task updates, keep statuses current, use close_missing only intentionally, avoid many separate task calls when one batch call is enough, and prefer pipeline-integrated task status updates only when the workflow has actually closed acceptance criteria and gates.
