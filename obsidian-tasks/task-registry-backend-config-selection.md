---
id: task-registry-backend-config-selection
title: Добавить config-driven выбор Lean или Obsidian task registry backend
status: done
priority: high
model_level: medium
task_type: implementation
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - backend
  - config
  - lean
  - obsidian
  - implementation
  - tests
  - weak-model
  - tdd
branch: implementation/task-registry-backend-config-selection
worktree_path: .worktrees/task-registry-backend-config-selection
acceptance_criteria:
  - Config tests are written first for unset, lean, obsidian, invalid backend and missing Obsidian settings.
  - Config supports explicit backend selection for lean and obsidian, with lean as the default when unset.
  - Invalid backend values, missing Obsidian registry path/settings, unreadable registry roots, and stale/unsupported backend state fail with structured diagnostics before mutation.
  - task_current and task_batch_upsert use the configured backend consistently for the same repository.
  - Existing Lean behavior and config remain backward compatible when backend is unset or lean.
  - Obsidian backend can be selected by config and used by task-facing tools without requiring an import/export command first.
  - config_schema and setup guidance describe the backend selection field and required Obsidian settings where those surfaces are present.
  - Code avoids hidden fallback behavior and keeps config precedence explicit.
verification_plan:
  - Run targeted config parser/validation tests for unset, lean, obsidian and invalid backend values.
  - Run focused task_current/task_batch_upsert routing tests for lean default and obsidian-selected repositories.
  - Run targeted schema/guidance tests if config_schema or server_setup_guidance output changes.
  - Run the existing lint/check command for touched Go packages if available; otherwise state that no lint gate was available.
  - Do not run broad suites unless shared config loading or MCP registration code changes.
created_at: 2026-05-14T09:15:25.492563Z
updated_at: 2026-05-14T09:15:25.492563Z
---

## Body

Weak-model execution contract: write config-selection tests first. Start with unset config, backend=lean, backend=obsidian with valid path/settings, invalid backend value, missing Obsidian path/settings, and stale/unreadable backend diagnostics. Only after tests describe the expected routing should implementation wire config parsing and backend factory selection. Do not auto-detect formats, do not silently fall back from obsidian to lean after a user explicitly selected obsidian, and do not weaken validation to make tests pass.

Implement user-configurable task registry backend selection from the contract in obsidian-registry-backend-contract. The helper must route task-facing tools through the selected backend for the repository: lean by default, obsidian when explicitly configured with valid Obsidian registry settings.

This task owns config parsing/validation, backend factory/selection wiring, config_schema/server_setup_guidance updates if those surfaces exist, and focused tests for selected-backend behavior. It should not implement Obsidian parsing itself beyond using the backend adapter already provided by obsidian-markdown-registry-backend, and it should not implement import/export. Before finalization, run focused config/routing tests plus schema/guidance tests if those outputs changed, and run the existing lint/check gate for touched Go packages if configured.

## Acceptance Criteria

- Config tests are written first for unset, lean, obsidian, invalid backend and missing Obsidian settings.
- Config supports explicit backend selection for lean and obsidian, with lean as the default when unset.
- Invalid backend values, missing Obsidian registry path/settings, unreadable registry roots, and stale/unsupported backend state fail with structured diagnostics before mutation.
- task_current and task_batch_upsert use the configured backend consistently for the same repository.
- Existing Lean behavior and config remain backward compatible when backend is unset or lean.
- Obsidian backend can be selected by config and used by task-facing tools without requiring an import/export command first.
- config_schema and setup guidance describe the backend selection field and required Obsidian settings where those surfaces are present.
- Code avoids hidden fallback behavior and keeps config precedence explicit.

## Verification Plan

1. Run targeted config parser/validation tests for unset, lean, obsidian and invalid backend values.
2. Run focused task_current/task_batch_upsert routing tests for lean default and obsidian-selected repositories.
3. Run targeted schema/guidance tests if config_schema or server_setup_guidance output changes.
4. Run the existing lint/check command for touched Go packages if available; otherwise state that no lint gate was available.
5. Do not run broad suites unless shared config loading or MCP registration code changes.
