---
id: task-registry-import-export
title: Добавить import/export между Lean и Obsidian task registry formats
status: done
priority: high
model_level: medium
task_type: implementation
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - lean
  - obsidian
  - import-export
  - implementation
  - tests
  - weak-model
  - tdd
branch: implementation/task-registry-import-export
worktree_path: .worktrees/task-registry-import-export
acceptance_criteria:
  - Round-trip, dry-run, conflict and loss-report tests are written before implementation behavior is accepted.
  - Lean -> Obsidian export creates deterministic Markdown registry output with all required fields preserved.
  - Obsidian -> Lean import validates the complete input before mutation and fails without partial writes on conflicts or lossy mappings.
  - Dry-run reports additions, updates, deletes/omissions if supported, conflicts and any loss risk without mutating files.
  - Default behavior is fail-closed when a field cannot be represented or a target registry is stale.
  - Import/export behavior is clear when the configured backend is lean vs obsidian; explicit source/target options override ambiguity.
  - Round-trip tests prove representative records survive Lean -> Obsidian -> Lean without semantic loss.
  - Code keeps conversion logic separate from backend selection and avoids broad unrelated refactors.
verification_plan:
  - Run targeted import/export tests only.
  - Include a lossy/unsupported fixture and verify the command fails with a compact loss report.
  - Run config interaction tests only if import/export uses configured defaults.
  - Run one compatibility test for existing Lean task tool behavior if import/export shares backend plumbing.
  - Run the existing lint/check command for touched Go packages if available; otherwise state that no lint gate was available.
created_at: 2026-05-14T09:15:25.492272Z
updated_at: 2026-05-14T09:15:25.492272Z
---

## Body

Weak-model execution contract: build import/export from failing round-trip tests first. Start with Lean -> Obsidian dry-run, Obsidian -> Lean dry-run, representative round-trip success, stale target, duplicate ID, and lossy unsupported-field fixtures. Implementation must be fail-closed: no partial writes after validation failure, no silent field drops, no hidden auto-detection, and no test weakening.

Implement explicit import/export between Lean registry format and Obsidian Markdown registry format using the backend contract. The surface may be CLI, MCP, or an existing helper command path selected by the design, but it must support dry-run validation, compact change summary, structured loss/conflict report, and fail-closed defaults.

Import/export source/target selection must be explicit and must not depend on silently auto-detecting the active backend. It may use the configured default backend as one side only when the command contract says so clearly. Import/export must not silently drop Lean-specific fields, task relationships, acceptance criteria, verification plans, status, model_level, branch/worktree data, tags, or provenance that the contract marks as required. Before finalization, run targeted import/export tests, relevant config interaction tests, and the existing lint/check gate for touched Go packages if configured.

## Acceptance Criteria

- Round-trip, dry-run, conflict and loss-report tests are written before implementation behavior is accepted.
- Lean -> Obsidian export creates deterministic Markdown registry output with all required fields preserved.
- Obsidian -> Lean import validates the complete input before mutation and fails without partial writes on conflicts or lossy mappings.
- Dry-run reports additions, updates, deletes/omissions if supported, conflicts and any loss risk without mutating files.
- Default behavior is fail-closed when a field cannot be represented or a target registry is stale.
- Import/export behavior is clear when the configured backend is lean vs obsidian; explicit source/target options override ambiguity.
- Round-trip tests prove representative records survive Lean -> Obsidian -> Lean without semantic loss.
- Code keeps conversion logic separate from backend selection and avoids broad unrelated refactors.

## Verification Plan

1. Run targeted import/export tests only.
2. Include a lossy/unsupported fixture and verify the command fails with a compact loss report.
3. Run config interaction tests only if import/export uses configured defaults.
4. Run one compatibility test for existing Lean task tool behavior if import/export shares backend plumbing.
5. Run the existing lint/check command for touched Go packages if available; otherwise state that no lint gate was available.
