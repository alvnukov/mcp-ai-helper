---
id: task-125
title: Реализовать server-config secret handles и pipeline injection MVP по failing tests
status: done
priority: critical
model_level: medium
task_type: implementation
parent_id: task-122
tags:
  - security
  - secrets
  - config
  - pipeline
  - workflow
  - implementation
  - tests
  - tdd
branch: implementation/task-125
worktree_path: .worktrees/task-125
acceptance_criteria:
  - Secret handles are resolved server-side only.
  - Unknown handles fail before command execution.
  - Backward-compatible no-secret pipeline/workflow behavior remains covered by existing tests.
verification_plan:
  - go test ./internal/pipeline
created_at: 2026-05-14T09:15:25.491195Z
updated_at: 2026-05-14T09:15:25.491195Z
---

## Body

Completed. Configured handles resolve to HELPER_SECRET_<HANDLE> environment variables for run_pipeline/run_workflow command execution, with fail-closed unknown handle behavior.

## Acceptance Criteria

- Secret handles are resolved server-side only.
- Unknown handles fail before command execution.
- Backward-compatible no-secret pipeline/workflow behavior remains covered by existing tests.

## Verification Plan

1. go test ./internal/pipeline
