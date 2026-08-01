---
id: task-context-truncation-must-report-exact-omissions
title: Task context truncation must report exact omissions
status: in_progress
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - tasks
    - context
    - truncation
    - metadata
    - llm-reliability
acceptance_criteria:
    - Every max_nodes and max_bytes omission is counted in the matching Truncated field
    - AlreadyDone and PlannedNext preserve the documented priority/order while removing exactly the required number of items
    - A max_bytes-trimmed response always carries an explicit reason and per-section omission counts
    - Regression tests cover successful byte-limit trimming and each node-limit branch
verification_plan:
    - Run targeted task_context tests
    - Run make quality
    - Run golangci-lint delta and confirm revive empty-block finding is removed
created_at: "2026-08-01T13:05:12.971058Z"
updated_at: "2026-08-01T13:05:12.971058Z"
---

## Body

Task context limit enforcement mutates returned sections without reliable omission metadata. max_bytes has a no-op block and returns successful trimmed output with no Truncated marker; max_nodes computes prerequisite and related-task omission counts from already-truncated slices and trims the wrong ends/counts. This makes an LLM treat incomplete context as complete.

## Acceptance Criteria

- Every max_nodes and max_bytes omission is counted in the matching Truncated field
- AlreadyDone and PlannedNext preserve the documented priority/order while removing exactly the required number of items
- A max_bytes-trimmed response always carries an explicit reason and per-section omission counts
- Regression tests cover successful byte-limit trimming and each node-limit branch

## Verification Plan

1. Run targeted task_context tests
2. Run make quality
3. Run golangci-lint delta and confirm revive empty-block finding is removed
