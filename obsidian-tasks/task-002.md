---
id: task-002
title: Provide bootstrap or repair path when Lean task registry exporter is missing
status: done
priority: medium
tags:
    - issue
    - feedback
    - setup
    - task-tools
    - lean-registry
    - downstream-blocker
worktree_path: .worktrees/task-002
created_at: "2026-05-14T09:15:25.482233Z"
updated_at: "2026-05-14T11:46:09.787344Z"
---

## Body

After the fix for task-001, dev_rebuild_server succeeds and task_current now returns a clearer diagnostic in downstream repo <repo:vkusvill-mcp>:

`Lean task registry exporter is not configured: found MCPAIHelperProject/ActiveTasks.lean but missing MCPAIHelperProject/TaskRegistryExport.lean; add the Lean registry exporter module and declare the "task_registry_export" executable in the Lake config, then rebuild/restart the helper`

This is an improvement over the previous `unknown executable task_registry_export`, but it still leaves the assistant blocked from following assistant_guidance. The helper can detect the exact missing files/config, so it should provide an actionable bootstrap/repair tool or a precise guided command/workflow to create MCPAIHelperProject/TaskRegistryExport.lean and update Lake config safely.

Expected behavior: when a repo has MCPAIHelperProject/ActiveTasks.lean but lacks the exporter module/executable, task_current/server_setup_guidance should expose a concrete remediation path, ideally an MCP tool that performs the guarded setup or a copy-paste minimal patch with verification steps.

Impact: downstream repos with partial Lean task registry setup remain unable to use mandatory task_current/task workflow even though the helper can identify the missing pieces.

source_repo_path: <repo:vkusvill-mcp>
