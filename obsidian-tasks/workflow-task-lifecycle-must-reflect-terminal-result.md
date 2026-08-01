---
id: workflow-task-lifecycle-must-reflect-terminal-result
title: Workflow task lifecycle must reflect terminal execution result
status: done
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - tasks
    - lifecycle
    - state
    - reliability
    - llm
acceptance_criteria:
    - A workflow that fails preflight does not mutate current_task_id before any executable step.
    - task_on_success is applied only after every workflow step succeeds; a late-step failure cannot leave the task done.
    - Failure responses report task-registry mutations in changed_files when a lifecycle transition actually occurred.
    - Focused regression tests cover failed preflight and failed late step without weakening existing workflow contracts.
verification_plan:
    - Run focused workflow lifecycle tests.
    - Run make quality and make lint because workflow execution and task state are cross-cutting.
created_at: "2026-08-01T17:35:39.942902Z"
updated_at: "2026-08-01T17:50:27.397455Z"
---

## Body

run workflow currently applies implicit task_on_start before workflow preflight and may apply task_on_success before later/external steps finish. This produces false task state and incomplete changed-file evidence. Make task lifecycle transitions terminally correct and observable without weakening workflow gates.

## Acceptance Criteria

- A workflow that fails preflight does not mutate current_task_id before any executable step.
- task_on_success is applied only after every workflow step succeeds; a late-step failure cannot leave the task done.
- Failure responses report task-registry mutations in changed_files when a lifecycle transition actually occurred.
- Focused regression tests cover failed preflight and failed late step without weakening existing workflow contracts.

## Verification Plan

1. Run focused workflow lifecycle tests.
2. Run make quality and make lint because workflow execution and task state are cross-cutting.
