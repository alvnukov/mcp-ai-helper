---
id: jira-comment-add-tool
title: Добавить инструмент jira_comment_add для комментариев в Jira-задачах
status: done
priority: high
model_level: medium
task_type: feature
tags:
    - jira
    - integrations
    - mcp
    - implementation
acceptance_criteria:
    - internal/jira client имеет AddComment/AddCommentContext с propagation cancellation и error joining по паттерну остальных методов
    - MCP tool jira_comment_add зарегистрирован с аннотацией addsRemote и guard'ом checkJiraMutate
    - annotations_test.go перечисляет jira_comment_add в группе addsRemote
    - Targeted go test (internal/jira, internal/mcp, -race -short) и make quality проходят
    - 'Финализация атомарна: task done + commit owned files в одном workflow'
verification_plan:
    - go test ./internal/jira/... ./internal/mcp/... -count=1 -race -short -timeout=300s
    - make quality (vet + test-short + build)
    - gofmt -l на изменённых пакетах пуст
    - git tag -a v1.1.3 + git push origin v1.1.3 запускает release workflow
created_at: "2026-08-18T07:46:26.512744Z"
updated_at: "2026-08-18T07:58:00.417699Z"
---

## Body

Добавить возможность оставлять комментарии к Jira-задачам: клиентский метод AddComment/AddCommentContext в internal/jira/client.go поверх go-jira AddCommentWithContext, MCP-инструмент jira_comment_add(issue_key, body) в internal/mcp/jira_tools.go, обновить annotations_test.go и добавить тесты клиента. Релиз патч-версии v1.1.3 через annotated tag (release tag-driven, версия инжектится ldflags).

## Acceptance Criteria

- internal/jira client имеет AddComment/AddCommentContext с propagation cancellation и error joining по паттерну остальных методов
- MCP tool jira_comment_add зарегистрирован с аннотацией addsRemote и guard'ом checkJiraMutate
- annotations_test.go перечисляет jira_comment_add в группе addsRemote
- Targeted go test (internal/jira, internal/mcp, -race -short) и make quality проходят
- Финализация атомарна: task done + commit owned files в одном workflow

## Verification Plan

1. go test ./internal/jira/... ./internal/mcp/... -count=1 -race -short -timeout=300s
2. make quality (vet + test-short + build)
3. gofmt -l на изменённых пакетах пуст
4. git tag -a v1.1.3 + git push origin v1.1.3 запускает release workflow
