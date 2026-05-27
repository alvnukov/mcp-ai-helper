---
id: task-125
title: Уточнить guidance и web tool descriptions для слабых моделей
status: done
priority: high
model_level: medium
task_type: docs
tags:
    - guidance
    - mcp
    - web
    - tools
    - weak-models
acceptance_criteria:
    - assistant_guidance contains a compact weak-model-friendly protocol for repo work and web research.
    - assistant_guidance explicitly states MCP-only constraints and fail-closed behavior when required MCP tools are missing.
    - web_search/web_fetch/fetched_doc_find/fetched_doc_read descriptions explain when to use each tool and the next recommended call.
    - Focused tests assert the guidance and web tool descriptions contain the key operational phrases.
verification_plan:
    - Run focused config guidance tests.
    - Run focused MCP web tool metadata tests.
created_at: "2026-05-14T09:15:25.491195Z"
updated_at: "2026-05-27T11:31:34.265066Z"
---

## Body

Make assistant guidance and web tool descriptions more operational for weaker models. The guidance should give explicit short protocols, MCP-only constraints, web search/fetch/read sequencing, and failure/blocker rules. Web tool descriptions should include compact next-step instructions and reinforce that search hits are not evidence until fetched/read.

## Acceptance Criteria

- assistant_guidance contains a compact weak-model-friendly protocol for repo work and web research.
- assistant_guidance explicitly states MCP-only constraints and fail-closed behavior when required MCP tools are missing.
- web_search/web_fetch/fetched_doc_find/fetched_doc_read descriptions explain when to use each tool and the next recommended call.
- Focused tests assert the guidance and web tool descriptions contain the key operational phrases.

## Verification Plan

1. Run focused config guidance tests.
2. Run focused MCP web tool metadata tests.
