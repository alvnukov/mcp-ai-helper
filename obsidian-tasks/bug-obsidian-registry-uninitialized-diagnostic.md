---
id: bug-obsidian-registry-uninitialized-diagnostic
title: Fail closed when Obsidian task registry is not initialized
status: done
priority: high
model_level: medium
tags:
    - tasks
    - registry
    - obsidian
    - diagnostics
    - bug
created_at: "2026-05-27T20:40:24.536453Z"
updated_at: "2026-05-27T20:40:24.558564Z"
---

## Body

When task tools are configured for the Obsidian backend but the configured task directory is missing or not initialized, task_current must not look like an empty registry. It must fail closed with compact actionable diagnostics that tell the caller to create the configured directory, update .mcp-ai-helper.yaml, switch back to lean, or inspect server_setup_guidance.
