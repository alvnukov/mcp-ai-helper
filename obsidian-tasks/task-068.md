---
id: task-068
title: Добавить bounded fetch/search tools для LLM через mcp-ai-helper
status: blocked
priority: high
model_level: high
task_type: epic
tags:
  - feature
  - network
  - fetch
  - search
  - llm-tools
  - security
  - evidence
  - mcp
  - epic
  - decomposed
branch: epic/task-068
worktree_path: .worktrees/task-068
acceptance_criteria:
  - Parent remains non-executable; medium child tasks own design and implementation.
  - Accepted fetches preserve source content losslessly or report blocked/incomplete explicitly; optimized text is never treated as the only copy.
  - Default fetch output returns a handle and metadata, not the full fetched page body.
  - Fetched documents can be consumed by bounded read/find tools and by pipeline/workflow preprocessing without automatically loading full content into model context.
  - Security, cache, rate-limit and prompt-injection boundaries are defined before implementation.
verification_plan:
  - Review task-093 for the full lossless fetch/search design decision.
  - Review task-094 for handle-based fetch_url core implementation and tests.
  - Review task-095 for compact search result contract.
  - Review task-121 for fetched-document read/find and pipeline integration behavior.
created_at: 2026-05-14T09:15:25.482493Z
updated_at: 2026-05-14T09:15:25.482493Z
---

## Body

Parent/epic. Bounded fetch/search is decomposed into task-093, task-094, task-095 and task-121. Parent remains blocked until design, lossless fetch core, compact search and fetched-document read/pipeline usage are complete.

Critical invariant for all child tasks: optimizing fetched pages for LLM reading must not discard source content. The accepted fetch source is preserved as the source of truth; reader/normalized text is derivative only. Default MCP responses must stay bounded and must not inject a full page into model context unless a bounded read/find tool explicitly returns selected fragments.

## Acceptance Criteria

- Parent remains non-executable; medium child tasks own design and implementation.
- Accepted fetches preserve source content losslessly or report blocked/incomplete explicitly; optimized text is never treated as the only copy.
- Default fetch output returns a handle and metadata, not the full fetched page body.
- Fetched documents can be consumed by bounded read/find tools and by pipeline/workflow preprocessing without automatically loading full content into model context.
- Security, cache, rate-limit and prompt-injection boundaries are defined before implementation.

## Verification Plan

1. Review task-093 for the full lossless fetch/search design decision.
2. Review task-094 for handle-based fetch_url core implementation and tests.
3. Review task-095 for compact search result contract.
4. Review task-121 for fetched-document read/find and pipeline integration behavior.
