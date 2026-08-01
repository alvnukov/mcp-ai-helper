---
id: mcp-helper-llm-reliability-audit
title: Устранить системные дефекты взаимодействия LLM с mcp-ai-helper
status: blocked
priority: critical
model_level: very_high
task_type: chore
tags:
    - reliability
    - llm
    - mcp
    - epic
    - workflow
    - tasks
    - commands
acceptance_criteria:
    - Every confirmed P0/P1 finding is represented by an executable child task or an existing linked task.
    - Child fixes preserve repo scoping, guarded edits, owned-file commits, and fail-closed behavior.
    - Parent is closed only after focused regressions and the required broader Go quality gate pass.
verification_plan:
    - Review child task completion and regression evidence.
    - Run make quality only at final epic closeout.
created_at: "2026-08-01T11:19:08.029976Z"
updated_at: "2026-08-01T11:19:08.029976Z"
---

## Body

Parent reliability epic for confirmed workflow ordering, command lifecycle, task consistency, output retention, read purity, and self-description defects. Parent remains blocked until the child fixes are complete and their focused regression gates pass.

## Acceptance Criteria

- Every confirmed P0/P1 finding is represented by an executable child task or an existing linked task.
- Child fixes preserve repo scoping, guarded edits, owned-file commits, and fail-closed behavior.
- Parent is closed only after focused regressions and the required broader Go quality gate pass.

## Verification Plan

1. Review child task completion and regression evidence.
2. Run make quality only at final epic closeout.
