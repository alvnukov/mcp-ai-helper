---
id: task-064
title: Jira: wire up server.go + config example + tests
status: done
priority: high
task_type: feature
tags:
  - jira
  - integration
  - tests
branch: feature/task-064
worktree_path: .worktrees/task-064
created_at: 2026-05-14T09:15:25.481499Z
updated_at: 2026-05-14T09:15:25.481499Z
---

## Body

1. В server.go: если cfg.Integrations.Jira не nil и enabled, вызвать registerJiraTools(srv, deps)
2. В Server struct добавить jiraClient *jira.Client (или создавать lazy)
3. Обновить configs/config.example.yaml — секция integrations.jira
4. Тесты:
   - internal/jira/client_test.go: unit тесты с httptest.NewServer (мок Jira API)
   - internal/mcp/jira_tools_test.go: интеграционные тесты тулов с мок-сервером
5. go test ./internal/jira/... && go test ./internal/mcp/... -run Jira
6. ruff check + mypy (не актуально для Go, пропускаем)
7. golangci-lint run
