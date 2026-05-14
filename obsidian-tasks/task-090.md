---
id: task-090
title: Спроектировать MCP schemas и result contract для external worker
status: todo
priority: high
model_level: medium
tags:
  - research
  - design
  - agents
  - mcp
  - schemas
  - result-contract
worktree_path: .worktrees/task-090
acceptance_criteria:
  - Proposed schemas include inputs, outputs, errors, status lifecycle and budget limits.
  - Result contract includes worktree/branch, changed files, diff summary, checks, rationale, unresolved risks and final status.
  - Schema design explicitly limits worker tools to restricted MCP surface and no direct shell/filesystem/git/network access.
  - Design chooses sync MVP or async MVP with concrete reasoning.
verification_plan:
  - Review schemas for fail-closed behavior and bounded payloads.
  - Check compatibility with current helper surfaces and documented surface gaps.
  - Confirm implementation backlog is left to task-103.
created_at: 2026-05-14T09:15:25.486774Z
updated_at: 2026-05-14T09:15:25.486774Z
---

## Body

Define proposed MCP tool schemas and output contract for bounded external worker. Cover synchronous vs async tradeoff, worker start/status/result/abort or external_worker_run, allowed read/search/snapshot/edit/check operations, result fields, evidence payloads, diagnostics, and compact risk reporting. No implementation in this task.

## Acceptance Criteria

- Proposed schemas include inputs, outputs, errors, status lifecycle and budget limits.
- Result contract includes worktree/branch, changed files, diff summary, checks, rationale, unresolved risks and final status.
- Schema design explicitly limits worker tools to restricted MCP surface and no direct shell/filesystem/git/network access.
- Design chooses sync MVP or async MVP with concrete reasoning.

## Verification Plan

1. Review schemas for fail-closed behavior and bounded payloads.
2. Check compatibility with current helper surfaces and documented surface gaps.
3. Confirm implementation backlog is left to task-103.
