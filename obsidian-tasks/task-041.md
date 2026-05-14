---
id: task-041
title: Retire legacy task storage as canonical state
status: done
priority: high
tags:
  - tasks
  - lean
  - migration
  - guidance
  - cleanup
worktree_path: .worktrees/task-041
created_at: 2026-05-14T09:15:25.47828Z
updated_at: 2026-05-14T09:15:25.47828Z
---

## Body

After Lean-backed reads and mutations are working, retire legacy tasks/*.lean JSON-comment storage as canonical state. Legacy files may remain as an archive/projection during transition, but every guide and tool path must treat the Lean registry as source of truth.

Required scope:
1. Update assistant_guidance/server_setup_guidance/project docs to state that Lean/Lake project registry is canonical for migrated repos.
2. Mark legacy tasks/*.lean storage as fallback/archive/projection, not source of truth.
3. Ensure task tools report or expose whether they are using Lean-backed mode or legacy fallback.
4. Remove or update tests that assert legacy storage is canonical, with explicit rationale; keep fallback tests.
5. Add a compatibility note explaining how non-Lake repos still use legacy fallback until migrated.

Out of scope:
- No deletion of legacy files unless there is a separate reviewed cleanup task.
- No task graph feature expansion.
- No LSP/editor integration.
