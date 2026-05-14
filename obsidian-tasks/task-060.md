---
id: task-060
title: Jira: добавить go-jira зависимость и JiraConfig в config
status: done
priority: high
task_type: feature
tags:
  - jira
  - config
branch: feature/task-060
worktree_path: .worktrees/task-060
created_at: 2026-05-14T09:15:25.481012Z
updated_at: 2026-05-14T09:15:25.481012Z
---

## Body

1. go get github.com/andygrunwald/go-jira@v1
2. Добавить IntegrationsConfig и JiraConfig в internal/config/config.go
3. Поля JiraConfig: url, username, api_key_env, enabled
4. Добавить LayerEnabled для jira
5. Обновить defaultConfigYAML и config.example.yaml
