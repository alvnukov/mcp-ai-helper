---
id: task-061
title: Jira: создать internal/jira/client.go — обёртку над go-jira
status: done
priority: high
task_type: feature
tags:
  - jira
  - client
branch: feature/task-061
worktree_path: .worktrees/task-061
created_at: 2026-05-14T09:15:25.481126Z
updated_at: 2026-05-14T09:15:25.481126Z
---

## Body

Создать пакет internal/jira с Client:
- NewClient(cfg JiraConfig) (*Client, error) — подключение с базовой аутентификацией
- SearchIssues(jql string, maxResults int) ([]Issue, error)
- GetIssue(key string, fields []string) (*Issue, error)
- UpdateIssue(key string, fields map[string]interface{}) error
- GetTransitions(key string) ([]Transition, error)
- DoTransition(key string, transitionName string) error
- AssignIssue(key string, username string) error
- UnassignIssue(key string) error

Для worklog:
- GetWorklogs(key string, since, until time.Time) ([]Worklog, error)
- GetWorklogsByUser(username string, since, until time.Time) — через search JQL + сбор worklog'ов
- AddWorklog(key string, timeSpent string, comment string, started *time.Time) (*Worklog, error)
- UpdateWorklog(key string, worklogID string, timeSpent *string, comment *string) error
- DeleteWorklog(key string, worklogID string) error

Все методы должны возвращать wrapped errors с контекстом.
