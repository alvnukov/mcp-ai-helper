---
id: auto-validate-auto-heal-obsidian-task-registry
title: Auto-validate and auto-heal Obsidian task registry projections
status: done
priority: high
model_level: medium
tags:
    - tasks
    - registry
    - obsidian
    - diagnostics
    - auto-heal
acceptance_criteria:
    - List-style task tools remain usable when one Obsidian projection file is invalid.
    - Missing Obsidian registry directories are auto-created and list/current return zero tasks without tool errors.
    - Filename/frontmatter id mismatch is auto-healed by safe rename when no target file exists.
    - Conflicting or malformed projection files are not overwritten or deleted and are surfaced as diagnostics.
    - task_batch_upsert reports changed files for Obsidian and Lean-backed mutations.
    - List-style responses include counts by status and compact registry validation metadata.
verification_plan:
    - Run focused Obsidian backend tests for missing-dir auto-init, degraded invalid notes and safe filename/id mismatch repair.
    - Run focused task MCP/tool tests for changed_files/count metadata compatibility.
    - Run targeted internal/config, internal/tasks and internal/mcp Go tests.
created_at: "2026-06-26T14:42:19.223594Z"
updated_at: "2026-06-26T16:13:51.12989Z"
---

## Body

Fix Obsidian task registry robustness so one bad projection file cannot make task list/current/search/graph/context unusable. Missing configured Obsidian registry directories must be auto-created when an LLM asks to list tasks, returning an empty task set instead of an error. Registry reads validate notes, safely auto-heal filename/id mismatches by rename only when the target filename is free, preserve conflicted or invalid files without overwriting, and report compact diagnostics plus changed files from task tools.

## Acceptance Criteria

- List-style task tools remain usable when one Obsidian projection file is invalid.
- Missing Obsidian registry directories are auto-created and list/current return zero tasks without tool errors.
- Filename/frontmatter id mismatch is auto-healed by safe rename when no target file exists.
- Conflicting or malformed projection files are not overwritten or deleted and are surfaced as diagnostics.
- task_batch_upsert reports changed files for Obsidian and Lean-backed mutations.
- List-style responses include counts by status and compact registry validation metadata.

## Verification Plan

1. Run focused Obsidian backend tests for missing-dir auto-init, degraded invalid notes and safe filename/id mismatch repair.
2. Run focused task MCP/tool tests for changed_files/count metadata compatibility.
3. Run targeted internal/config, internal/tasks and internal/mcp Go tests.
