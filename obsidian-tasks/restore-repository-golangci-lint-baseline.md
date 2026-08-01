---
id: restore-repository-golangci-lint-baseline
title: Восстановить зелёный repository-wide golangci-lint baseline
status: done
priority: high
model_level: high
task_type: chore
tags:
    - quality
    - lint
    - tooling
    - baseline
acceptance_criteria:
    - make lint passes repository-wide with the compatible system golangci-lint.
    - No linter is disabled and no blanket exclusions or generated suppressions are introduced merely to obtain green status.
    - Existing findings are fixed in reviewable, logically grouped changes.
    - Normal feature and bug-fix workflows can complete the mandatory lint gate.
verification_plan:
    - Run make lint.
    - Run make quality.
    - Confirm golangci-lint is built with a Go version at least as new as the module target.
created_at: "2026-08-01T12:01:13.192838Z"
updated_at: "2026-08-01T14:41:39.547518Z"
---

## Body

The now-compatible system golangci-lint exposes 349 repository-wide findings. Resolve or explicitly baseline the existing debt without weakening linters, disabling checks, or mixing the cleanup into unrelated product fixes. This task blocks strict full-gate commits even when a patch has zero new lint findings.

## Acceptance Criteria

- make lint passes repository-wide with the compatible system golangci-lint.
- No linter is disabled and no blanket exclusions or generated suppressions are introduced merely to obtain green status.
- Existing findings are fixed in reviewable, logically grouped changes.
- Normal feature and bug-fix workflows can complete the mandatory lint gate.

## Verification Plan

1. Run make lint.
2. Run make quality.
3. Confirm golangci-lint is built with a Go version at least as new as the module target.
