---
id: feedback-background-command-process-control-surface
title: 'Feedback: добавить MCP surface для списка и управления фоновыми command/process jobs'
status: todo
priority: high
model_level: medium
tags:
    - feedback
    - commands
    - processes
    - mcp
    - observability
    - control
created_at: "2026-06-17T09:31:46.429713Z"
updated_at: "2026-06-17T09:31:46.429713Z"
---

## Body

Context from live Codex session: model can inspect a known retained command via command_get(command_id), but there is no visible helper API to list its own active/background processes or manage them. Add bounded MCP tools for listing helper-owned command/process jobs and controlling them: get current/own process info, list by repo/session/status, inspect one job, abort/terminate, pause/suspend, resume/continue where OS/runtime supports it. Requirements: bounded output, repo/session scoping, stable job ids, clear lifecycle states, permission/policy checks, no raw shell bypass, durable status after client timeout, structured errors for unsupported pause/resume, and tests covering list/get/abort plus unsupported controls.
