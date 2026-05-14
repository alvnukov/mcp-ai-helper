---
id: task-067
title: Добавить браузерный просмотр и редактирование задач через Lake serve
status: done
priority: high
model_level: high
task_type: feature
tags:
  - tasks
  - ui
  - browser
  - lake-serve
  - lean-registry
  - safety
branch: feature/task-067
worktree_path: .worktrees/task-067
created_at: 2026-05-14T09:15:25.48234Z
updated_at: 2026-05-14T09:15:25.48234Z
---

## Body

Нужно добавить browser UI для просмотра и редактирования задач mcp-ai-helper.

Ключевое архитектурное требование: браузерный UI не должен иметь отдельный task-store, не должен парсить или редактировать Lean registry source напрямую и не должен обходить canonical task tools. Backend UI должен работать с тем же Lean/Lake task layer, preferably through `lake --serve` / canonical Lean-owned command surface, который используется MCP task tools (`task_current`, `task_get`, `task_upsert`, `task_batch_upsert`, `task_set_status`). Если для Go/browser integration существуют готовые библиотеки или стабильный protocol client для `lake --serve`, сначала проверить их и использовать вместо ad-hoc протокола.

Проблема:
Сейчас задачи доступны агентам через MCP tools, но человеку неудобно быстро просматривать backlog, читать acceptance criteria и безопасно редактировать задачи. Ручное редактирование `MCPAIHelperProject/ActiveTasks.lean` запрещено и небезопасно, поэтому UI должен быть тонким клиентом поверх canonical task mutation path.

MVP scope:
1. Read-only browser view: список задач, фильтры по status/priority/tags/query, карточка задачи с title/body/status/priority/tags/model_level/acceptance_criteria/verification_plan/diagnostics/source.
2. Safe edit view: создание задачи, редактирование title/body/priority/tags/model_level/acceptance_criteria/verification_plan и status transition через canonical task API.
3. Backend adapter: один task service, общий с MCP task tools или использующий тот же Lean/Lake serve layer без дублирования логики и без regex/string mutation Lean files.
4. Diagnostics: exporter/serve/validation errors показываются пользователю компактно и machine-readably; partial success запрещён.
5. Safety: неизвестные поля/status/task id fail closed; concurrent edit должен обнаруживаться через revision/hash/updated_at или другой guard, а не silently overwrite.

Integration points to investigate before implementation:
- какой именно protocol/API доступен у `lake --serve` для вызова task commands;
- есть ли Go libraries или stable client abstractions для Lake/Lean server protocol, которые можно использовать безопасно;
- где сейчас реализованы task_current/task_get/task_upsert/task_batch_upsert/task_set_status и можно ли вынести общий service layer для MCP и browser backend;
- нужен ли отдельный HTTP server/route внутри helper или dev-only browser endpoint;
- как UI должен получать repo_path и как ограничить его локальным trusted usage;
- как запускать/останавливать Lake serve process, timeouts и compact diagnostics;
- как гарантировать, что task mutation после UI edit проходит Lean validation и не оставляет partial registry changes.

Non-goals for first version:
- multi-user auth/permissions;
- remote deployment;
- collaborative realtime editing;
- direct editing of Lean source;
- replacing MCP task tools;
- broad project management features outside task registry.

Acceptance criteria:
- UI can list tasks from the canonical Lean-backed registry and clearly shows source/diagnostics.
- UI can open a task and render all first-class fields exposed by the task payload.
- UI can create/update a task and transition status only through the canonical Lean/Lake task mutation path.
- No production code parses or mutates `MCPAIHelperProject/ActiveTasks.lean` directly for UI behavior.
- Backend and MCP task tools share the same task service/adapter or demonstrably call the same Lake-owned command surface.
- Failed validation/exporter/serve calls return compact diagnostics and do not leave partial task changes.
- Concurrent/stale edit attempts are rejected or explicitly guarded.
- Focused tests cover read list/get, create/update/status transition, validation failure, stale edit guard, and no direct registry source mutation path.

Verification plan:
- Run focused Go tests for the task service/browser backend package.
- Run focused UI/backend smoke test for list/get/update if a browser UI framework is added.
- Run `lake build` after task mutation path tests that touch the Lean registry/exporter contract.
- Run a minimal manual/browser smoke only after the local server starts successfully.
