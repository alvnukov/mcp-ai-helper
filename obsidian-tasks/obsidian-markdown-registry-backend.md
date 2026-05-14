---
id: obsidian-markdown-registry-backend
title: Реализовать Obsidian Markdown TaskRegistryBackend
status: done
priority: high
model_level: medium
task_type: implementation
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - obsidian
  - markdown
  - backend
  - implementation
  - tests
  - weak-model
  - tdd
branch: implementation/obsidian-markdown-registry-backend
worktree_path: .worktrees/obsidian-markdown-registry-backend
acceptance_criteria:
  - Parser/writer tests and fixtures exist before implementation behavior is accepted.
  - Obsidian backend parses valid task notes into the canonical task record without losing required fields.
  - Writer emits deterministic Markdown/frontmatter that can be parsed back into the same canonical record.
  - Invalid, stale, duplicate, or unsupported task notes fail with structured diagnostics.
  - The backend exposes the hooks/config inputs needed by task-registry-backend-config-selection, but does not change default Lean behavior by itself.
  - Focused fixtures cover parent task, child task, multiline body, acceptance criteria, verification plan, tags and model_level.
  - Code is typed, deterministic, compact, and avoids global side effects or broad formatting churn.
verification_plan:
  - Run targeted Obsidian backend parser/writer tests.
  - Run one Lean backend compatibility test if shared canonical record code changed.
  - Run the existing lint/check command for touched Go packages if available; otherwise state that no lint gate was available.
  - Inspect one golden fixture diff to confirm deterministic output and no hidden field drops.
created_at: 2026-05-14T09:15:25.492137Z
updated_at: 2026-05-14T09:15:25.492137Z
---

## Body

Weak-model execution contract: implement from fixtures and tests first. Start with the approved Obsidian schema examples: valid parent, valid child, multiline body, acceptance criteria, verification plan, tags, model_level and at least one invalid/lossy fixture. Write parser/writer tests before implementation. Keep parser/writer logic typed and deterministic; do not parse with fragile ad-hoc string slicing when a structured frontmatter/Markdown parser or small explicit state machine is appropriate.

Implement the Obsidian Markdown backend defined by obsidian-registry-backend-contract. It must read and write human-editable Markdown task notes, validate required metadata, preserve body/acceptance/verification content deterministically, and report unsupported or lossy cases explicitly.

This task owns the Obsidian backend adapter itself, not global config routing. Lean must remain default until task-registry-backend-config-selection wires user-configurable selection. Do not implement cross-format import/export in this task. Before finalization, run focused Obsidian backend tests and the existing lint/check gate for touched Go packages if configured; if no lint gate exists, report that explicitly.

## Acceptance Criteria

- Parser/writer tests and fixtures exist before implementation behavior is accepted.
- Obsidian backend parses valid task notes into the canonical task record without losing required fields.
- Writer emits deterministic Markdown/frontmatter that can be parsed back into the same canonical record.
- Invalid, stale, duplicate, or unsupported task notes fail with structured diagnostics.
- The backend exposes the hooks/config inputs needed by task-registry-backend-config-selection, but does not change default Lean behavior by itself.
- Focused fixtures cover parent task, child task, multiline body, acceptance criteria, verification plan, tags and model_level.
- Code is typed, deterministic, compact, and avoids global side effects or broad formatting churn.

## Verification Plan

1. Run targeted Obsidian backend parser/writer tests.
2. Run one Lean backend compatibility test if shared canonical record code changed.
3. Run the existing lint/check command for touched Go packages if available; otherwise state that no lint gate was available.
4. Inspect one golden fixture diff to confirm deterministic output and no hidden field drops.
