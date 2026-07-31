# AGENTS.md for /Users/zol/src/mcp-ai-helper

Ты работаешь с этим репозиторием только через MCP-инструменты `mcp-ai-helper`.
Прямые shell/file/git операции вне MCP helper запрещены, пока пользователь явно не разрешит fallback.

## Самое строгое требование: выполнение команд только через слабого subagent

Основная модель не должна выполнять, ожидать, polling'ить, читать tail/result/status или иным образом сопровождать долгие команды и проверки сама.
Для запуска, ожидания и сбора результата команд обязательно делегируй минимальный контекст самому слабому доступному subagent.
Subagent получает только: repo_path, точную команду или command_id, разрешённые MCP helper tools, ожидаемый компактный результат и запрет на лишние действия.
Если слабому subagent недоступны нужные MCP helper tools (`run action=pipeline`, `command action=run`, `command action=get`, `command action=filter` или актуальный эквивалент), не выполняй команду основной моделью и не имитируй ожидание; верни surface mismatch/blocker.
Это правило приоритетнее всех остальных workflow/check/verification инструкций, включая требования запускать tests/gates/quality checks.

## Текущее состояние helper surface

Ежедневный surface — шесть action-dispatch tools. Каждый принимает `action`,
и список действий в схеме генерируется из самих handler'ов, так что схема не
может разойтись с тем, на что сервер отвечает.

| Tool | Actions |
| --- | --- |
| `file` | `read`, `read_many`, `list`, `search`, `snapshot` |
| `edit` | `replace`, `write` |
| `command` | `run`, `get`, `filter`, `list`, `abort`, `cleanup`, `health` |
| `git` | `status`, `diff`, `commit`; со слоем `git_advanced` также `log`, `log_diff`, `blame`, `stash_list`, `branch_list`, `remote_list`, `tag_list`, `prepare_task_worktree` |
| `task` | `current`, `get`, `list`, `search`, `upsert`, `set_status`, `batch_upsert`, `delete` |
| `run` | `pipeline`, `workflow`, `schema` |

Плюс `health`, `list_models`, `query_model`, `assistant_guidance`,
`server_setup_guidance`, `tool_manifest`, `language_profiles`, `language_detect`,
`plan_task_execution`, `task_packet`, `reasoning_patterns`, `config_*`.
За opt-in слоями: `task_graph`/`task_context`/`task_export` (`task_advanced`),
`web_*` (`web`), `issue_*` (`issues`), `lake_*` (`lake`), `feature_*`
(`config_advanced`), `jira_*`/`conf_*` (интеграции).

Не полагайся на этот список: вызови `tool_manifest` и работай по нему.
Если AGENTS/guidance требует неэкспонированный tool, сначала зафиксируй surface
mismatch и работай через текущий canonical surface, не имитируя отсутствующий
инструмент.

## Workflow contract caveat

`run action=schema` может описывать step field как `type`, но фактический
`run action=workflow` contract использует:

```json
{"id":"step-id","tool":"guarded_replace|command|git_commit_owned|task_batch_upsert|task_transition|git_prepare_task_worktree","args":{}}
```

Имена step'ов — не имена tools: `guarded_replace` и `git_commit_owned`
существуют только внутри `steps`.

Для guarded edits сначала `file action=read` / `file action=snapshot`, затем
`run action=workflow` с `tool: "guarded_replace"`.
Не используй JSON-строки внутри `steps`; передавай step objects.

## Работа с задачами

Перед выполнением repo-задачи читай `task action=current` и выбирай только задачу, соответствующую уровню текущей модели.
Для standard-level модели допустимы локальные implementation/test/docs задачи с уже заданным контрактом.
Strong-level research/design/security/concurrency задачи не выполнять младшей моделью.

Если текущая задача требует отсутствующего tool из roadmap, результат должен быть `blocked` или surface mismatch task, а не выдуманный вызов.

Жёстко запрещено редактировать задачи в обход task tools MCP helper. Не правь вручную `MCPAIHelperProject/ActiveTasks.lean`, `tasks/*.lean`, legacy task projection файлы, JSON-комментарии задач или любые task registry/source files ради изменения title/body/status/priority/tags/criteria/verification. Любое изменение задач делай только через `task_upsert`, `task_batch_upsert`, `task_set_status`, `task_delete` или другой явно task-facing MCP helper tool. Если task tool не поддерживает нужный id/поле/массовую операцию, остановись и верни surface mismatch/blocker; не обходи это file edits, scripts, guarded_replace или shell.

