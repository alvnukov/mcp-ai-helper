---
id: task-118
title: Добавить bounded read_files MCP tool для чтения нескольких малых файлов одним вызовом
status: done
priority: medium
model_level: medium
task_type: feature
tags:
  - fileops
  - mcp
  - read
  - batch
  - bounded-output
  - implementation
  - tests
branch: feature/task-118
worktree_path: .worktrees/task-118
acceptance_criteria:
  - `read_files` is registered with an accurate MCP input schema: required `repo_path` and required string-array `paths`.
  - Empty `paths` returns a compact tool error and does not attempt reads.
  - The tool enforces bounded output: max 8 paths, max 64 KiB per file, max 128 KiB total returned file bytes.
  - Response preserves input order and returns per-file structured results with path/relative_path, hash, size, exists, lines, error, truncated/omitted_reason where applicable.
  - Missing or unreadable files are reported per file and do not fail the whole call.
  - Existing `read_file` behavior and tests remain unchanged.
  - No globbing, recursive reads, caching, summarization, or write behavior is introduced.
verification_plan:
  - Run targeted Go tests for fileops/MCP read_files behavior only.
  - Test two small valid files succeed in one call and preserve input order.
  - Test one present file plus one missing file returns mixed per-file results without failing the whole call.
  - Test empty paths returns a tool error.
  - Test path count and byte bounds produce compact bounded/truncated metadata.
  - Test existing read_file path still works or keep its existing focused test passing.
created_at: 2026-05-14T09:15:25.490238Z
updated_at: 2026-05-14T09:15:25.490238Z
---

## Body

Implement a small, bounded `read_files` MCP tool for the common context-gathering case where an LLM needs several small repo-relative files at once.

Goal:
- Reduce repeated MCP round trips and wrapper metadata for related small-file reads.
- Do not optimize for large files; large or precise reads must continue to use `read_file` with offset/limit.

Input:
- `repo_path` string, required.
- `paths` array of strings, required.

Behavior:
- Reject an empty `paths` array with a compact tool error.
- Enforce conservative hard bounds before/while reading:
  - maximum number of paths: 8;
  - maximum bytes per file: 64 KiB;
  - maximum total returned file bytes: 128 KiB.
- Preserve input order in the response.
- Reuse `fileops.ReadFileContentInRepo` per path so existing repo-relative, symlink and escape protections remain unchanged.
- Per-file read errors must not fail the whole call. A bad or missing file returns a result item with `exists=false` where applicable and `error` set.
- If a file exists but exceeds per-file or total byte limits, return metadata plus `truncated=true`, `omitted_reason`, and no or partial lines according to the simplest safe implementation.
- Keep existing `read_file` behavior unchanged.

Output:
Return a structured object with:
- `files`: array of per-file results.
- `total_files`, `returned_files`, `returned_bytes`.
- optional `truncated` or `omitted` metadata when limits affect output.

Each file result must include:
- `path` or `relative_path` matching the requested repo-relative path;
- `hash`, `size`, `exists` when available;
- `lines` with line numbers when content is returned;
- `error` for per-file failures;
- `truncated` and `omitted_reason` when content is not fully returned because of bounds.

Scope:
- Add tool registration in `internal/mcp/file_tools.go`.
- Add only minimal helper types/functions in `internal/fileops` if this keeps the handler simple.
- Do not add broad directory reads, globbing, recursive search, write operations, caching, or model summarization.

## Acceptance Criteria

- `read_files` is registered with an accurate MCP input schema: required `repo_path` and required string-array `paths`.
- Empty `paths` returns a compact tool error and does not attempt reads.
- The tool enforces bounded output: max 8 paths, max 64 KiB per file, max 128 KiB total returned file bytes.
- Response preserves input order and returns per-file structured results with path/relative_path, hash, size, exists, lines, error, truncated/omitted_reason where applicable.
- Missing or unreadable files are reported per file and do not fail the whole call.
- Existing `read_file` behavior and tests remain unchanged.
- No globbing, recursive reads, caching, summarization, or write behavior is introduced.

## Verification Plan

1. Run targeted Go tests for fileops/MCP read_files behavior only.
2. Test two small valid files succeed in one call and preserve input order.
3. Test one present file plus one missing file returns mixed per-file results without failing the whole call.
4. Test empty paths returns a tool error.
5. Test path count and byte bounds produce compact bounded/truncated metadata.
6. Test existing read_file path still works or keep its existing focused test passing.
