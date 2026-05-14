---
id: task-127
title: Сначала добавить failing TDD tests для secret injection и non-disclosure
status: done
priority: critical
model_level: low
task_type: tests
parent_id: task-122
tags:
  - security
  - secrets
  - pipeline
  - workflow
  - tests
  - regression
  - tdd
  - low-model
branch: tests/task-127
worktree_path: .worktrees/task-127
acceptance_criteria:
  - Tests assert raw fake secrets are absent from serialized model-facing output.
  - Tests cover failure paths without command execution assumptions.
  - No real credentials are used.
verification_plan:
  - go test ./internal/pipeline ./internal/mcp ./internal/config ./internal/security
created_at: 2026-05-14T09:15:25.491451Z
updated_at: 2026-05-14T09:15:25.491451Z
---

## Body

Completed. Regression tests cover fake secret injection, echo redaction, unknown handle failure, config JSON non-disclosure, schema discoverability and review-found global masking/status closeout cases.

## Acceptance Criteria

- Tests assert raw fake secrets are absent from serialized model-facing output.
- Tests cover failure paths without command execution assumptions.
- No real credentials are used.

## Verification Plan

1. go test ./internal/pipeline ./internal/mcp ./internal/config ./internal/security
