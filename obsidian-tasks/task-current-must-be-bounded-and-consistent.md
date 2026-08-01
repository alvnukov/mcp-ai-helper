---
id: task-current-must-be-bounded-and-consistent
title: Сделать task current компактным, свежим и семантически однозначным
status: todo
priority: high
model_level: medium
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - tasks
    - discovery
    - pagination
    - tokens
    - llm
acceptance_criteria:
    - Large registries cannot produce unbounded task current responses.
    - active_total and registry_total are distinct and correct.
    - Response exposes freshness/revision information and explicit omission metadata.
    - next_call references only callable surface or includes a valid fallback.
verification_plan:
    - Run focused large-registry response tests.
    - Run self-describing discovery-flow tests.
created_at: "2026-08-01T11:19:08.032731Z"
updated_at: "2026-08-01T11:19:08.032731Z"
---

## Body

Add bounded discovery semantics: limit/cursor or ranked executable subset, separate active and registry totals, revision/freshness metadata, and actionable next_call that only names visible tools. Avoid repeated absolute paths and blocked epic noise.

## Acceptance Criteria

- Large registries cannot produce unbounded task current responses.
- active_total and registry_total are distinct and correct.
- Response exposes freshness/revision information and explicit omission metadata.
- next_call references only callable surface or includes a valid fallback.

## Verification Plan

1. Run focused large-registry response tests.
2. Run self-describing discovery-flow tests.
