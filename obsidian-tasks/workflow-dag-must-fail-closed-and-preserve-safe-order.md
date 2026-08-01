---
id: workflow-dag-must-fail-closed-and-preserve-safe-order
title: Сделать workflow DAG fail-closed и исключить проверку старого кода
status: in_progress
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - dag
    - concurrency
    - safety
    - llm
acceptance_criteria:
    - Unknown dependency ids and duplicate step ids fail before any step executes.
    - Dependency cycles fail before any step executes instead of falling back to one parallel wave.
    - Canonical workflow documentation makes edit -> check -> task transition -> commit ordering explicit.
    - Independent valid steps continue to run concurrently.
verification_plan:
    - Run focused pipeline DAG/concurrency tests.
    - Run focused MCP workflow schema/example tests if changed.
created_at: "2026-08-01T11:19:08.030851Z"
updated_at: "2026-08-01T11:30:15.989062Z"
---

## Body

Validate workflow step graphs before execution. Reject duplicate ids, unknown depends_on/condition references, and cycles instead of running all steps concurrently. Ensure the canonical edit-check-transition-commit example encodes explicit dependencies so checks cannot race edits and commits cannot race task finalization.

## Acceptance Criteria

- Unknown dependency ids and duplicate step ids fail before any step executes.
- Dependency cycles fail before any step executes instead of falling back to one parallel wave.
- Canonical workflow documentation makes edit -> check -> task transition -> commit ordering explicit.
- Independent valid steps continue to run concurrently.

## Verification Plan

1. Run focused pipeline DAG/concurrency tests.
2. Run focused MCP workflow schema/example tests if changed.
