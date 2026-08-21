---
id: command-vars-env-stdin-interface-spec
title: Спроектировать интерфейс vars/env/stdin для command run против quoting hell
status: done
priority: high
model_level: medium
task_type: design
parent_id: mcp-helper-llm-reliability-audit
tags:
    - commands
    - mcp
    - schema
    - quoting
    - vars
    - env
    - stdin
    - secrets
    - design
    - llm-ergonomics
acceptance_criteria:
    - 'Зафиксированы имена полей и JSON-схемы для env, vars, stdin/stdin_var на трёх поверхностях: command action=run, run action=pipeline, run action=workflow steps (включая command step)'
    - 'Определена семантика подстановки {{name}}: literal-escape для {{{{, подстановка после JSON-декодирования и до shell, без eval; перечислены поля, к которым применяется (command, stdin, содержимое file-параметров) и к которым нет (repo_path, путевые параметры)'
    - 'Определён контракт унификации секретов: secret_handles резолвятся в vars ({{GH_TOKEN}} и $GH_TOKEN эквивалентны), HELPER_SECRET_* остаётся back-compat алиасом, sanitize по-прежнему маскирует значения в outputs/records как [HELPER_SECRET:NAME]'
    - 'Определены правила приоритета при коллизии имён: env vs vars vs secret handles; неизвестный {{name}} без значения — fail-closed с teaching error'
    - 'Спецификация проверена против существующего кода: internal/config/config.go ResolveSecretEnv, internal/pipeline, internal/mcp/pipeline_tools.go'
    - 'В спеку включён план обновления скиллов: mcp-ai-helper-commands — секция «значения едут данными» (env/vars/stdin вместо квотинга и heredoc), mcp-ai-helper-edits/tasks — где уместно; сегодня ни один mcp-ai-helper-* скилл не документирует даже secret_handles'
    - Спек декомпозирован в implementation-подзадачи (schema + runtime + sanitize + tests + guidance docs + skill updates)
verification_plan:
    - Спека согласована с кодом ResolveSecretEnv/SecretHandles и тестами internal/pipeline/pipeline_test.go (кейсы HELPER_SECRET_*)
    - 'Контр-примеры прогнаны вручную: значение с '', ", $, \n, {{ проходит через env/vars/stdin без ручного квотинга'
    - 'Ревью спеки против внешнего отчёта: классы ошибок квотинга закрыты одним из трёх каналов'
created_at: "2026-08-21T09:58:44.725257Z"
updated_at: "2026-08-21T10:35:43.829508Z"
---

## Body

Проблема: значения (JSON, SQL, regex, сообщения, токены) едут текстом внутри shell-строки command, и модель вынуждена строить вложенный квотинг — по внешнему отчёту это 135 ssh-однострочников с тройным квотингом, heredoc-записи, PAT в TOK=. Принцип решения: значение едет в RPC как данные (JSON-поле), квотинг исчезает как класс.\n\nРешённое направление (фиксировать в спеке):\n1. env: map[string]string на command run / pipeline / workflow — значения инжектятся в окружение процесса; модель пишет $VAR.\n2. vars: map[string]string + подстановка {{name}} в command/stdin/содержимом file-параметров; подстановка после JSON-декодирования, до shell, литеральная, без eval.\n3. stdin / stdin_var: команда читает из stdin — замена heredoc.\n4. Унификация секретов: secret_handles резолвятся в vars; {{GH_TOKEN}} и $GH_TOKEN работают одинаково; HELPER_SECRET_* — back-compat.\n\nОткрытые вопросы спека: escape для literal {{; правило ссылки на env всегда "$VAR"; fail-closed на неизвестный {{name}}; применение подстановки (command, stdin, file contents) и запрет на repo_path; взаимодействие с sanitize ([HELPER_SECRET:X]).

## Acceptance Criteria

- Зафиксированы имена полей и JSON-схемы для env, vars, stdin/stdin_var на трёх поверхностях: command action=run, run action=pipeline, run action=workflow steps (включая command step)
- Определена семантика подстановки {{name}}: literal-escape для {{{{, подстановка после JSON-декодирования и до shell, без eval; перечислены поля, к которым применяется (command, stdin, содержимое file-параметров) и к которым нет (repo_path, путевые параметры)
- Определён контракт унификации секретов: secret_handles резолвятся в vars ({{GH_TOKEN}} и $GH_TOKEN эквивалентны), HELPER_SECRET_* остаётся back-compat алиасом, sanitize по-прежнему маскирует значения в outputs/records как [HELPER_SECRET:NAME]
- Определены правила приоритета при коллизии имён: env vs vars vs secret handles; неизвестный {{name}} без значения — fail-closed с teaching error
- Спецификация проверена против существующего кода: internal/config/config.go ResolveSecretEnv, internal/pipeline, internal/mcp/pipeline_tools.go
- В спеку включён план обновления скиллов: mcp-ai-helper-commands — секция «значения едут данными» (env/vars/stdin вместо квотинга и heredoc), mcp-ai-helper-edits/tasks — где уместно; сегодня ни один mcp-ai-helper-* скилл не документирует даже secret_handles
- Спек декомпозирован в implementation-подзадачи (schema + runtime + sanitize + tests + guidance docs + skill updates)

## Verification Plan

1. Спека согласована с кодом ResolveSecretEnv/SecretHandles и тестами internal/pipeline/pipeline_test.go (кейсы HELPER_SECRET_*)
2. Контр-примеры прогнаны вручную: значение с ', ", $, \n, {{ проходит через env/vars/stdin без ручного квотинга
3. Ревью спеки против внешнего отчёта: классы ошибок квотинга закрыты одним из трёх каналов
