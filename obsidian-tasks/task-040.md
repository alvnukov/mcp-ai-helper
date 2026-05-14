---
id: task-040
title: Add Lean-backed task mutation path with Lake validation
status: done
priority: critical
tags:
  - tasks
  - lean
  - lake
  - mutation
  - safety
worktree_path: .worktrees/task-040
created_at: 2026-05-14T09:15:25.478148Z
updated_at: 2026-05-14T09:15:25.478148Z
---

## Body

Add the minimal safe mutation path for Lean-native task state. The goal is to let task status/upsert operations update canonical Lean source and then prove the result with Lake. This must be constrained and fail closed; do not build a general Lean source editor.

Required scope:
1. Define the canonical Lean source layout for mutable task artifacts so exact, localized edits are possible without parsing arbitrary Lean syntax.
2. Implement task_set_status against Lean-backed tasks for the migrated repo: update only the status field/declaration for a known task id, with hash/snapshot guarding or equivalent conflict detection.
3. Implement task_upsert or task_batch_upsert only for the minimal fields needed by current workflows, using deterministic Lean source templates and explicit owned files.
4. After every mutation, run targeted Lean check and/or `lake build`; if validation fails, return a compact blocker and do not leave a half-applied state where practical.
5. Keep legacy mutation fallback only for repos without Lean workspace/exporter, and make fallback explicit.
6. Add tests for successful status change, duplicate id rejection, invalid generated source rejection, dirty/conflicting file guard, and legacy fallback.

Out of scope:
- No general-purpose Lean AST editor.
- No LSP server yet.
- No arbitrary graph rewrites beyond task status/upsert/batch-upsert needed for existing task tools.
- No broad git commit workflow changes.
