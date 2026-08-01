---
id: workflow-guarded-edits-must-be-atomic
title: Workflow guarded edits must rollback on pre-gate failure
status: todo
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - atomicity
    - llm-reliability
    - data-integrity
acceptance_criteria:
    - All file mutations in a workflow are atomic before command/gate execution
    - A failed guarded_replace restores every earlier file mutation from the same workflow
    - Failure evidence identifies the failed step and reports rollback outcome
    - Regression test covers two edits where the second conflicts
verification_plan:
    - Run targeted workflow transaction tests
    - Run make quality
    - Run golangci-lint
created_at: "2026-08-01T12:41:31.27252Z"
updated_at: "2026-08-01T12:41:31.27252Z"
---

## Body

A multi-step workflow can apply an earlier guarded_replace and then fail on a later guarded_replace, leaving the worktree partially mutated before gates run. Reproduced while editing setup.go/setup_test.go: imports were applied, the production edit conflicted, and no rollback occurred.

## Acceptance Criteria

- All file mutations in a workflow are atomic before command/gate execution
- A failed guarded_replace restores every earlier file mutation from the same workflow
- Failure evidence identifies the failed step and reports rollback outcome
- Regression test covers two edits where the second conflicts

## Verification Plan

1. Run targeted workflow transaction tests
2. Run make quality
3. Run golangci-lint
