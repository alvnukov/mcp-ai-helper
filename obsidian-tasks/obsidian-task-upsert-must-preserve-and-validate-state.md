---
id: obsidian-task-upsert-must-preserve-and-validate-state
title: Исправить destructive partial upsert и validation Obsidian-задач
status: done
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - tasks
    - obsidian
    - state
    - validation
    - data-integrity
acceptance_criteria:
    - Partial update preserves omitted existing fields or the public contract explicitly requires a full object.
    - New tasks receive valid defaults when optional fields are omitted.
    - Invalid enums fail before any file write.
    - A successful upsert is immediately returned by task current/get.
verification_plan:
    - Run focused Obsidian upsert/read roundtrip tests.
    - Run task MCP handler contract tests.
created_at: "2026-08-01T11:19:08.031831Z"
updated_at: "2026-08-16T11:19:03.086914Z"
---

## Body

Define and implement safe task upsert semantics. Optional fields must not silently erase existing fields; new tasks need valid defaults; status/priority/model_level must be normalized and validated before writing so a successful mutation cannot disappear from the next read.

## Acceptance Criteria

- Partial update preserves omitted existing fields or the public contract explicitly requires a full object.
- New tasks receive valid defaults when optional fields are omitted.
- Invalid enums fail before any file write.
- A successful upsert is immediately returned by task current/get.

## Verification Plan

1. Run focused Obsidian upsert/read roundtrip tests.
2. Run task MCP handler contract tests.
