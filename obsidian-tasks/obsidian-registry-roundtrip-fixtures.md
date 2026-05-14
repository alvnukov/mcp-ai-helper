---
id: obsidian-registry-roundtrip-fixtures
title: Добавить Obsidian/Lean round-trip fixtures и usage docs
status: done
priority: medium
model_level: low
task_type: tests
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - lean
  - obsidian
  - config
  - fixtures
  - tests
  - docs
  - weak-model
branch: tests/obsidian-registry-roundtrip-fixtures
worktree_path: .worktrees/obsidian-registry-roundtrip-fixtures
acceptance_criteria:
  - Fixtures cover parent epic, executable child task, multiline body, tags, model_level, priority, branch/worktree, acceptance criteria and verification plan.
  - Usage docs show how a user selects lean or obsidian backend in config, how to run dry-run import/export, and how fail-closed loss reports work.
  - Fixture names and expected outputs are compact enough for low/medium models to use without reading the whole registry.
  - At least one fixture/test would fail if a Lean-required field were silently dropped.
  - No broad documentation rewrite, production parser rewrite or unrelated snapshot churn is included.
verification_plan:
  - Run only focused fixture/docs or snapshot tests introduced by this task.
  - Check that at least one fixture would fail if a Lean-required field were silently dropped.
  - Check docs state Lean default, explicit Obsidian config, and no silent auto-detection.
  - Run docs/schema/lint gates only if the repo already defines them for touched files; otherwise state they are unavailable.
  - Confirm no implementation scope from parser/import/export/config tasks is pulled into this task.
created_at: 2026-05-14T09:15:25.492419Z
updated_at: 2026-05-14T09:15:25.492419Z
---

## Body

Weak-model execution contract: this is a low-level tests/docs task. Do not change parser/import/export behavior except minimal fixture wiring required by existing test harnesses. Start by adding fixtures that encode expected behavior; if fixture tests fail because implementation is missing, report the dependency instead of changing production logic outside this task. Keep docs compact and operational.

Add compact fixtures and usage documentation for the Obsidian task registry format, config-driven backend selection, and Lean/Obsidian import-export path after the backend contract is approved. Fixtures must intentionally include Lean-specific fields so loss-prevention behavior is tested instead of only simple happy paths.

The docs must tell a weak model exactly how to verify its work: run focused fixture/snapshot tests, run docs/schema checks if configured, and run existing lint/check gates only when touched files require them. Do not run broad suites without a concrete shared-code risk.

## Acceptance Criteria

- Fixtures cover parent epic, executable child task, multiline body, tags, model_level, priority, branch/worktree, acceptance criteria and verification plan.
- Usage docs show how a user selects lean or obsidian backend in config, how to run dry-run import/export, and how fail-closed loss reports work.
- Fixture names and expected outputs are compact enough for low/medium models to use without reading the whole registry.
- At least one fixture/test would fail if a Lean-required field were silently dropped.
- No broad documentation rewrite, production parser rewrite or unrelated snapshot churn is included.

## Verification Plan

1. Run only focused fixture/docs or snapshot tests introduced by this task.
2. Check that at least one fixture would fail if a Lean-required field were silently dropped.
3. Check docs state Lean default, explicit Obsidian config, and no silent auto-detection.
4. Run docs/schema/lint gates only if the repo already defines them for touched files; otherwise state they are unavailable.
5. Confirm no implementation scope from parser/import/export/config tasks is pulled into this task.
