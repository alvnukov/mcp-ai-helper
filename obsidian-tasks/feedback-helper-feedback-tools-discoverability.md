---
id: feedback-helper-feedback-tools-discoverability
title: 'Feedback: document feedback and command-history tools in assistant guidance'
status: done
priority: high
model_level: medium
tags:
    - feedback
    - guidance
    - commands
    - issues
    - mcp
    - discoverability
acceptance_criteria:
    - assistant_guidance output names command_get and filter_command_history for retained command output.
    - assistant_guidance output names issue_add, issue_list, and issue_accept for feedback intake.
    - Existing configured assistant_guidance values receive the mandatory discovery addendum at runtime without losing custom guidance.
    - Generated/default guidance and config example include the same discovery hints.
verification_plan:
    - Run focused config guidance tests.
    - Run focused MCP guidance and tool-registration tests.
created_at: "2026-06-17T09:47:40.450354Z"
updated_at: "2026-06-17T09:49:48.518374Z"
---

## Body

Live Codex feedback showed that command_get/filter_command_history and issue feedback tools exist in source but were not obvious to the calling model from assistant_guidance. Fix guidance so existing configs still return a mandatory discovery addendum and generated/example configs document the tool names.

## Acceptance Criteria

- assistant_guidance output names command_get and filter_command_history for retained command output.
- assistant_guidance output names issue_add, issue_list, and issue_accept for feedback intake.
- Existing configured assistant_guidance values receive the mandatory discovery addendum at runtime without losing custom guidance.
- Generated/default guidance and config example include the same discovery hints.

## Verification Plan

1. Run focused config guidance tests.
2. Run focused MCP guidance and tool-registration tests.
