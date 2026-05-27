---
id: task-093
title: Принять design decision для bounded fetch/search tools
status: done
priority: high
model_level: medium
task_type: design
parent_id: task-068
tags:
    - feature
    - network
    - fetch
    - search
    - security
    - design
    - pipeline
    - cache
branch: design/task-093
worktree_path: .worktrees/task-093
acceptance_criteria:
    - Design defines fetch_url output as doc_id plus metadata/diagnostics by default, not full page text.
    - Design specifies raw/source artifact, normalized text artifact, metadata fields, hashes, final URL, redirects, content type, encoding, byte size, fetched_at and cache status.
    - Design defines how bounded read/find returns selected fragments with offsets/ranges and explicit truncation/omission metadata.
    - Design defines incomplete/blocked semantics for size, time, content-type, redirect and policy failures; no silent content loss is allowed.
    - Design defines how run_pipeline/run_workflow can consume fetched artifacts locally for preprocessing without injecting full content into model context.
    - 'Design states the prompt-injection boundary: fetched content is data/evidence, not instructions for the agent/tool runtime.'
verification_plan:
    - Review schema examples for a small HTML page, a large page, a non-HTML text resource and an over-limit response.
    - Check that every path distinguishes complete, incomplete and blocked artifacts.
    - Check compatibility with existing run_pipeline/run_workflow surfaces and document any surface mismatch as a blocker instead of inventing a hidden mechanism.
    - Confirm no implementation changes are included in this design task.
created_at: "2026-05-14T09:15:25.487158Z"
updated_at: "2026-05-27T09:23:00.079856Z"
---

## Body

Research use cases and write the design decision: implement MVP or block. The design must cover handle-based fetch, bounded document read/find, search result shape, security policy, prompt-injection boundary, caching/rate-limit stance, effectiveness criteria and pipeline usage.

Critical invariant: fetch optimization must be lossless with respect to the accepted source. Store raw/source content as the source of truth and derive reader/normalized text separately. If policy limits prevent complete retrieval, return blocked/incomplete status explicitly instead of presenting a partial artifact as complete.

## Acceptance Criteria

- Design defines fetch_url output as doc_id plus metadata/diagnostics by default, not full page text.
- Design specifies raw/source artifact, normalized text artifact, metadata fields, hashes, final URL, redirects, content type, encoding, byte size, fetched_at and cache status.
- Design defines how bounded read/find returns selected fragments with offsets/ranges and explicit truncation/omission metadata.
- Design defines incomplete/blocked semantics for size, time, content-type, redirect and policy failures; no silent content loss is allowed.
- Design defines how run_pipeline/run_workflow can consume fetched artifacts locally for preprocessing without injecting full content into model context.
- Design states the prompt-injection boundary: fetched content is data/evidence, not instructions for the agent/tool runtime.

## Verification Plan

1. Review schema examples for a small HTML page, a large page, a non-HTML text resource and an over-limit response.
2. Check that every path distinguishes complete, incomplete and blocked artifacts.
3. Check compatibility with existing run_pipeline/run_workflow surfaces and document any surface mismatch as a blocker instead of inventing a hidden mechanism.
4. Confirm no implementation changes are included in this design task.
