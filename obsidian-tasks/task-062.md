---
id: task-062
title: Jira: создать MCP issue тулы (search, read, update, transition, assign)
status: done
priority: high
task_type: feature
tags:
  - jira
  - mcp
  - issues
branch: feature/task-062
worktree_path: .worktrees/task-062
created_at: 2026-05-14T09:15:25.481251Z
updated_at: 2026-05-14T09:15:25.481251Z
---

## Body

В internal/mcp/jira_tools.go добавить 5 issue тулов:
- jira_search: jql + max_results → список issues
- jira_read: issue_key → полный issue с переходами
- jira_update: issue_key + опциональные поля (summary, description, priority, labels, components, fixVersions, custom_fields)
- jira_transition: issue_key + transition_name → сменить статус
- jira_assign: issue_key + username | unassign

Каждый тул:
- bind аргументов в строгую структуру
- вызов методов internal/jira.Client
- возврат structured результата
- при ошибке: compact error с причиной
