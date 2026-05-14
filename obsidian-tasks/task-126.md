---
id: task-126
title: Добавить centralized redaction для secrets по failing non-disclosure tests
status: done
priority: critical
model_level: medium
task_type: implementation
parent_id: task-122
tags:
  - security
  - secrets
  - redaction
  - outputs
  - workflow
  - implementation
  - tests
  - tdd
branch: implementation/task-126
worktree_path: .worktrees/task-126
acceptance_criteria:
  - Configured secret values are masked from model-facing command output and history.
  - Per-handle injection output is masked with [HELPER_SECRET:<HANDLE>].
  - Short/common values are not added to the global mask.
verification_plan:
  - go test ./internal/command ./internal/pipeline ./internal/security
created_at: 2026-05-14T09:15:25.491325Z
updated_at: 2026-05-14T09:15:25.491325Z
---

## Body

Completed and review-hardened. Command output/history now receives a base mask from server config plus per-request handle masks, so configured secrets are masked even when printed without an explicit handle.

## Acceptance Criteria

- Configured secret values are masked from model-facing command output and history.
- Per-handle injection output is masked with [HELPER_SECRET:<HANDLE>].
- Short/common values are not added to the global mask.

## Verification Plan

1. go test ./internal/command ./internal/pipeline ./internal/security
