---
id: task-124
title: Спроектировать secret reference contract и fail-closed redaction policy
status: done
priority: critical
model_level: medium
task_type: design
parent_id: task-122
tags:
  - security
  - secrets
  - config
  - pipeline
  - workflow
  - design
  - redaction
branch: design/task-124
worktree_path: .worktrees/task-124
acceptance_criteria:
  - The implementation follows handle-only model-facing references and fail-closed secret resolution.
  - Redaction policy covers command output and retained command history.
verification_plan:
  - Schema and pipeline tests verify the contract.
created_at: 2026-05-14T09:15:25.491062Z
updated_at: 2026-05-14T09:15:25.491062Z
---

## Body

Completed contract: server config owns secret values, model-facing requests pass secret_handles, commands receive HELPER_SECRET_<HANDLE>, missing/invalid handles fail closed, outputs/history redact configured secret values.

## Acceptance Criteria

- The implementation follows handle-only model-facing references and fail-closed secret resolution.
- Redaction policy covers command output and retained command history.

## Verification Plan

1. Schema and pipeline tests verify the contract.
