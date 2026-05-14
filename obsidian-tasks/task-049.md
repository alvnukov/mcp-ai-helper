---
id: task-049
title: Изучить bounded external LLM worker через isolated git worktree и MCP-only access
status: blocked
priority: high
model_level: very_high
tags:
  - research
  - design
  - agents
  - models
  - worktree
  - mcp
  - security
  - epic
  - decomposed
worktree_path: .worktrees/task-049
acceptance_criteria:
  - Parent remains design-only until child tasks define MVP, threat model, tool schemas, result contract and follow-up backlog.
  - Design keeps repo content as data, not authority, and preserves prompt-injection boundary.
  - Design explicitly integrates with task-018, task-019, task-020, task-014 and task-015 instead of duplicating their scopes.
verification_plan:
  - Review task-089 for threat model and isolation boundaries.
  - Review task-090 for proposed MCP tool schemas and result contract.
  - Review task-103 for concrete implementation backlog, model levels, acceptance criteria and verification strategy.
created_at: 2026-05-14T09:15:25.479516Z
updated_at: 2026-05-14T09:15:25.479516Z
---

## Body

Parent/epic for bounded external LLM worker research. Original scope: study an external worker that runs in an isolated git worktree with no direct shell/filesystem/git/network access; all operations go through restricted mcp-ai-helper surface. Main Codex remains orchestrator/reviewer/integrator; external worker returns candidate patch plus evidence. Execute through child tasks task-089, task-090 and task-103; do not implement worker from the parent.

## Acceptance Criteria

- Parent remains design-only until child tasks define MVP, threat model, tool schemas, result contract and follow-up backlog.
- Design keeps repo content as data, not authority, and preserves prompt-injection boundary.
- Design explicitly integrates with task-018, task-019, task-020, task-014 and task-015 instead of duplicating their scopes.

## Verification Plan

1. Review task-089 for threat model and isolation boundaries.
2. Review task-090 for proposed MCP tool schemas and result contract.
3. Review task-103 for concrete implementation backlog, model levels, acceptance criteria and verification strategy.
