---
id: task-104
title: Добавить self-describing task graph/context MCP surface
status: blocked
priority: high
model_level: very_high
task_type: epic
tags:
  - tasks
  - context
  - mcp
  - llm-tools
  - graph
  - self-describing
  - epic
  - decomposed
branch: epic/task-104
worktree_path: .worktrees/task-104
acceptance_criteria:
  - Parent remains non-executable until design, graph tool, context tool and self-description/guidance coverage are complete through medium/low child tasks.
  - Feature provides short factual context that reduces LLM speculation instead of returning an unbounded task dump.
  - Tool contracts are discoverable from MCP surface itself: tool descriptions, schema/help output, assistant guidance and structured errors.
  - Graph/context responses distinguish explicit facts from inferred relationships and expose truncation/omission metadata.
verification_plan:
  - Review task-105 child outputs for final tool contracts and field semantics before implementation children start.
  - Check task-106 child outputs implement factual graph extraction with bounded output and provenance.
  - Check task-107 child outputs implement execution-oriented context projection for a selected task.
  - Check task-108 child outputs prove discoverability from MCP responses without README/manual registry reads.
created_at: 2026-05-14T09:15:25.48845Z
updated_at: 2026-05-14T09:15:25.48845Z
---

## Body

Parent/epic. Add a compact task graph and execution-context surface so LLMs can understand project goals, task decomposition, dependencies, already completed direction, planned work, boundaries and verification needs without reading README/docs or raw task registry files. Decomposed into task-105, task-106, task-107 and task-108; those are coordination parents and should be executed through their medium/low children. Do not implement directly from this parent.

## Acceptance Criteria

- Parent remains non-executable until design, graph tool, context tool and self-description/guidance coverage are complete through medium/low child tasks.
- Feature provides short factual context that reduces LLM speculation instead of returning an unbounded task dump.
- Tool contracts are discoverable from MCP surface itself: tool descriptions, schema/help output, assistant guidance and structured errors.
- Graph/context responses distinguish explicit facts from inferred relationships and expose truncation/omission metadata.

## Verification Plan

1. Review task-105 child outputs for final tool contracts and field semantics before implementation children start.
2. Check task-106 child outputs implement factual graph extraction with bounded output and provenance.
3. Check task-107 child outputs implement execution-oriented context projection for a selected task.
4. Check task-108 child outputs prove discoverability from MCP responses without README/manual registry reads.
