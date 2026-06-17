---
id: feedback-registry-missing-obsidian-path-breaks-task-protocol
title: 'Feedback: registry management breaks when configured Obsidian path is missing'
status: todo
priority: high
model_level: medium
tags:
    - feedback
    - tasks
    - registry
    - obsidian
    - config
    - diagnostics
created_at: "2026-06-17T09:31:46.431472Z"
updated_at: "2026-06-17T09:31:46.431472Z"
---

## Body

Observed bug: task_current for /Users/zol/src/lesta_mods failed with 'task_registry.obsidian.path is not initialized: /Users/zol/src/lesta_mods/obsidian-tasks does not exist', which blocks the mandatory Repo Task Protocol before any task context can be selected. Fix registry behavior so missing configured Obsidian path has an explicit recovery path: auto-initialize only when policy allows, otherwise return structured diagnostics with exact next actions and do not make task_current unusable without a clear setup/migration route. Also verify repo-path mismatch diagnostics: Codex workspace was /Users/zol/src/lesta_mods while helper dev repo was /Users/zol/src/mcp-ai-helper, so tools should make this mismatch obvious.
