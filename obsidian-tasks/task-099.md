---
id: task-099
title: Спроектировать durable workflow result/status contract
status: done
priority: high
model_level: medium
tags:
    - workflow
    - timeout
    - observability
    - design
worktree_path: .worktrees/task-099
created_at: "2026-05-14T09:15:25.487896Z"
updated_at: "2026-08-18T09:07:55.33088Z"
---

## Body

Контракт (реализуется в этом проходе): 1) run action=workflow принимает mcp_wait_seconds (default 10s, cap 25s — с запасом под ~30s клиентский таймаут); 2) workflow исполняется detached на background-context — таймаут/дисконнект MCP-запроса больше не отменяет шаги; 3) каждый запуск получает workflow_id и попадает в процесс-wide WorkflowRegistry (переживает пересоздание Runner'а при repo-local config; ёмкость 128 завершённых записей); 4) при укладке в бюджет — прежний конверт WorkflowResult; при превышении — {status:running, workflow_id, note}; 5) новый run action=workflow_status(workflow_id, wait_seconds?≤120) возвращает durable-запись: status, step_results, commit_result, changed_files, reason, error — без перезапуска workflow; 6) границы: персистентность на диск не входит (перезапуск сервера очищает реестр; пошаговые command_id остаются доступны через command action=get), отмена запущенного workflow не входит. Регрессионный тест: бюджет превышен, финальный результат доступен по id без перезапуска.
