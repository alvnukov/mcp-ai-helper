---
id: obsidian-registry-capability-inventory
title: Инвентаризировать Lean task registry capabilities для parity matrix
status: done
priority: high
model_level: low
task_type: design
parent_id: obsidian-task-registry-backend
tags:
  - tasks
  - registry
  - lean
  - obsidian
  - inventory
  - design
  - weak-model
branch: design/obsidian-registry-capability-inventory
worktree_path: .worktrees/obsidian-registry-capability-inventory
acceptance_criteria:
  - Inventory covers all task fields visible through current task tools and the Lean registry projection.
  - Parity matrix identifies which Lean capabilities must survive Obsidian read/write, config selection and import/export.
  - Missing or inferred semantics are explicitly marked instead of guessed.
  - Output includes a focused test/fixture checklist for the medium implementation tasks.
  - No code behavior, task status, or registry format is changed except adding the inventory artifact if required by the existing docs/design pattern.
verification_plan:
  - Use only bounded MCP helper reads or focused helper-supported probes.
  - Validate the matrix against at least one blocked epic and one executable child task.
  - Check the final note distinguishes confirmed facts, assumptions and unknowns.
  - Confirm the output gives task authors enough information for the backend contract task.
created_at: 2026-05-14T09:15:25.491712Z
updated_at: 2026-05-14T09:15:25.491712Z
---

## Body

Weak-model execution contract: this is a low-level design/inventory task, not an implementation task. Work from observable facts only. Do not edit product code, do not change tests to fit assumptions, and do not invent unsupported registry semantics. Use bounded MCP helper reads/probes and record evidence compactly.

Inventory the current Lean-backed task registry capabilities before designing Obsidian support: task fields, statuses, priorities, model_level, parent_id, task_type, branch/worktree paths, tags, body, acceptance criteria, verification plan, projection/provenance fields, timestamps where applicable, task_current/task_batch_upsert behavior, and any workflow assumptions around task transitions and commits.

Output a compact parity matrix that marks each field/semantic as required, optional, derived, unsupported, or explicitly out of scope for Obsidian. Include a follow-up test checklist for implementation tasks: representative fixtures, expected loss-report cases, and minimal commands/linters that later implementation tasks must run. If a needed fact is unavailable through current MCP helper surface, mark it as unknown with a concrete blocker instead of guessing.

## Acceptance Criteria

- Inventory covers all task fields visible through current task tools and the Lean registry projection.
- Parity matrix identifies which Lean capabilities must survive Obsidian read/write, config selection and import/export.
- Missing or inferred semantics are explicitly marked instead of guessed.
- Output includes a focused test/fixture checklist for the medium implementation tasks.
- No code behavior, task status, or registry format is changed except adding the inventory artifact if required by the existing docs/design pattern.

## Verification Plan

1. Use only bounded MCP helper reads or focused helper-supported probes.
2. Validate the matrix against at least one blocked epic and one executable child task.
3. Check the final note distinguishes confirmed facts, assumptions and unknowns.
4. Confirm the output gives task authors enough information for the backend contract task.
