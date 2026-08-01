---
id: task-discovery-must-be-read-only
title: Убрать mutation и auto-heal из task discovery read path
status: todo
priority: high
model_level: medium
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - tasks
    - obsidian
    - read-only
    - auto-heal
    - safety
acceptance_criteria:
    - Read actions perform zero filesystem mutations.
    - Filename/id mismatches return structured diagnostics and explicit repair next_action.
    - Explicit repair remains idempotent and conflict-safe.
verification_plan:
    - Run focused read-purity tests using mismatched and missing registry fixtures.
    - Run registry init/repair integration tests.
created_at: "2026-08-01T11:19:08.034356Z"
updated_at: "2026-08-01T11:19:08.034356Z"
---

## Body

task current/list/get must not create directories, rename notes, or dirty the repository. Move repairs behind an explicit task-facing repair/init operation and return structured diagnostics from reads.

## Acceptance Criteria

- Read actions perform zero filesystem mutations.
- Filename/id mismatches return structured diagnostics and explicit repair next_action.
- Explicit repair remains idempotent and conflict-safe.

## Verification Plan

1. Run focused read-purity tests using mismatched and missing registry fixtures.
2. Run registry init/repair integration tests.
