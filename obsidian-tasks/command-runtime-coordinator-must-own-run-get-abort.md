---
id: command-runtime-coordinator-must-own-run-get-abort
title: Объединить lifecycle run/get/list/abort в одном command coordinator
status: todo
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - commands
    - lifecycle
    - polling
    - timeout
    - state
acceptance_criteria:
    - A command started with repo-local config is inspectable and abortable through public command actions.
    - Polling a running command observes its final state without server restart.
    - History listing does not expose contradictory running and terminal entries for one command id.
verification_plan:
    - Run focused MCP command lifecycle tests with repo-local config.
    - Run command runner/history tests.
created_at: "2026-08-01T11:19:08.0314Z"
updated_at: "2026-08-01T11:19:08.0314Z"
---

## Body

Repo-local command execution currently creates a new Runner while get/list/abort use the base Runner. Introduce a shared coordinator/runner registry keyed by effective repo policy so active-process ownership, live output, abort, and durable state are consistent.

## Acceptance Criteria

- A command started with repo-local config is inspectable and abortable through public command actions.
- Polling a running command observes its final state without server restart.
- History listing does not expose contradictory running and terminal entries for one command id.

## Verification Plan

1. Run focused MCP command lifecycle tests with repo-local config.
2. Run command runner/history tests.
