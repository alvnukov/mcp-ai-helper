---
id: command-abort-and-get-must-agree-on-terminal-state
title: Command abort and get must agree on terminal state
status: todo
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - commands
    - lifecycle
    - timeout
    - state
    - race
    - reliability
acceptance_criteria:
    - abort and get observe one authoritative terminal state for the same command_id.
    - Requested execution timeout produces a durable terminal timeout result within a bounded publication delay.
    - Regression test covers the already_completed versus running race without sleeps or weakened timeouts.
verification_plan:
    - Run focused command coordinator lifecycle tests under the race detector.
    - Run make quality and make lint.
created_at: "2026-08-01T17:48:23.008208Z"
updated_at: "2026-08-01T17:48:23.008208Z"
---

## Body

Observed durable command 147aa629c9fac7f37f00129e6e7e8a80 exceed requested timeout: command abort returned already_completed while immediate command get still returned running with no output. Unify runtime coordinator state publication so run/get/abort cannot expose contradictory terminal state.

## Acceptance Criteria

- abort and get observe one authoritative terminal state for the same command_id.
- Requested execution timeout produces a durable terminal timeout result within a bounded publication delay.
- Regression test covers the already_completed versus running race without sleeps or weakened timeouts.

## Verification Plan

1. Run focused command coordinator lifecycle tests under the race detector.
2. Run make quality and make lint.
