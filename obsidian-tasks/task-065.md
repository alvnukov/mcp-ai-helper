---
id: task-065
title: Маскирование секретов в MCP-ответах: api_key, токены, пароли
status: done
priority: critical
tags:
  - security
  - secrets
  - config
  - mcp
worktree_path: .worktrees/task-065
created_at: 2026-05-14T09:15:25.481717Z
updated_at: 2026-05-14T09:15:25.481717Z
---

## Body

Проблема: секретные значения могут утекать в контекст LLM через error messages, HTTP-ответы, config вывод.

Требуется:
1. Единый механизм маскирования секретных полей в ошибках — обёртка, заменяющая значения на ***
2. Очистка HTTP-ошибок от заголовков Authorization
3. В config schema явно пометить секретные поля и гарантировать их непопадание в вывод
4. Тест на отсутствие секретов в error messages
5. Контракт: какие поля секретны, как обрабатываются
