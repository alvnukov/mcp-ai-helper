---
id: task-089
title: Сформировать MVP scope и threat model для bounded external worker
status: todo
priority: high
model_level: medium
tags:
  - research
  - design
  - agents
  - models
  - worktree
  - mcp
  - security
worktree_path: .worktrees/task-089
acceptance_criteria:
  - MVP scope is small enough to be implemented safely after design approval.
  - Threat model covers direct access denial, repo-content-as-data, prompt injection, worktree isolation, ownership violations and budget/loop abuse.
  - Non-goals exclude full autonomous agent behavior, arbitrary shell/network access and direct filesystem/git access.
  - Failure states include success, needs_review, blocked, policy_violation and check_failed.
verification_plan:
  - Review design against the original task-049 research list.
  - Check that task-014/task-015/task-020 dependencies are referenced instead of duplicated.
  - Confirm no implementation is requested or performed.
created_at: 2026-05-14T09:15:25.486635Z
updated_at: 2026-05-14T09:15:25.486635Z
---

## Body

Produce the first design artifact for bounded external LLM worker: recommended MVP scope, explicit non-goals, threat model, isolation boundaries, worktree lifecycle, ownership policy, command policy dependency, prompt-injection boundary and failure semantics. Do not define final tool schemas or implementation backlog here except as references to task-090/task-103.

## Acceptance Criteria

- MVP scope is small enough to be implemented safely after design approval.
- Threat model covers direct access denial, repo-content-as-data, prompt injection, worktree isolation, ownership violations and budget/loop abuse.
- Non-goals exclude full autonomous agent behavior, arbitrary shell/network access and direct filesystem/git access.
- Failure states include success, needs_review, blocked, policy_violation and check_failed.

## Verification Plan

1. Review design against the original task-049 research list.
2. Check that task-014/task-015/task-020 dependencies are referenced instead of duplicated.
3. Confirm no implementation is requested or performed.
