---
id: skills-and-block-apply-authoring-best-practices
title: Переписать скиллы и блок AGENTS.md по best practices авторинга
status: done
priority: medium
model_level: medium
task_type: docs
tags:
    - skills
    - instructions
    - agents-md
    - llm
    - usability
    - best-practices
created_at: "2026-08-21T11:31:33.37532Z"
updated_at: "2026-08-21T11:35:05.52848Z"
---

## Body

По итогам исследования (Anthropic skill authoring best practices, Anthropic engineering blog, спецификация agents.md, локальный writing-for-agents): переписать 4 из 5 скиллов (tasks, edits, commands, web; surface без изменений) — убрать дублированный ритуал вызова guidance/manifest, заменить чистые негации позитивными формулировками с сохранением жёстких guardrail'ов, добавить секцию Values as data (env/vars/stdin_var/secret_handles, fail-closed {{NAME}}) в commands и Feedback intake (issue_add/issue_accept) в tasks, заострить descriptions (front-load, один триггер на ветку). В блоке blockBody: заточить триггер guidance («before the first helper call») и снять финальный дубль запрета fallback (45→43 строки). Контракты тестов: workflowSteps в tasks, snake_case в секции Closing a task, валидные action=-упоминания, блок ≤45 строк, пиннированные иглы паттернов.
