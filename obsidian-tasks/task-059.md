---
id: task-059
title: Добавить управление feature flags helper через MCP-инструменты
status: done
priority: high
model_level: high
task_type: feature
tags:
  - features
  - configuration
  - mcp-tools
  - repo-state
  - global-state
  - safety
  - gitignore
branch: feature/task-059
worktree_path: .worktrees/task-059
created_at: 2026-05-14T09:15:25.481973Z
updated_at: 2026-05-14T09:15:25.481973Z
---

## Body

Нужно сделать включение, выключение и просмотр feature flags mcp-ai-helper через сам helper в локальном репозитории, без ручного редактирования registry/config файлов агентом или пользователем.

Проблема:
Сейчас экспериментальные или условные возможности helper нельзя безопасно переключать из canonical MCP surface. Это вынуждает либо править файлы напрямую, либо держать фичи включёнными/выключенными неявно, что плохо сочетается с MCP-only workflow, воспроизводимостью и auditability.

Цель:
Сделать feature flags first-class механизмом, доступным через MCP tools helper, с явным состоянием, описанием, defaults, источником значения и безопасными ограничениями. Нужны оба уровня управления: глобальный user/machine-level список enable/disable и локальный repo-level override.

Принятое архитектурное решение:
- feature registry живёт в коде helper: id, title, description, default, stability/risk, optional preconditions/conflicts;
- storage хранит только overrides относительно defaults, а не полный снимок registry;
- resolution order для обычных flags: code default < global override < repo-local override;
- если фича явно указана в локальном repo config как enabled или disabled, это имеет приоритет над глобальным enable/disable списком;
- global config нужен для user/machine-level defaults: включить или выключить фичу по умолчанию во всех локальных репозиториях;
- repo-local config нужен для исключений конкретного репозитория;
- первым выбором для repo-local state должен быть существующий typed repo-scoped config/state mechanism, если он уже есть;
- первым выбором для global state должен быть существующий typed helper user/machine-level config mechanism, если он уже есть;
- если локального helper config/state файла для repo ещё нет, helper должен создать его молча при первой записи repo override;
- созданный локальный config/state файл должен быть автоматически защищён от случайного коммита: helper должен добавить точную запись в .gitignore или использовать уже существующую ignored helper-state директорию;
- создание config/state и обновление .gitignore должны быть идемпотентными;
- Lean task registry не использовать как storage для feature flags;
- hard security lock, если он понадобится, должен быть отдельной policy layer, а не путаться с обычным global enable/disable override.

Ожидаемая функциональность:
- tool для просмотра доступных фич и effective state в заданном repo_path;
- tool для получения подробностей по одной фиче: описание, code default, global override, repo override, effective value, source, stability/risk notes;
- tool для включения/выключения фичи локально в репозитории;
- tool для сброса локального repo override, чтобы снова применялся global/code default;
- tool для включения/выключения фичи глобально на user/machine-level;
- tool для сброса global override, чтобы снова применялся code default;
- защита от неизвестных feature id: fail closed с понятной ошибкой;
- защита от небезопасных переключений, если фича требует preconditions или несовместима с текущим состоянием;
- машинно-читаемый output без длинных логов;
- минимальный audit trail для изменений: scope global/repo, кто/когда/что изменил, предыдущее и новое значение, причина если передана.

Архитектурные ограничения:
- не добавлять обходной путь через прямые file edits для задач/registry/config;
- не смешивать feature flags с task status mutations;
- не делать ad-hoc string parsing для structured config, если уже есть canonical storage/parser;
- не включать risky/experimental фичи по умолчанию без явного решения;
- не ломать существующий helper surface и backward compatibility;
- не создавать dynamic feature ids из пользовательского ввода;
- не считать global disable абсолютным запретом, если repo-local override явно включает фичу; абсолютный запрет должен быть отдельным lock/policy mechanism.

Интеграционные точки, которые нужно проверить перед реализацией:
- где сейчас хранятся repo-scoped settings/state;
- где сейчас хранятся helper-level user/machine settings/state;
- есть ли уже ignored helper-owned local state directory;
- как устроена регистрация MCP tools;
- есть ли уже config abstraction, которую можно расширить;
- как helper возвращает typed errors и compact diagnostics;
- как command/workflow tools читают capability/config state;
- как безопасно и идемпотентно добавить точную gitignore-запись, если локальный state файл создаётся впервые.

Предлагаемый MVP:
1. Ввести registry известных feature flags в коде: id, title, description, default, stability, optional preconditions.
2. Ввести persisted overrides на двух scope: global user/machine-level и repo-local.
3. Реализовать deterministic resolution: code default, затем global override, затем repo-local override как highest priority.
4. При первой записи repo override создать локальный helper config/state файл, если он отсутствует, и гарантировать, что он ignored в git.
5. Добавить MCP tools: feature_list, feature_get, feature_enable, feature_disable, feature_reset. Tools должны принимать scope: global или repo, где repo требует repo_path.
6. Добавить targeted regression tests на unknown flag, default/global/repo resolution, repo override precedence over global, reset semantics, repo isolation, persisted override, first-write config creation и gitignore idempotency.

Не-цели первой версии:
- remote rollout management;
- multi-user permission model;
- UI;
- динамическая загрузка фич из непроверенных файлов;
- автоматическое включение фич по модели/стоимости/эвристикам;
- hard security locks поверх обычных overrides, кроме явного design decision отдельной задачей.
