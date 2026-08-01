---
id: integration-tools-must-propagate-handler-context
title: Jira and Confluence tools must propagate handler cancellation
status: done
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - jira
    - confluence
    - context
    - cancellation
    - timeouts
    - llm-reliability
acceptance_criteria:
    - Every Jira and Confluence network request is derived from the MCP handler context
    - Cancellation returns promptly and stops in-flight HTTP work
    - Client APIs preserve useful error context while exposing context-aware methods
    - Tests cover canceled and deadline-exceeded requests without goroutine leaks
verification_plan:
    - Run targeted Jira and Confluence client/tool tests
    - Run make quality
    - Run golangci-lint
created_at: "2026-08-01T13:14:35.725458Z"
updated_at: "2026-08-01T14:41:39.546782Z"
---

## Body

All Jira and Confluence MCP handlers receive context.Context but ignore it. Their client requests therefore continue after MCP cancellation/timeout, which can produce late work, leaked resources, and helper timeouts. The unused-parameter lint findings are a symptom; renaming ctx to underscore would hide the missing lifecycle contract.

## Acceptance Criteria

- Every Jira and Confluence network request is derived from the MCP handler context
- Cancellation returns promptly and stops in-flight HTTP work
- Client APIs preserve useful error context while exposing context-aware methods
- Tests cover canceled and deadline-exceeded requests without goroutine leaks

## Verification Plan

1. Run targeted Jira and Confluence client/tool tests
2. Run make quality
3. Run golangci-lint
