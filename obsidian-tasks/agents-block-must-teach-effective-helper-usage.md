---
id: agents-block-must-teach-effective-helper-usage
title: Блок в AGENTS.md должен учить модель эффективному использованию хелпера
status: done
priority: medium
model_level: medium
task_type: docs
tags:
    - instructions
    - agents-md
    - llm
    - usability
    - setup
created_at: "2026-08-21T11:23:23.865685Z"
updated_at: "2026-08-21T11:25:15.472891Z"
---

## Body

blockBody в internal/setup/instructions.go — единственный текст, который гарантированно грузится в каждую сессию обслуживаемого репо (guidance и скиллы модель читает только по своей инициативе). Сегодня блок отвечает на вопрос «стоит ли тянуться к этим инструментам», но не учит базовым паттернам, на которых модели ошибаются чаще всего: работа без task action=current, чтение целых файлов вместо search+read, rerun команд и sleep-поллинг вместо get/wait_seconds/filter/abort, выводы из вывода без проверки exit_code/truncated/failure_markers. Добавить в блок компактную секцию из четырёх паттернов, обновить doc-комментарий константы, добавить тест, пинирующий наличие паттернов (аналог защиты embeddedSkillDirs). Существующий текст блока не менять.
