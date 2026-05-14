---
id: task-094
title: Реализовать lossless handle-based fetch_url MVP после design approval
status: todo
priority: high
model_level: medium
task_type: implementation
parent_id: task-068
tags:
  - feature
  - network
  - fetch
  - security
  - implementation
  - tests
  - cache
branch: implementation/task-094
worktree_path: .worktrees/task-094
acceptance_criteria:
  - Accepted HTML/text fixtures round-trip raw/source bytes exactly; normalized text is stored as a derivative artifact, not as the only copy.
  - fetch_url default MCP response is bounded and contains handle/metadata/diagnostics only, with no full page body in normal success output.
  - Protocol, domain, redirect, size, timeout and content-type policy failures are fail-closed with structured diagnostics.
  - Over-limit or interrupted downloads are marked blocked/incomplete explicitly and are never reported as complete source artifacts.
  - Repeat fetches use the defined cache/hash behavior without duplicating model-context payload.
  - Focused tests cover success, redirect, denied URL/policy, size limit, content type and cache hit behavior.
verification_plan:
  - Run targeted fetch package and MCP handler tests for fetch_url only.
  - Inspect one success fixture response to confirm it contains doc_id/metadata and not page body.
  - Do not run broad tests unless shared MCP server registration or policy code changes.
created_at: 2026-05-14T09:15:25.48727Z
updated_at: 2026-05-14T09:15:25.48727Z
---

## Body

After task-093 approves the MVP, implement the core fetch_url transport/artifact path. The tool must fetch allowed URLs under policy controls, persist accepted source content losslessly, derive normalized/reader text separately and return only a bounded handle-based response.

Default response must include doc_id, URL metadata, content hash, byte length, completeness status, cache status and compact diagnostics. It must not return the full fetched page body. Do not implement broad search here; compact search remains task-095, and fetched-document read/find plus pipeline integration are task-121.

## Acceptance Criteria

- Accepted HTML/text fixtures round-trip raw/source bytes exactly; normalized text is stored as a derivative artifact, not as the only copy.
- fetch_url default MCP response is bounded and contains handle/metadata/diagnostics only, with no full page body in normal success output.
- Protocol, domain, redirect, size, timeout and content-type policy failures are fail-closed with structured diagnostics.
- Over-limit or interrupted downloads are marked blocked/incomplete explicitly and are never reported as complete source artifacts.
- Repeat fetches use the defined cache/hash behavior without duplicating model-context payload.
- Focused tests cover success, redirect, denied URL/policy, size limit, content type and cache hit behavior.

## Verification Plan

1. Run targeted fetch package and MCP handler tests for fetch_url only.
2. Inspect one success fixture response to confirm it contains doc_id/metadata and not page body.
3. Do not run broad tests unless shared MCP server registration or policy code changes.
