---
id: run-workflow-write-file-step
title: Добавить write_file step в run workflow
status: done
priority: high
model_level: medium
task_type: feature
tags:
    - workflow
    - fileops
    - mcp
    - schema
    - reliability
created_at: "2026-08-08T11:34:58.02252Z"
updated_at: "2026-08-08T11:44:41.886243Z"
---

## Body

Acceptance: (1) write_file creates whole file with content/content_b64 and reports changed file; (2) expected_hash/mode and malformed-base64 preflight match edit.write safety contract; (3) schema/docs/skill discover step and focused+quality gates pass.

Verification: targeted Go tests; make quality.
