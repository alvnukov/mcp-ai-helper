---
id: helper-runtime-surface-must-match-guidance
title: Синхронизировать manifest, layers, schemas и assistant guidance
status: todo
priority: high
model_level: medium
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - mcp
    - schema
    - guidance
    - manifest
    - discoverability
acceptance_criteria:
    - Contract tests compare public schema/guidance claims with registered handlers.
    - Every documented command get mode has distinct implemented behavior or is removed.
    - Layer toggles have tested, predictable visibility semantics.
    - run schema and file list required fields match runtime validation.
verification_plan:
    - Run focused MCP registration/dispatch/guidance tests.
    - Compare tool_manifest for representative layer configurations.
created_at: "2026-08-01T11:19:08.033982Z"
updated_at: "2026-08-01T11:19:08.033982Z"
---

## Body

Eliminate self-description drift: guidance must not name hidden tools without fallback; command get modes must work or be removed; status enums and optional/required fields must match handlers; configured layers must control registration consistently; dead planning tools must be registered or removed from docs.

## Acceptance Criteria

- Contract tests compare public schema/guidance claims with registered handlers.
- Every documented command get mode has distinct implemented behavior or is removed.
- Layer toggles have tested, predictable visibility semantics.
- run schema and file list required fields match runtime validation.

## Verification Plan

1. Run focused MCP registration/dispatch/guidance tests.
2. Compare tool_manifest for representative layer configurations.
