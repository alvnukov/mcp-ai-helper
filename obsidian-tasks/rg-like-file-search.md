---
id: rg-like-file-search
title: Add rg-like search options to file action=search
status: done
priority: high
model_level: medium
task_type: feature
tags:
    - fileops
    - search
    - rg
    - mcp
    - implementation
acceptance_criteria:
    - 'Literal substring default preserved: existing fileops and mcp search tests pass unchanged'
    - regex, ignore_case, smart_case, glob, glob_exclude, context_before, context_after, files_only, invert all work at fileops seam and via file action=search
    - Invalid regex returns a readable tool error, not a panic or silent empty result
    - 'Quality gate green: gofmt clean, go vet clean, go test ./internal/fileops/... ./internal/mcp/... -race -short'
    - 'Task finalized atomically: gates, task transition to done, and git commit of owned files in one run action=workflow'
verification_plan:
    - 'Write failing test per vertical slice at fileops seam: regex, case, globs, context, files_only, invert'
    - Run targeted go test per slice until green; keep pre-existing search tests untouched and green
    - 'Add MCP handler test: new params bind, literal default intact, invalid regex returns tool error'
    - 'Final gate: gofmt -l empty, go vet, go test -race -short on both packages, then atomic workflow finalization'
created_at: "2026-08-20T10:08:27.996724Z"
updated_at: "2026-08-20T10:21:52.43432Z"
---

## Body

Add rg-like search options to file action=search while keeping the literal substring default. Options: regex, ignore_case, smart_case (icase when pattern has no uppercase), glob include/exclude, context_before/context_after, files_only (rg -l), invert (rg -v). Seams confirmed with user: (1) public fileops search function with options, (2) file action=search handler binding. Result line format stays file:line:text (context lines use '-' separator, grep-style).

## Acceptance Criteria

- Literal substring default preserved: existing fileops and mcp search tests pass unchanged
- regex, ignore_case, smart_case, glob, glob_exclude, context_before, context_after, files_only, invert all work at fileops seam and via file action=search
- Invalid regex returns a readable tool error, not a panic or silent empty result
- Quality gate green: gofmt clean, go vet clean, go test ./internal/fileops/... ./internal/mcp/... -race -short
- Task finalized atomically: gates, task transition to done, and git commit of owned files in one run action=workflow

## Verification Plan

1. Write failing test per vertical slice at fileops seam: regex, case, globs, context, files_only, invert
2. Run targeted go test per slice until green; keep pre-existing search tests untouched and green
3. Add MCP handler test: new params bind, literal default intact, invalid regex returns tool error
4. Final gate: gofmt -l empty, go vet, go test -race -short on both packages, then atomic workflow finalization
