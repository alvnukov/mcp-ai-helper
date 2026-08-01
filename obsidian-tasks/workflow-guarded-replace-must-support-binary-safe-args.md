---
id: workflow-guarded-replace-must-support-binary-safe-args
title: Workflow guarded_replace must accept binary-safe arguments
status: todo
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - surface-contract
    - escaping
    - llm-reliability
acceptance_criteria:
    - guarded_replace workflow steps accept old_b64/new_b64 with parity to edit action=replace
    - Schema and runtime contract describe the same accepted fields
    - Regression test edits source containing literal escape sequences through b64 arguments
    - Malformed base64 fails before any workflow mutation
verification_plan:
    - Run targeted workflow guarded_replace tests
    - Run make quality
    - Run golangci-lint
created_at: "2026-08-01T12:43:25.227907Z"
updated_at: "2026-08-01T12:43:25.227907Z"
---

## Body

The workflow guarded_replace step rejects documented old_b64/new_b64 arguments with 'old text is required'. This prevents reliable edits containing escape-sensitive source such as Go string literals with backslash-n and creates avoidable partial-workflow failures.

## Acceptance Criteria

- guarded_replace workflow steps accept old_b64/new_b64 with parity to edit action=replace
- Schema and runtime contract describe the same accepted fields
- Regression test edits source containing literal escape sequences through b64 arguments
- Malformed base64 fails before any workflow mutation

## Verification Plan

1. Run targeted workflow guarded_replace tests
2. Run make quality
3. Run golangci-lint
