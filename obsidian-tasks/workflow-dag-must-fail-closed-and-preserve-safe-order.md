---
id: workflow-dag-must-fail-closed-and-preserve-safe-order
title: Сделать workflow DAG fail-closed и исключить проверку старого кода
status: done
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - dag
    - concurrency
    - safety
    - llm
created_at: "2026-08-01T11:19:08.030851Z"
updated_at: "2026-08-22T16:17:28.923343Z"
---

## Body

Validate workflow step graphs before execution. Reject duplicate ids, unknown depends_on/condition references, and cycles instead of running all steps concurrently. Ensure the canonical edit-check-transition-commit example encodes explicit dependencies so checks cannot race edits and commits cannot race task finalization.

Evidence 2026-08-14 (две живые репродукции в основной сессии, оба шаг-набора без depends_on):
1. Workflow правки server.go в три guarded_replace шага без depends_on: часть шагов параллельной волны молча потерялась (import и вызов не применились), gate упал на 'undefined: debug', а task_transition 'done' успел отработать до гейта. Потребовался откат статуса и повторный workflow.
2. Та же сессия, второй случай: два шага на один файл в одной волне — шаг drop-errors-import потерян, провал виден только по 'imported and not used' на gate.
3. Провалившиеся шаги НЕ появляются в step_results вовсе (нет ни failed-записи, ни reason) — наблюдаемость нулевая: caller видит только чужой след провала (упавший gate), а не сам потерянный шаг. Это отдельное требование к фиксу: каждый заявленный шаг обязан иметь запись в результате (ok/conflict/failed/skipped).
