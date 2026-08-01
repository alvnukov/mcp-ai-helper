---
id: command-output-must-preserve-diagnostic-tail
title: Сохранить диагностический tail и явные omission metadata command output
status: todo
priority: high
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - commands
    - logs
    - truncation
    - evidence
    - observability
acceptance_criteria:
    - Truncated output retains the true final diagnostic tail.
    - Response reports exact or conservative omitted byte/line metadata.
    - Output hash semantics are documented and tested.
    - command get/filter clearly distinguish retained artifact completeness.
verification_plan:
    - Run focused limit-buffer/history/filter tests.
    - Run pipeline evidence regression tests.
created_at: "2026-08-01T11:19:08.03344Z"
updated_at: "2026-08-01T11:19:08.03344Z"
---

## Body

Current output buffering retains only the prefix, so final failures are irrecoverable. Introduce a bounded artifact contract that preserves useful head and true tail, hashes the intended stream, and reports omitted bytes/lines. Filtering must not claim it can recover discarded content.

## Acceptance Criteria

- Truncated output retains the true final diagnostic tail.
- Response reports exact or conservative omitted byte/line metadata.
- Output hash semantics are documented and tested.
- command get/filter clearly distinguish retained artifact completeness.

## Verification Plan

1. Run focused limit-buffer/history/filter tests.
2. Run pipeline evidence regression tests.