## Строгий стиль выполнения задач

Для любой repo-задачи обязательный протокол:
1. Сначала собрать полный минимально достаточный контекст: `task action=current`, релевантные `file action=read`/`file action=snapshot`, и только узкие `run action=pipeline`/`command action=get` выборки. Не строить исполняющий pipeline/workflow до понимания контракта, архитектуры, точек интеграции, тестовых паттернов и уже сделанных изменений.
2. После сбора контекста остановиться и кратко зафиксировать решение: выбранные задачи, почему они подходят текущей модели, что именно править, `owned_files`, какие файлы трогать нельзя, какие acceptance criteria закрываются, какой минимальный gate доказывает результат, и как задача будет закрыта полностью.
3. Только после этого собрать один самодостаточный `run action=pipeline` или `run action=workflow`, который делает весь оставшийся implementation path: минимальные правки, форматирование, релевантные проверки, перевод задачи в финальный статус и commit owned files только при отсутствии ошибок.
4. Для repo-задачи с изменениями финализация должна быть атомарной внутри одного `run action=workflow`: после успешных acceptance gates выполнить task-facing transition в финальный статус, затем в том же workflow выполнить `git_commit_owned`, включив в `owned_files` и изменённые рабочие файлы, и canonical task registry mutation. Запрещено сначала коммитить код, а потом отдельным шагом или отдельным commit фиксировать статус задачи.
5. Не ставить задачу в `done`, пока acceptance criteria, релевантный gate, task status transition и commit owned files реально не закрыты в одном workflow. Для repo-задачи с изменениями отсутствие такого единого коммита означает, что задача не сделана. Частичный зелёный тест, timeout, evidence-only analysis, skipped check, failed commit, отдельный post-hoc status commit или непроверенный MCP/tool-facing путь не являются основанием для `done`.
6. Если один pipeline/workflow не может безопасно закрыть задачу из-за surface mismatch, отсутствующего tool, противоречивого контракта или неподтверждённой архитектурной точки, не имитировать завершение. Вернуть compact blocker с тем, что подтверждено, что не сходится, и какая следующая развилка.
7. Если workflow упал или завис, не закрывать задачу. Сначала проверить фактическое состояние, отделить подтверждённое от неподтверждённого, и выбрать следующий минимальный шаг с новой гипотезой.

Запрещено начинать с большого исправляющего скрипта без предварительного контекста и решения. Запрещено закрывать задачу только потому, что часть тестов прошла, если acceptance criteria шире фактически проверенного поведения.

## Проверки

После изменения кода запускать только минимально релевантные проверки через `run action=pipeline` или `run action=workflow` command step.

### Baseline Go quality gate (обязательно перед коммитом)

```
make quality        # go vet + go test -race -short + go build (~40s)
make test-pkg PKG=./internal/gitops/...  # targeted tests
make lint           # golangci-lint (если установлен)
```

Или вручную:
```
go vet ./...
go test ./... -count=1 -race -short -timeout=300s
go build ./...
```

`-short` пропускает тесты, которые собирают и запускают настоящий Lean workspace
(`internal/mcp`, `internal/lake`). Они остаются в `make test` и в CI. Полный
прогон перед сменой контракта task registry:
```
make test           # всё, включая Lean, ~10 минут
```

Почему так долго: каждая мутация task registry делает `ResetAfterCall`, то есть
убивает общий `lake serve` и следующий вызов платит за холодный старт. Это
свойство дизайна, а не тестов.

### Targeted проверки

Для Go-кода baseline gate: targeted `go test`, затем более широкий прогон только при конкретном риске регрессии.
Для Python-кода baseline gate: `ruff check`, `mypy --strict`, targeted tests.

## Обязательный перезапуск Codex

После изменения этого файла пользователь должен перезапустить Codex/сессию, чтобы новые AGENTS.md instructions были перечитаны и начали действовать стабильно.
До перезапуска агент обязан явно сказать: `Нужен перезапуск Codex после обновления AGENTS.md`.
