---
id: obsidian-registry-backend-contract
title: Определить TaskRegistryBackend contract, config selection и Obsidian schema
status: done
priority: high
model_level: medium
task_type: design
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - backend
  - config
  - obsidian
  - schema
  - design
  - weak-model
branch: design/obsidian-registry-backend-contract
worktree_path: .worktrees/obsidian-registry-backend-contract
acceptance_criteria:
  - Backend contract is explicit enough for a medium model to implement Lean and Obsidian adapters plus config selection without inventing semantics.
  - Config contract exposes explicit user selection between lean and obsidian and keeps lean as the backward-compatible default.
  - Obsidian schema preserves every required Lean capability from the parity matrix or defines a fail-closed unsupported case.
  - Contract includes test-first examples: valid fixtures, invalid fixtures, loss-report cases, config-routing cases and round-trip expectations.
  - Import/export loss report semantics are specified before implementation.
  - Design states how existing Lean default behavior remains backward compatible across task_current/task_batch_upsert and related task tools.
  - Examples include backend=lean, backend=obsidian with path settings, one parent epic note and one executable child task note.
verification_plan:
  - Review contract against the parity matrix from obsidian-registry-capability-inventory and current config_schema/server_setup_guidance surface.
  - Check examples for deterministic parse/write behavior, config routing clarity and no silent data loss.
  - Check each implementation task can derive its tests directly from this contract.
  - Confirm no product code is changed.
created_at: 2026-05-14T09:15:25.491878Z
updated_at: 2026-05-14T09:15:25.491878Z
---

## Body

Weak-model execution contract: this is a medium design task. Do not write product code. Design from tests first: define the fixtures, validation failures and focused gates that prove the future implementation before specifying code structure. Keep the contract narrow enough that a later medium model can implement it without broad refactoring.

Using obsidian-registry-capability-inventory, define an implementation-ready backend contract for task registries, the user config selection semantics, and a concrete Obsidian Markdown storage format. The contract must cover canonical task record fields, validation, deterministic ordering, atomic write expectations, conflict/stale edit detection, provenance, structured errors, and loss reporting.

Define the config behavior: exact config field names after checking the current config schema, supported backend enum values, default Lean behavior, required Obsidian registry path/vault settings, path resolution rules, reload behavior if applicable, invalid-config diagnostics, and whether per-repo config overrides global config. Also define how all task-facing MCP tools select the backend from config without ad-hoc auto-detection.

Define the Obsidian representation: one-note-per-task or selected layout, required frontmatter fields, Markdown sections for body/acceptance/verification, escaping rules, stable IDs, links/parent references, and how Lean-specific fields are preserved. Include explicit fixture names and expected parse/write/import/export outcomes. No implementation in this task.

## Acceptance Criteria

- Backend contract is explicit enough for a medium model to implement Lean and Obsidian adapters plus config selection without inventing semantics.
- Config contract exposes explicit user selection between lean and obsidian and keeps lean as the backward-compatible default.
- Obsidian schema preserves every required Lean capability from the parity matrix or defines a fail-closed unsupported case.
- Contract includes test-first examples: valid fixtures, invalid fixtures, loss-report cases, config-routing cases and round-trip expectations.
- Import/export loss report semantics are specified before implementation.
- Design states how existing Lean default behavior remains backward compatible across task_current/task_batch_upsert and related task tools.
- Examples include backend=lean, backend=obsidian with path settings, one parent epic note and one executable child task note.

## Verification Plan

1. Review contract against the parity matrix from obsidian-registry-capability-inventory and current config_schema/server_setup_guidance surface.
2. Check examples for deterministic parse/write behavior, config routing clarity and no silent data loss.
3. Check each implementation task can derive its tests directly from this contract.
4. Confirm no product code is changed.
