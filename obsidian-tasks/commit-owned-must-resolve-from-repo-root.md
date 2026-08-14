---
id: commit-owned-must-resolve-from-repo-root
title: CommitOwned должен резолвиться от корня репо
status: done
priority: medium
model_level: low
task_type: bug
parent_id: review-2026-08-helper-hardening
tags:
    - gitops
    - paths
acceptance_criteria:
    - CommitOwned нормализует repo к --show-toplevel (как PrepareTaskWorktree), diff --cached даёт toplevel-относительные пути
    - из подкаталога коммит owned files не даёт ложного конфликта
    - тест из подкаталога
created_at: "2026-08-14T21:18:50.505043Z"
updated_at: "2026-08-14T21:35:45.875801Z"
---

## Body

gitops/git.go:120-126 — CommitOwned работает от filepath.Abs(repoInput): из подкаталога diff --cached --name-only даёт cwd-относительные пути, которые не совпадают с toplevel-относительным owned-набором → ложный conflict.

## Acceptance Criteria

- CommitOwned нормализует repo к --show-toplevel (как PrepareTaskWorktree), diff --cached даёт toplevel-относительные пути
- из подкаталога коммит owned files не даёт ложного конфликта
- тест из подкаталога
