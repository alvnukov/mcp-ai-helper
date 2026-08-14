---
id: process-group-kill-must-not-hit-reused-pgid
title: Kill группы процессов не должен задевать реюзнутый pgid (дизайн)
status: todo
priority: medium
model_level: high
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - commands
    - process
    - design
    - unix
acceptance_criteria:
    - выбрана схема, исключающая kill(-pid) по реюзнутому pgid после reap'а лидера (например, kill до Wait или отслеживание потомков)
    - рассмотрены платформенные различия unix/windows
    - протестировано насколько возможно детерминистанно
created_at: "2026-08-14T21:18:50.510682Z"
updated_at: "2026-08-14T21:18:50.510682Z"
---

## Body

command/termination_unix.go:24 — killCommandProcessGroup вызывается после command.Run() (лидер уже reaped): kill(-pid) бьёт по тому, кто сейчас владеет pgid — при реюзе PID можно SIGKILL чужую группу. Design-heavy.

## Acceptance Criteria

- выбрана схема, исключающая kill(-pid) по реюзнутому pgid после reap'а лидера (например, kill до Wait или отслеживание потомков)
- рассмотрены платформенные различия unix/windows
- протестировано насколько возможно детерминистанно
