# mcp-ai-helper

Следуй assistant_guidance (mcp__mcp-ai-helper__assistant_guidance) для всех process rules.
Этот файл — только project-specific.

## Languages & quality gates
- Go: targeted `go test`, broader only on regression risk
- Python: `ruff check` + `mypy --strict` + targeted tests
- Lake/Lean: `lake build` or `lake check`

## Tool surface
Используй `assistant_guidance` + `run action=schema` + `language_profiles` для актуального surface.
Не полагайся на захардкоженные списки tools — они устаревают.

## Workflow contract
`run action=workflow` steps: `{"id":"...","tool":"guarded_replace|write_file|command|git_commit_owned|task_batch_upsert|task_transition|git_prepare_task_worktree","args":{}}`
Для guarded_replace: `file action=read` → `file action=snapshot` → guarded_replace с expected_hash.
Пробы только на чтение батчить в один workflow с `on_failure: "continue"`, а не по одной команде за ход.

## Project constraint
Это сам проект helper. Изменения guidance/tools/schemas влияют на все downstream repos.
Минимальные изменения. Репозиторий под git — бэкапы файлов не делать,
вместо них промежуточные коммиты на рабочей ветке после каждого проверенного шага.

## После изменения этого файла
Перезапусти Codex сессию.
