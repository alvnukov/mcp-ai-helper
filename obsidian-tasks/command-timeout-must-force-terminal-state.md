---
id: command-timeout-must-force-terminal-state
title: Гарантировать terminal state после command timeout
status: done
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - commands
    - timeout
    - lifecycle
    - state
    - observability
    - llm-reliability
acceptance_criteria:
    - Every command reaches ok, error, aborted, or timed_out no later than a bounded grace period after timeout_seconds.
    - Timeout forcibly terminates the process tree and persists an explicit timed_out reason.
    - command get/list/filter agree on terminal status and retain bounded diagnostic tail/output metadata.
    - A regression test covers a child/process-group that ignores normal termination.
verification_plan:
    - Run focused command timeout and process-tree termination tests.
    - Verify get/list/filter converge on the same terminal record without server restart.
    - Run make quality and make lint.
created_at: "2026-08-01T14:38:14.547212Z"
updated_at: "2026-08-01T14:49:24.653978Z"
---

## Body

Command ea275f2c077b0ccf4e020a795ff53c8a remained running beyond its configured timeout_seconds=600 (observed at 617s) while command get/filter returned no stdout, stderr, or terminal error. Timeout enforcement and durable command state must converge even when a child process or pipeline does not exit normally.

## Acceptance Criteria

- Every command reaches ok, error, aborted, or timed_out no later than a bounded grace period after timeout_seconds.
- Timeout forcibly terminates the process tree and persists an explicit timed_out reason.
- command get/list/filter agree on terminal status and retain bounded diagnostic tail/output metadata.
- A regression test covers a child/process-group that ignores normal termination.

## Verification Plan

1. Run focused command timeout and process-tree termination tests.
2. Verify get/list/filter converge on the same terminal record without server restart.
3. Run make quality and make lint.
