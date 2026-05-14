---
id: task-001
title: 'task_current fails after successful helper rebuild: unknown executable task_registry_export'
status: done
priority: high
tags:
    - issue
    - feedback
    - task-tools
    - rebuild
    - downstream-blocker
worktree_path: .worktrees/task-001
created_at: "2026-05-14T09:15:25.482116Z"
updated_at: "2026-05-14T11:46:09.785933Z"
---

## Body

While working in downstream repo <repo:vkusvill-mcp>, assistant_guidance requires Phase 1 to start with task_current and mandates MCP-only repo operations. After dev_rebuild_server succeeded (`rebuilt and restarted child in 1.23s`), task_current still failed with: `Lean task exporter failed: error: unknown executable task_registry_export`.

Expected behavior: after successful rebuild/restart, task_current should either work or report a concrete setup/config remediation path for the missing task_registry_export executable. If the exporter is optional or repo-specific, assistant_guidance should not make task_current mandatory without a fallback.

Impact: downstream work is blocked from following the required helper protocol; read_file/run_workflow are available, but task status workflow cannot be used safely.

source_repo_path: <repo:vkusvill-mcp>
