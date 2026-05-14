---
id: task-038
title: Migrate active backlog to Lean-native task registry
status: done
priority: critical
tags:
  - lean
  - lake
  - tasks
  - migration
  - git-native
worktree_path: .worktrees/task-038
created_at: 2026-05-14T09:15:25.477893Z
updated_at: 2026-05-14T09:15:25.477893Z
---

## Body

Migrate the active mcp-ai-helper backlog from legacy tasks/*.lean JSON-comment files into the Lean-native registry created by task-036. This is a mechanical migration after the registry/export path exists. The Lean registry becomes the canonical representation for active tasks.

Required scope:
1. Encode every currently active todo/in_progress/blocked task as a Lean-native artifact value in the registry or imported registry modules.
2. Preserve ids, titles, statuses, priorities, tags and bodies at minimum.
3. Preserve explicit dependencies/blockers only where known; do not invent dependency edges.
4. Keep legacy tasks/*.lean readable for rollback during this task, but mark them as legacy/projection in documentation or comments if appropriate.
5. Ensure task-037 exporter returns the migrated active backlog.
6. Ensure `lake build` proves the migrated registry invariants.

Out of scope:
- No migration of full historical done task archive unless already represented by task-034/task-035 samples.
- No semantic rewrite of task contents.
- No changing tests to fit migration output unless the old test explicitly assumed legacy storage as canonical and is updated with a clear reason.
