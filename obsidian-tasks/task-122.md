---
id: task-122
title: Запретить generic tools читать и менять Lean source files
status: done
priority: high
model_level: medium
tags:
  - security
  - tasks
  - lean
  - mcp
  - pipeline
worktree_path: .worktrees/task-122
acceptance_criteria:
  - Generic file read/snapshot/edit/search surfaces reject or skip .lean content.
  - Repo-scoped command execution used by collect_command_output, run_pipeline, and workflow command steps fails closed when the command explicitly references Lean source or task registry paths.
  - Task-facing task tools remain the permitted path for Lean task registry mutation.
verification_plan:
  - Run targeted Go tests for fileops, command, and pipeline packages.
created_at: 2026-05-14T09:15:25.490822Z
updated_at: 2026-05-14T09:15:25.490822Z
---

## Body

Generic file tools and repo-scoped command/pipeline/workflow command steps must not be usable as a bypass for reading or editing Lean source files. Task-facing Lean/Lake task tools remain the allowed path for task registry operations.

## Acceptance Criteria

- Generic file read/snapshot/edit/search surfaces reject or skip .lean content.
- Repo-scoped command execution used by collect_command_output, run_pipeline, and workflow command steps fails closed when the command explicitly references Lean source or task registry paths.
- Task-facing task tools remain the permitted path for Lean task registry mutation.

## Verification Plan

1. Run targeted Go tests for fileops, command, and pipeline packages.
