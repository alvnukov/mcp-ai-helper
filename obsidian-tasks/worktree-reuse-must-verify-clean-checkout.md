---
id: worktree-reuse-must-verify-clean-checkout
title: Reuse worktree должен проверять чистоту и принадлежность
status: done
priority: medium
model_level: medium
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - gitops
    - worktree
    - safety
acceptance_criteria:
    - переиспользование .worktrees/<id> требует чистый status --porcelain и подтверждение, что это linked worktree этого репо
    - грязный/чужой чекаут даёт ошибку, а не молчаливое «ok»
    - тест на грязный reuse
created_at: "2026-08-14T21:18:50.506223Z"
updated_at: "2026-08-14T21:49:59.93799Z"
---

## Body

gitops/git.go:75-86 — reuse возвращает ok без проверки чистоты и принадлежности: любой чекаут на нужной ветке под .worktrees/<id> принимается со своим грязным состоянием.

## Acceptance Criteria

- переиспользование .worktrees/<id> требует чистый status --porcelain и подтверждение, что это linked worktree этого репо
- грязный/чужой чекаут даёт ошибку, а не молчаливое «ok»
- тест на грязный reuse
