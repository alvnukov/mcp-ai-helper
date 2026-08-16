---
id: workflow-explicit-task-transition-must-not-auto-close-again
title: Workflow explicit task transition must suppress duplicate auto-closeout
status: done
priority: high
model_level: high
task_type: bug
tags:
    - workflow
    - tasks
    - lifecycle
    - git
    - atomicity
acceptance_criteria:
    - A successful explicit task_transition for current_task_id suppresses the deferred duplicate status write.
    - Implicit current_task_id lifecycle remains unchanged when no explicit transition succeeds.
    - A workflow commit that includes the task registry file does not dirty it again after commit.
verification_plan:
    - Run focused pipeline lifecycle tests including transition call count.
    - Run make quality and a live atomic workflow status check.
created_at: "2026-08-16T11:30:04.457387Z"
updated_at: "2026-08-16T11:38:27.426121Z"
---

## Body

When a workflow supplies current_task_id and also successfully runs a task_transition for that same task, the deferred automatic lifecycle update writes the final status a second time after git_commit_owned. This dirties the task registry immediately after a successful atomic commit.

## Acceptance Criteria

- A successful explicit task_transition for current_task_id suppresses the deferred duplicate status write.
- Implicit current_task_id lifecycle remains unchanged when no explicit transition succeeds.
- A workflow commit that includes the task registry file does not dirty it again after commit.

## Verification Plan

1. Run focused pipeline lifecycle tests including transition call count.
2. Run make quality and a live atomic workflow status check.
