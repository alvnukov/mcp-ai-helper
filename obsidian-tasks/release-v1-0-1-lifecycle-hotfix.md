---
id: release-v1-0-1-lifecycle-hotfix
title: Publish v1.0.1 workflow and command lifecycle hotfix
status: done
priority: critical
model_level: high
task_type: ci
tags:
    - release
    - v1.0.1
    - github-actions
    - homebrew
    - hotfix
    - reliability
acceptance_criteria:
    - Annotated tag v1.0.1 points to commit 5ebecea and is pushed without rewriting v1.0.0.
    - GitHub Release v1.0.1 contains all six platform archives and checksums.txt.
    - Homebrew formula resolves version 1.0.1 and brew upgrade/install yields mcp-ai-helper 1.0.1.
    - Release workflow completes successfully and remains idempotent.
verification_plan:
    - Inspect the tag-triggered GitHub Actions run and release assets through helper commands.
    - Run brew update/upgrade and verify mcp-ai-helper --version.
created_at: "2026-08-01T17:55:48.914088Z"
updated_at: "2026-08-01T18:01:47.724302Z"
---

## Body

Publish the verified workflow lifecycle and durable command coordinator fixes as immutable patch release v1.0.1. Tag must trigger GitHub Release and idempotently update the Homebrew tap; installed binary must report 1.0.1.

## Acceptance Criteria

- Annotated tag v1.0.1 points to commit 5ebecea and is pushed without rewriting v1.0.0.
- GitHub Release v1.0.1 contains all six platform archives and checksums.txt.
- Homebrew formula resolves version 1.0.1 and brew upgrade/install yields mcp-ai-helper 1.0.1.
- Release workflow completes successfully and remains idempotent.

## Verification Plan

1. Inspect the tag-triggered GitHub Actions run and release assets through helper commands.
2. Run brew update/upgrade and verify mcp-ai-helper --version.
