---
id: obsidian-task-batch-must-be-atomic
title: Сделать Obsidian batch_upsert атомарным и не скрывать ошибки
status: todo
priority: high
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - tasks
    - obsidian
    - transaction
    - batch
    - reliability
acceptance_criteria:
    - Validation failure causes zero task-file mutations.
    - close_missing read/write failures are returned, never silently skipped.
    - Result changed_files and validation describe the committed state exactly.
verification_plan:
    - Run focused fault-injection batch tests.
    - Run Obsidian backend integration tests.
created_at: "2026-08-01T11:19:08.032261Z"
updated_at: "2026-08-01T11:19:08.032261Z"
---

## Body

Obsidian batch_upsert can leave early writes after a later failure and silently continues after close_missing read/write failures. Stage and validate the complete mutation before commit, then apply atomically or fail with explicit partial-state diagnostics.

## Acceptance Criteria

- Validation failure causes zero task-file mutations.
- close_missing read/write failures are returned, never silently skipped.
- Result changed_files and validation describe the committed state exactly.

## Verification Plan

1. Run focused fault-injection batch tests.
2. Run Obsidian backend integration tests.
