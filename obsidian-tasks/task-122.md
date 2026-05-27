---
id: task-122
title: Реализовать компактный web_search provider
status: done
priority: high
model_level: medium
task_type: implementation
tags:
    - web
    - search
    - mcp
    - config
    - tests
acceptance_criteria:
    - web_search can return compact search hits from a configured provider without reading result pages into model context.
    - Search provider configuration is represented in typed config and config schema with bounded defaults.
    - Unsupported or disabled providers return compact diagnostics instead of silently scraping arbitrary services.
    - Focused tests cover parsing compact hits and fail-closed provider errors.
verification_plan:
    - Run focused websearch package tests.
    - Run focused MCP web_search handler tests.
    - Run config tests only if config/schema behavior changes.
created_at: "2026-05-14T09:15:25.490822Z"
updated_at: "2026-05-27T10:09:09.7013Z"
---

## Body

Implement a real bounded web_search provider adapter instead of the previous fail-closed placeholder. Results must stay compact: title, URL, snippet and metadata only; page bodies are still retrieved separately through web_fetch/fetched_doc tools. Keep provider behavior explicit in config/schema, fail closed on unsupported providers, and add focused tests with a fake search endpoint.

## Acceptance Criteria

- web_search can return compact search hits from a configured provider without reading result pages into model context.
- Search provider configuration is represented in typed config and config schema with bounded defaults.
- Unsupported or disabled providers return compact diagnostics instead of silently scraping arbitrary services.
- Focused tests cover parsing compact hits and fail-closed provider errors.

## Verification Plan

1. Run focused websearch package tests.
2. Run focused MCP web_search handler tests.
3. Run config tests only if config/schema behavior changes.
