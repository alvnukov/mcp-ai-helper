---
id: obsidian-task-registry-backend
title: Добавить configurable Lean/Obsidian task registry backend без потери функционала
status: blocked
priority: high
model_level: very_high
task_type: epic
tags:
  - tasks
  - registry
  - lean
  - obsidian
  - backend
  - config
  - import-export
  - epic
  - decomposed
branch: epic/obsidian-task-registry-backend
worktree_path: .worktrees/obsidian-task-registry-backend
acceptance_criteria:
  - Parent remains non-executable; all design/implementation work is represented by low/medium child tasks.
  - User config exposes an explicit task registry backend selection for at least lean and obsidian; Lean remains the default when the option is absent.
  - All task-facing MCP tools use the selected backend consistently for a repository, with structured diagnostics for invalid backend config, missing Obsidian path, stale registry, or unsupported format.
  - Lean registry behavior remains backward compatible when backend=lean or when no backend is configured.
  - Obsidian format is human-editable Markdown but has a typed mapping for all supported task registry fields.
  - Import/export between Lean and Obsidian either preserves all supported fields and semantics or fails with a compact loss report; it never drops data silently.
  - Round-trip tests cover Lean -> Obsidian -> Lean and Obsidian -> Lean -> Obsidian for representative task records.
verification_plan:
  - Review child tasks for explicit Lean parity, config selection contract, Obsidian format contract, import/export failure semantics and focused gates.
  - Check the config-selection task proves task_current/task_batch_upsert route to the selected backend.
  - Do not close parent until backend abstraction, Obsidian backend, config selection and import/export tasks are merged and verified.
created_at: 2026-05-14T09:15:25.491579Z
updated_at: 2026-05-14T09:15:25.491579Z
---

## Body

Parent/epic. Add a configurable task registry backend so the helper can operate against either the existing Lean registry or an Obsidian-style Markdown registry selected by user config. Execute only through low/medium child tasks: inventory current Lean capabilities, define backend/config/Obsidian format contracts, keep Lean as the default backend, implement Lean adapter, implement Obsidian read/write, wire config-driven backend selection, and add explicit import/export between registry formats.

Do not implement this parent directly. The core invariant is fail-closed parity: no Lean-supported task field or workflow semantic may be silently dropped when reading, writing, importing, exporting, selecting a backend, or round-tripping through Obsidian format.

## Acceptance Criteria

- Parent remains non-executable; all design/implementation work is represented by low/medium child tasks.
- User config exposes an explicit task registry backend selection for at least lean and obsidian; Lean remains the default when the option is absent.
- All task-facing MCP tools use the selected backend consistently for a repository, with structured diagnostics for invalid backend config, missing Obsidian path, stale registry, or unsupported format.
- Lean registry behavior remains backward compatible when backend=lean or when no backend is configured.
- Obsidian format is human-editable Markdown but has a typed mapping for all supported task registry fields.
- Import/export between Lean and Obsidian either preserves all supported fields and semantics or fails with a compact loss report; it never drops data silently.
- Round-trip tests cover Lean -> Obsidian -> Lean and Obsidian -> Lean -> Obsidian for representative task records.

## Verification Plan

1. Review child tasks for explicit Lean parity, config selection contract, Obsidian format contract, import/export failure semantics and focused gates.
2. Check the config-selection task proves task_current/task_batch_upsert route to the selected backend.
3. Do not close parent until backend abstraction, Obsidian backend, config selection and import/export tasks are merged and verified.
