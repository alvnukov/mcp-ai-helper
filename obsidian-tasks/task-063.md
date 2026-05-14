---
id: task-063
title: Jira: создать MCP worklog тулы (list, report, add, update, delete)
status: done
priority: high
task_type: feature
tags:
  - jira
  - mcp
  - worklog
branch: feature/task-063
worktree_path: .worktrees/task-063
created_at: 2026-05-14T09:15:25.481377Z
updated_at: 2026-05-14T09:15:25.481377Z
---

## Body

В internal/mcp/jira_tools.go (или jira_worklog_tools.go) добавить 5 worklog тулов:
- jira_worklog_list: issue_key, since, until, username → список worklog записей
- jira_worklog_report: username, since, until → сводка по задачам + тотал часов
- jira_worklog_add: issue_key, time_spent (Jira формат: 1h 30m), comment, started
- jira_worklog_update: issue_key, worklog_id, time_spent?, comment?
- jira_worklog_delete: issue_key, worklog_id

Особое внимание:
- time_spent парсинг и валидация (формат Jira: '1h 30m', '2d', '4h')
- report: агрегация по issues, человекочитаемая сводка
- started: опциональное время начала (по умолчанию now)
