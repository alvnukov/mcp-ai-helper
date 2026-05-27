---
id: task-123
title: Добавить Google Custom Search provider для web_search
status: done
priority: high
model_level: medium
task_type: implementation
tags:
    - web
    - search
    - google
    - mcp
    - config
    - tests
acceptance_criteria:
    - web_search supports explicit provider google_cse and maps Google Custom Search JSON items to compact hits.
    - Google provider configuration is represented in typed config and config schema without exposing literal API keys in JSON output.
    - Missing Google cx or API key returns compact fail-closed diagnostics.
    - Focused tests cover successful Google JSON parsing and missing configuration errors.
verification_plan:
    - Run focused websearch package tests.
    - Run focused MCP web_search handler tests.
    - Run config tests because config/schema fields change.
created_at: "2026-05-14T09:15:25.490944Z"
updated_at: "2026-05-27T10:18:56.444034Z"
---

## Body

Add a google_cse provider adapter for web_search using the official Google Custom Search JSON API. Keep results compact and compatible with the existing web_search hit contract. The provider must fail closed when cx/API key configuration is missing and must not scrape Google HTML search results.

## Acceptance Criteria

- web_search supports explicit provider google_cse and maps Google Custom Search JSON items to compact hits.
- Google provider configuration is represented in typed config and config schema without exposing literal API keys in JSON output.
- Missing Google cx or API key returns compact fail-closed diagnostics.
- Focused tests cover successful Google JSON parsing and missing configuration errors.

## Verification Plan

1. Run focused websearch package tests.
2. Run focused MCP web_search handler tests.
3. Run config tests because config/schema fields change.
