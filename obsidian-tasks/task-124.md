---
id: task-124
title: Добавить web-search usage и MCP-only запреты в assistant guidance
status: done
priority: high
model_level: low
task_type: docs
tags:
    - guidance
    - web
    - mcp
    - config
    - tests
acceptance_criteria:
    - 'Default assistant guidance explains the token-efficient web workflow: web_search returns compact hits, web_fetch creates doc_id artifacts, fetched_doc_find/read query bounded content fragments.'
    - Default assistant guidance explicitly forbids direct repo filesystem/shell/git tools outside mcp-ai-helper MCP surface when working in this repository.
    - Generated/example config guidance carries the same rules.
    - Focused tests assert the guidance contains the web workflow and MCP-only prohibition.
verification_plan:
    - Run focused config tests for guidance/default config behavior.
    - Run no broad tests unless shared config parsing changes unexpectedly.
created_at: "2026-05-14T09:15:25.491062Z"
updated_at: "2026-05-27T11:17:31.807201Z"
---

## Body

Update helper assistant guidance so models know how to use web_search/web_fetch/fetched_doc tools token-efficiently and see an explicit prohibition on non-MCP repo tools. Keep changes focused on guidance/default config and tests. Existing active user config may also need a config_replace update because assistant_guidance is config-owned at runtime.

## Acceptance Criteria

- Default assistant guidance explains the token-efficient web workflow: web_search returns compact hits, web_fetch creates doc_id artifacts, fetched_doc_find/read query bounded content fragments.
- Default assistant guidance explicitly forbids direct repo filesystem/shell/git tools outside mcp-ai-helper MCP surface when working in this repository.
- Generated/example config guidance carries the same rules.
- Focused tests assert the guidance contains the web workflow and MCP-only prohibition.

## Verification Plan

1. Run focused config tests for guidance/default config behavior.
2. Run no broad tests unless shared config parsing changes unexpectedly.
