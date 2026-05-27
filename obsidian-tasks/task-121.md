---
id: task-121
title: Добавить bounded read/find и pipeline usage для fetched documents
status: done
priority: high
model_level: medium
task_type: implementation
parent_id: task-068
tags:
    - feature
    - network
    - fetch
    - pipeline
    - mcp
    - implementation
    - tests
    - context-budget
branch: implementation/task-121
worktree_path: .worktrees/task-121
acceptance_criteria:
    - Bounded read returns requested ranges/sections only, with offsets or line ranges, content source marker and truncation metadata.
    - Bounded find searches the complete stored normalized text for complete artifacts and returns limited snippets with stable offsets/ranges; incomplete artifacts are reported explicitly.
    - Pipeline/workflow command steps can consume a fetched artifact by doc_id or explicit artifact reference for local preprocessing without injecting the full document into the model response.
    - Artifact access is constrained to the helper-managed cache/workspace and does not expose arbitrary filesystem paths.
    - Structured errors cover unknown doc_id, expired artifact, incomplete artifact, invalid range/query and pipeline surface mismatch.
    - Regression tests prove fetch_url does not return the full body, read/find return only bounded fragments, and a pipeline step can compute a hash or compact summary from the local artifact.
verification_plan:
    - Run targeted tests for fetched-document read/find handlers and pipeline artifact resolution.
    - Use a large local fixture to verify full content remains stored while model-facing outputs stay bounded.
    - Do not broaden to global network/search tests unless shared policy or pipeline command plumbing changes.
created_at: "2026-05-14T09:15:25.490691Z"
updated_at: "2026-05-27T09:43:35.128802Z"
---

## Body

After task-094, expose fetched document handles for bounded model reading and local pipeline preprocessing. Implement the read/find surface selected by task-093, and let run_pipeline/run_workflow command steps consume fetched artifacts by doc_id or an explicit artifact reference without automatically adding full content to model context.

The feature exists to reduce token use while preserving quality: fetch downloads and stores complete accepted content; read/find returns only requested fragments; pipeline preprocessing can operate locally on raw/normalized artifacts.

## Acceptance Criteria

- Bounded read returns requested ranges/sections only, with offsets or line ranges, content source marker and truncation metadata.
- Bounded find searches the complete stored normalized text for complete artifacts and returns limited snippets with stable offsets/ranges; incomplete artifacts are reported explicitly.
- Pipeline/workflow command steps can consume a fetched artifact by doc_id or explicit artifact reference for local preprocessing without injecting the full document into the model response.
- Artifact access is constrained to the helper-managed cache/workspace and does not expose arbitrary filesystem paths.
- Structured errors cover unknown doc_id, expired artifact, incomplete artifact, invalid range/query and pipeline surface mismatch.
- Regression tests prove fetch_url does not return the full body, read/find return only bounded fragments, and a pipeline step can compute a hash or compact summary from the local artifact.

## Verification Plan

1. Run targeted tests for fetched-document read/find handlers and pipeline artifact resolution.
2. Use a large local fixture to verify full content remains stored while model-facing outputs stay bounded.
3. Do not broaden to global network/search tests unless shared policy or pipeline command plumbing changes.
