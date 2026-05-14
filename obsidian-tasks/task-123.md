---
id: task-123
title: Инвентаризировать config, pipeline и model-facing output surfaces для secrets
status: done
priority: high
model_level: low
task_type: inventory
parent_id: task-122
tags:
  - security
  - secrets
  - config
  - pipeline
  - inventory
  - low-model
branch: inventory/task-123
worktree_path: .worktrees/task-123
acceptance_criteria:
  - Inventory informed config schema, pipeline/workflow request fields, command output/history redaction and MCP schema tests.
verification_plan:
  - Review changed files and focused package tests.
created_at: 2026-05-14T09:15:25.490944Z
updated_at: 2026-05-14T09:15:25.490944Z
---

## Body

Completed as part of the secret implementation path: relevant config, pipeline/workflow, command output/history and MCP schema surfaces were identified and covered by focused tests.

## Acceptance Criteria

- Inventory informed config schema, pipeline/workflow request fields, command output/history redaction and MCP schema tests.

## Verification Plan

1. Review changed files and focused package tests.
