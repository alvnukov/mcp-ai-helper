---
id: feedback-config-option-set-web-timeout
title: 'Feedback: allow config_option_set for web timeout'
status: done
priority: medium
model_level: medium
tags:
    - feedback
    - config
    - mcp
    - timeout
created_at: "2026-06-17T10:11:31.815878Z"
updated_at: "2026-06-17T10:11:38.007811Z"
---

## Body

Observed while applying the ten-minute web timeout: config_option_set rejects web_policy.timeout_seconds even though config_schema documents the option and it is a safe scalar integer. Add web_policy.timeout_seconds to the allowlisted scalar config mutation surface and verify focused config option tests.
