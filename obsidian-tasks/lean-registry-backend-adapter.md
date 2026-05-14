---
id: lean-registry-backend-adapter
title: Ввести TaskRegistryBackend abstraction с Lean backend по умолчанию
status: done
priority: high
model_level: medium
task_type: implementation
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - lean
  - backend
  - implementation
  - tests
  - weak-model
  - tdd
branch: implementation/lean-registry-backend-adapter
worktree_path: .worktrees/lean-registry-backend-adapter
acceptance_criteria:
  - Focused tests are written or updated before the adapter change and fail for the missing abstraction or compatibility condition where practical.
  - Existing Lean-backed task tools keep the same observable behavior and structured errors for supported calls.
  - Backend abstraction has typed records and validation without using untyped catch-all data for core fields.
  - Lean adapter owns existing Lean parse/write behavior with minimal code movement.
  - No Obsidian format support, config backend selection, or import/export behavior is mixed into this task.
  - Code follows existing package style, has small functions with explicit errors, and avoids broad renames/format churn.
verification_plan:
  - Run the narrow task registry/backend tests affected by the adapter.
  - Run a focused task_current/task_batch_upsert compatibility scenario.
  - Run the existing lint/check command for touched Go packages if the repo defines one; otherwise state that no lint gate was available.
  - Do not run broad suites unless shared MCP registration or global task parsing code changed.
created_at: 2026-05-14T09:15:25.492003Z
updated_at: 2026-05-14T09:15:25.492003Z
---

## Body

Weak-model execution contract: implement test-first and keep the patch local. First add or update focused compatibility tests that capture current Lean-backed behavior for task_current and task_batch_upsert. Only then introduce the smallest TaskRegistryBackend abstraction needed by the approved contract. Do not refactor unrelated task code, do not change public behavior to make tests pass, and do not add untyped catch-all maps for core task fields.

Implement the minimal backend abstraction selected by obsidian-registry-backend-contract and move existing Lean registry access behind a Lean TaskRegistryBackend adapter. Preserve current Lean behavior as the default path for task_current/task_batch_upsert and any existing task-facing tools.

This task must not implement Obsidian parsing, config backend selection, or import/export. The goal is a compatibility-preserving adapter with focused tests proving the Lean path did not regress. Before finalization, run the narrow relevant tests and the existing lint/check gate for the touched Go packages if configured; if no lint gate exists, record that explicitly instead of pretending it ran.

## Acceptance Criteria

- Focused tests are written or updated before the adapter change and fail for the missing abstraction or compatibility condition where practical.
- Existing Lean-backed task tools keep the same observable behavior and structured errors for supported calls.
- Backend abstraction has typed records and validation without using untyped catch-all data for core fields.
- Lean adapter owns existing Lean parse/write behavior with minimal code movement.
- No Obsidian format support, config backend selection, or import/export behavior is mixed into this task.
- Code follows existing package style, has small functions with explicit errors, and avoids broad renames/format churn.

## Verification Plan

1. Run the narrow task registry/backend tests affected by the adapter.
2. Run a focused task_current/task_batch_upsert compatibility scenario.
3. Run the existing lint/check command for touched Go packages if the repo defines one; otherwise state that no lint gate was available.
4. Do not run broad suites unless shared MCP registration or global task parsing code changed.
