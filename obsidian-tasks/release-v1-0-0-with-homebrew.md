---
id: release-v1-0-0-with-homebrew
title: Выпустить mcp-ai-helper v1.0.0 через tag-driven GitHub Release и Homebrew
status: done
priority: critical
model_level: very_high
task_type: release
tags:
    - release
    - github-actions
    - homebrew
    - semver
    - distribution
acceptance_criteria:
    - Go module и публичные import paths соответствуют github.com/alvnukov/mcp-ai-helper
    - mcp-ai-helper --version сообщает версию, внедряемую release build из тега
    - Push тега v1.0.0 собирает проверенные Linux/macOS/Windows amd64/arm64 assets и публикует GitHub Release с checksums
    - После успешного GitHub Release workflow обновляет Formula/mcp-ai-helper.rb в alvnukov/homebrew-tap и отсутствие HOMEBREW_TAP_TOKEN завершает job ошибкой
    - README и RELEASE.md документируют Homebrew install, tag/version contract, секрет и воспроизводимый выпуск
    - Focused tests, make quality, make lint и actionlint проходят; commit содержит только owned release files и canonical import rewrite
    - main и тег v1.0.0 отправлены в origin; tag-triggered workflow завершился успешно
verification_plan:
    - Проверить unit test версии и canonical import path через go list
    - Запустить actionlint для CI/release workflow
    - Запустить make quality и make lint
    - После push тега проверить GitHub Actions, наличие GitHub Release assets/checksums и формулу Homebrew tap
created_at: "2026-08-01T17:07:33.076846Z"
updated_at: "2026-08-01T17:33:55.354061Z"
---

## Body

Подготовить первый стабильный релиз по проверенному паттерну happ: canonical module identity, versioned cross-platform artifacts, GitHub Release по тегу v1.0.0 и обязательное обновление alvnukov/homebrew-tap. Не включать посторонние пользовательские изменения.

## Acceptance Criteria

- Go module и публичные import paths соответствуют github.com/alvnukov/mcp-ai-helper
- mcp-ai-helper --version сообщает версию, внедряемую release build из тега
- Push тега v1.0.0 собирает проверенные Linux/macOS/Windows amd64/arm64 assets и публикует GitHub Release с checksums
- После успешного GitHub Release workflow обновляет Formula/mcp-ai-helper.rb в alvnukov/homebrew-tap и отсутствие HOMEBREW_TAP_TOKEN завершает job ошибкой
- README и RELEASE.md документируют Homebrew install, tag/version contract, секрет и воспроизводимый выпуск
- Focused tests, make quality, make lint и actionlint проходят; commit содержит только owned release files и canonical import rewrite
- main и тег v1.0.0 отправлены в origin; tag-triggered workflow завершился успешно

## Verification Plan

1. Проверить unit test версии и canonical import path через go list
2. Запустить actionlint для CI/release workflow
3. Запустить make quality и make lint
4. После push тега проверить GitHub Actions, наличие GitHub Release assets/checksums и формулу Homebrew tap
