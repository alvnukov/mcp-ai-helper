---
id: install-mcp-ai-helper-llm-skills
title: Установить полный набор навыков безопасной работы с mcp-ai-helper
status: done
priority: critical
model_level: high
task_type: feature
parent_id: mcp-helper-llm-reliability-audit
tags:
    - skills
    - setup
    - codex
    - llm
    - workflow
    - reliability
acceptance_criteria:
    - Setup устанавливает skills для Claude Code, Codex и OpenCode в документированные каталоги и remove удаляет только helper-owned skill files.
    - Skills покрывают discovery/surface mismatch, выбор и атомарное закрытие задач, guarded edits/owned commits, bounded command output и durable polling без повторного запуска.
    - Каждый skill имеет валидный SKILL.md и agents/openai.yaml с согласованными именем и default prompt.
    - Focused setup tests, make quality и make lint проходят.
verification_plan:
    - go test ./internal/setup -count=1 -race -timeout=120s
    - make quality
    - make lint
created_at: "2026-08-01T16:30:55.805119Z"
updated_at: "2026-08-01T16:48:35.938038Z"
---

## Body

Разработать компактные Agent Skills, закрывающие discovery/task workflow, безопасное редактирование и lifecycle команд; устанавливать и удалять их идемпотентно для Claude Code, Codex и OpenCode вместе с helper.

## Acceptance Criteria

- Setup устанавливает skills для Claude Code, Codex и OpenCode в документированные каталоги и remove удаляет только helper-owned skill files.
- Skills покрывают discovery/surface mismatch, выбор и атомарное закрытие задач, guarded edits/owned commits, bounded command output и durable polling без повторного запуска.
- Каждый skill имеет валидный SKILL.md и agents/openai.yaml с согласованными именем и default prompt.
- Focused setup tests, make quality и make lint проходят.

## Verification Plan

1. go test ./internal/setup -count=1 -race -timeout=120s
2. make quality
3. make lint
