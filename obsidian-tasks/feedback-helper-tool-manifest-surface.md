---
id: feedback-helper-tool-manifest-surface
title: 'Feedback: add MCP-only tool manifest for helper surface diagnostics'
status: done
priority: high
model_level: medium
tags:
    - feedback
    - mcp
    - discoverability
    - tools
    - diagnostics
acceptance_criteria:
    - A compact MCP tool_manifest tool is registered alongside assistant_guidance.
    - tool_manifest returns sorted registered tool names and count from the running helper server.
    - tool_manifest includes issue_add, issue_list, issue_accept, command_get, and filter_command_history when those tools are registered.
    - assistant_guidance mentions tool_manifest as the first MCP-only check when named tools are not visible.
verification_plan:
    - Run focused MCP guidance/manifest registration tests.
    - Run existing issue and command registration tests.
created_at: "2026-06-17T09:56:40.77061Z"
updated_at: "2026-06-17T10:02:21.531157Z"
---

## Body

Live Codex feedback showed a mismatch between helper source/tool guidance and the callable tool surface visible to the model. Existing issue_* and command_get handlers are registered in source, but the caller needs a helper-owned way to list the server's actual registered tools and diagnose MCP client rediscovery/surface gaps without using external discovery tools.

## Acceptance Criteria

- A compact MCP tool_manifest tool is registered alongside assistant_guidance.
- tool_manifest returns sorted registered tool names and count from the running helper server.
- tool_manifest includes issue_add, issue_list, issue_accept, command_get, and filter_command_history when those tools are registered.
- assistant_guidance mentions tool_manifest as the first MCP-only check when named tools are not visible.

## Verification Plan

1. Run focused MCP guidance/manifest registration tests.
2. Run existing issue and command registration tests.
