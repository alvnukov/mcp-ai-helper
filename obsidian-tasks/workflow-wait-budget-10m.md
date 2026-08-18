---
id: workflow-wait-budget-10m
title: Поднять бюджет ожидания workflow до 10 минут
status: done
priority: medium
model_level: low
task_type: chore
created_at: "2026-08-18T09:18:22.069339Z"
updated_at: "2026-08-18T09:21:10.460906Z"
---

## Body

Поднять бюджет ожидания run action=workflow и workflow_status до 10 минут: defaultWorkflowWaitSeconds/maxWorkflowWaitSeconds 10/25 → 600/600, клэмп wait_seconds в workflow_status 120 → 600, обновить описания тула и тест клэмпа. Обоснование: исполнение detached (фикс из 0c96047), поэтому длинное ожидание безопасно — даже при обрыве клиентского вызова workflow продолжает исполняться и остаётся доступным по workflow_id; бюджет теперь лишь ограничивает блокировку вызова.
