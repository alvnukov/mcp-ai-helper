---
id: dev-wrapper-rebuild-must-find-go
title: Dev wrapper rebuild must find Go outside interactive PATH
status: done
priority: high
model_level: medium
task_type: bug
tags:
    - wrapper
    - rebuild
    - go
    - path
    - runtime
acceptance_criteria:
    - dev_rebuild_server resolves a usable Go executable when the inherited PATH is minimal.
    - Resolution does not hardcode one repository or one user-specific absolute path.
    - A focused wrapper test covers the minimal-PATH case.
    - The rebuilt child restarts without closing wrapper stdio.
verification_plan:
    - Run focused cmd/mcp-ai-helper-dev tests.
    - Run make quality and invoke dev_rebuild_server through the wrapper.
created_at: "2026-08-16T11:19:30.535584Z"
updated_at: "2026-08-16T11:28:30.153534Z"
---

## Body

dev_rebuild_server fails when the MCP client launches the wrapper with a minimal PATH that omits the Go toolchain. Resolve the Go executable safely without tying the wrapper to one repository or one user path.

## Acceptance Criteria

- dev_rebuild_server resolves a usable Go executable when the inherited PATH is minimal.
- Resolution does not hardcode one repository or one user-specific absolute path.
- A focused wrapper test covers the minimal-PATH case.
- The rebuilt child restarts without closing wrapper stdio.

## Verification Plan

1. Run focused cmd/mcp-ai-helper-dev tests.
2. Run make quality and invoke dev_rebuild_server through the wrapper.
