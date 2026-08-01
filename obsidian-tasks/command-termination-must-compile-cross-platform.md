---
id: command-termination-must-compile-cross-platform
title: Сделать command termination cross-platform без потери process-tree cleanup
status: done
priority: critical
model_level: high
task_type: bug
parent_id: release-v1-0-0-with-homebrew
tags:
    - commands
    - windows
    - process-lifecycle
    - timeout
    - release-blocker
acceptance_criteria:
    - Общий runner.go не импортирует и не использует Unix-only syscall API
    - Unix timeout descendant test сохраняет terminal timeout semantics и process-tree cleanup
    - Windows amd64 и arm64 command package/binary cross-compile без ошибок
    - Windows cancellation пытается завершить дерево процессов и безопасно обрабатывает уже завершившийся процесс
verification_plan:
    - go test ./internal/command -count=1 -race -timeout=120s
    - GOOS=windows GOARCH=amd64 и arm64 go build ./cmd/mcp-ai-helper
    - make quality и make lint
created_at: "2026-08-01T17:21:44.342083Z"
updated_at: "2026-08-01T17:25:09.189909Z"
---

## Body

Release cross-build выявил Unix-only syscall.Setpgid/Kill в общем runner.go. Вынести termination policy в OS-specific implementation: Unix сохраняет process-group SIGKILL, Windows использует taskkill /T /F с fallback Process.Kill.

## Acceptance Criteria

- Общий runner.go не импортирует и не использует Unix-only syscall API
- Unix timeout descendant test сохраняет terminal timeout semantics и process-tree cleanup
- Windows amd64 и arm64 command package/binary cross-compile без ошибок
- Windows cancellation пытается завершить дерево процессов и безопасно обрабатывает уже завершившийся процесс

## Verification Plan

1. go test ./internal/command -count=1 -race -timeout=120s
2. GOOS=windows GOARCH=amd64 и arm64 go build ./cmd/mcp-ai-helper
3. make quality и make lint
