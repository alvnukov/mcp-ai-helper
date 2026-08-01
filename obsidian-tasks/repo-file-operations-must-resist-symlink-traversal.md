---
id: repo-file-operations-must-resist-symlink-traversal
title: Защитить repo file operations от symlink traversal
status: done
priority: critical
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - security
    - filesystem
    - symlink
    - path-traversal
    - gosec
acceptance_criteria:
    - Repo-scoped reads, writes, creates, deletes, renames, and directory creation cannot escape through symlinked path components.
    - Obsidian task note operations remain confined to the configured registry root.
    - Regression tests cover symlinked files and parent directories targeting paths outside the root.
    - G703 findings are resolved by enforceable path invariants rather than blanket nosec suppressions.
verification_plan:
    - Run focused fileops and Obsidian backend symlink traversal tests.
    - Run gosec/golangci-lint for the affected packages.
    - Run make quality.
created_at: "2026-08-01T12:19:07.941763Z"
updated_at: "2026-08-01T14:41:39.547979Z"
---

## Body

Lexical repoRelativePath/cleanPath validation does not prevent a symlink inside an allowed repository or Obsidian registry from redirecting reads, writes, renames, or directory creation outside the intended root. Replace path-based mutations with a root-scoped/no-follow design (for example os.Root/openat-style operations) and remove existing G703 suppressions only after the invariant is enforced.

## Acceptance Criteria

- Repo-scoped reads, writes, creates, deletes, renames, and directory creation cannot escape through symlinked path components.
- Obsidian task note operations remain confined to the configured registry root.
- Regression tests cover symlinked files and parent directories targeting paths outside the root.
- G703 findings are resolved by enforceable path invariants rather than blanket nosec suppressions.

## Verification Plan

1. Run focused fileops and Obsidian backend symlink traversal tests.
2. Run gosec/golangci-lint for the affected packages.
3. Run make quality.
