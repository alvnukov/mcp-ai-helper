---
id: homebrew-publish-must-stage-before-idempotency-check
title: Исправить публикацию новой и уже актуальной Homebrew formula
status: done
priority: critical
model_level: medium
task_type: bug
parent_id: release-v1-0-0-with-homebrew
tags:
    - release
    - homebrew
    - git
    - idempotency
    - regression
acceptance_criteria:
    - Новая untracked formula staged и коммитится в homebrew-tap
    - Byte-identical formula на rerun завершается успешно без нового commit
    - v1.0.0 formula опубликована и brew install сообщает mcp-ai-helper 1.0.0
    - Release workflow проходит actionlint
verification_plan:
    - actionlint .github/workflows/release.yml
    - Опубликовать Formula/mcp-ai-helper.rb из v1.0.0 release assets и проверить tap commit
    - brew install alvnukov/tap/mcp-ai-helper и mcp-ai-helper --version
created_at: "2026-08-01T17:32:38.097895Z"
updated_at: "2026-08-01T17:33:55.340526Z"
---

## Body

Tag workflow проверяет git diff до staging и поэтому принимает новый untracked Formula/mcp-ai-helper.rb за already current. Stage formula first, then use cached diff so first publish commits and exact rerun succeeds without ложной ошибки.

## Acceptance Criteria

- Новая untracked formula staged и коммитится в homebrew-tap
- Byte-identical formula на rerun завершается успешно без нового commit
- v1.0.0 formula опубликована и brew install сообщает mcp-ai-helper 1.0.0
- Release workflow проходит actionlint

## Verification Plan

1. actionlint .github/workflows/release.yml
2. Опубликовать Formula/mcp-ai-helper.rb из v1.0.0 release assets и проверить tap commit
3. brew install alvnukov/tap/mcp-ai-helper и mcp-ai-helper --version
