---
id: server-version-must-come-from-build-info
title: Версия сервера должна браться из build info
status: todo
priority: low
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - mcp
    - versioning
    - release
acceptance_criteria:
    - server.New не хардкодит 0.1.0, версия приходит из runtime/debug BuildInfo (fallback при go run)
    - в README/RELEASE нет противоречий
created_at: "2026-08-14T21:18:50.512331Z"
updated_at: "2026-08-14T21:18:50.512331Z"
---

## Body

server.go:178 — версия «0.1.0» захардкожена при существующем release-процессе (brew tap, GitHub Releases).

## Acceptance Criteria

- server.New не хардкодит 0.1.0, версия приходит из runtime/debug BuildInfo (fallback при go run)
- в README/RELEASE нет противоречий
