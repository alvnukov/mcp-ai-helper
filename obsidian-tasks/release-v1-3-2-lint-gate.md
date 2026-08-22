---
id: release-v1-3-2-lint-gate
title: Закрыть lint-блокеры перед patch-релизом v1.3.2
status: done
priority: critical
model_level: medium
task_type: bug
tags:
    - release
    - v1.3.2
    - lint
    - ci
    - release-blocker
acceptance_criteria:
    - make lint passes on the exact committed HEAD.
    - Jira and Confluence fail-closed allowlist behavior is unchanged.
    - Goal-tag task ordering behavior is unchanged.
    - make quality and actionlint pass before the release tag is pushed.
verification_plan:
    - Run focused tests for config repository tools and current task ordering.
    - Run make quality, make lint, and actionlint on a clean archive of HEAD.
created_at: "2026-08-22T17:28:48.594693Z"
updated_at: "2026-08-22T17:36:36.077259Z"
---

## Body

Fix the three deterministic golangci-lint findings at HEAD without changing Jira/Confluence allowlist behavior or task current selection semantics, then prove the exact release gates on a clean HEAD archive.

## Acceptance Criteria

- make lint passes on the exact committed HEAD.
- Jira and Confluence fail-closed allowlist behavior is unchanged.
- Goal-tag task ordering behavior is unchanged.
- make quality and actionlint pass before the release tag is pushed.

## Verification Plan

1. Run focused tests for config repository tools and current task ordering.
2. Run make quality, make lint, and actionlint on a clean archive of HEAD.
