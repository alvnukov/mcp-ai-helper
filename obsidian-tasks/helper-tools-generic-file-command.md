---
id: helper-tools-generic-file-command
title: Распознавать обход профильных helper tools через generic file/command
status: done
priority: high
model_level: high
task_type: feature
tags:
    - issue
    - feedback
    - security
    - policy
    - tasks
    - config
    - secrets
    - diagnostics
    - llm-reliability
acceptance_criteria:
    - Generic file/read/list/snapshot/edit access to the configured task registry and raw helper configs is denied with a precise helper-tool alternative.
    - Command, pipeline and workflow command surfaces recognize direct high-confidence task-registry, raw-config and secret-environment access patterns through one shared policy path.
    - Existing legitimate source access and ordinary targeted commands remain allowed; the guard is explicitly heuristic rather than a sandbox.
    - Focused tests cover configured Obsidian paths, legacy Lean paths, global and repo configs, secret environment dumps, explanations and allowed controls.
verification_plan:
    - Run focused Go tests for the shared policy, fileops, command and affected MCP/pipeline surfaces.
    - Run make quality because the policy path is shared by all command and generic file/edit entry points.
    - Inspect the owned-file diff and commit code plus task transition atomically.
created_at: "2026-08-22T09:19:53.145837Z"
updated_at: "2026-08-22T09:48:27.421972Z"
---

## Body

Добавить эвристический misuse detector: распознавать обычные попытки читать или менять task registry и raw helper config через generic file/edit/search/list/command/run, а также очевидные попытки получить секреты через raw config или environment dump. Отказывать локально с точным объяснением и next_call к task/config/secret_handles. Это диагностический guardrail, не security sandbox.

## Acceptance Criteria

- Generic file/read/list/snapshot/edit access to the configured task registry and raw helper configs is denied with a precise helper-tool alternative.
- Command, pipeline and workflow command surfaces recognize direct high-confidence task-registry, raw-config and secret-environment access patterns through one shared policy path.
- Existing legitimate source access and ordinary targeted commands remain allowed; the guard is explicitly heuristic rather than a sandbox.
- Focused tests cover configured Obsidian paths, legacy Lean paths, global and repo configs, secret environment dumps, explanations and allowed controls.

## Verification Plan

1. Run focused Go tests for the shared policy, fileops, command and affected MCP/pipeline surfaces.
2. Run make quality because the policy path is shared by all command and generic file/edit entry points.
3. Inspect the owned-file diff and commit code plus task transition atomically.
