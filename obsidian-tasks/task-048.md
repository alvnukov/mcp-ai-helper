---
id: task-048
title: run_workflow git_commit_owned ignores top-level owned_files/commit_message
status: done
priority: high
model_level: high
tags:
    - issue
    - feedback
    - workflow
    - git
    - tool-contract
    - agent-friction
worktree_path: .worktrees/task-048
created_at: "2026-05-14T09:15:25.479358Z"
updated_at: "2026-05-14T11:46:09.788268Z"
---

## Body

Observed failure: calling run_workflow with top-level owned_files and commit_message, plus a git_commit_owned step with empty args, returned status ok but skipped commit with reason `no files to commit`. Immediately after, `git status --short` showed 14 modified owned files. Repro evidence: skipped workflow returned `no files to commit`; status check command_id `4e159542f106de96` showed modified files.

Expected behavior: either git_commit_owned should use the run_workflow top-level owned_files/commit_message as the public schema suggests, or fail validation with a clear error requiring step args.files and args.message.

Previous attempted fix in commit `60a6254` is incomplete and must be reworked.

Review finding to address:
- In `internal/pipeline/pipeline.go`, step-level `git_commit_owned` currently uses top-level `Commit.Files` only when both `step.args.files` and `changedSet` are empty. This is wrong. If a workflow has a `guarded_replace` changing `x.txt` and a `command` or `task_batch_upsert` changing `y.txt`, then `changedSet` is non-empty, top-level `owned_files` is ignored, and `y.txt` can remain uncommitted while the workflow still returns ok.

Required behavior:
- When `git_commit_owned` step has no explicit `args.files`, the top-level `WorkflowRequest.Commit.Files` / MCP `owned_files` must be honored. Do not silently narrow the commit to `changedSet` when top-level owned files were supplied.
- Choose a clear precedence and document it in code/tests. Recommended: step `args.files` wins; otherwise top-level `Commit.Files` wins; otherwise fall back to `changedSet`.
- Preserve top-level `commit_message` fallback for empty step message.
- Do not weaken git safety or stage all files.

Required regression test:
- Add a test where top-level files are `["x.txt", "y.txt"]`.
- `guarded_replace` changes `x.txt`.
- a `command` step changes `y.txt`.
- `git_commit_owned` step has empty args.
- after workflow, `git status --short` must be empty and the commit must include both files.

Suggested verification:
- `go test ./internal/pipeline -run 'TestRunWorkflowStepsCommitUsesTopLevelOwnedFiles|TestRunWorkflowStepsEditCheckCommit|TestCommit'`
- `go test ./internal/gitops -run 'TestCommitOwned'`
- `go test ./internal/mcp -run '^TestRunWorkflowSchemaIncludesWorkflowFields$'`

source_repo_path: <repo:mcp-ai-helper>
