---
id: workflow-guarded-replace-must-support-binary-safe-args
title: Workflow guarded_replace must accept the same text arguments as edit action=replace
status: done
priority: high
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - surface-contract
    - escaping
    - llm-reliability
acceptance_criteria:
    - guarded_replace workflow steps accept old_b64/new_b64 with parity to edit action=replace
    - The legacy edits list honours the same arguments as the step form
    - Schema and binder name the same fields, checked in both directions against the struct the step decodes into
    - Regression test edits source containing literal escape sequences through b64 arguments
    - Malformed base64 fails the request before any workflow step writes
verification_plan:
    - Run go test ./internal/pipeline ./internal/fileops ./internal/mcp
    - Mutation-check each new test by removing the code it covers
    - Run go vet and gofmt
created_at: "2026-08-01T12:43:25.227907Z"
updated_at: "2026-08-03T21:14:35.111348Z"
---

## Body

Corrected 2026-08-03. The original report said the workflow step "rejects documented old_b64/new_b64". It does not reject them: WorkflowEdit carried no such fields, and bindStepArgs is a plain json.Unmarshal, so the arguments were dropped without a word. The step then reached ApplyGuardedReplace with an empty span and failed with "old text is required (set old or old_b64)" — an error naming a field the workflow layer never bound, which sends the caller back for a retry that cannot succeed.

The documentation half was already closed: run action=schema stopped advertising base64 for the workflow path and pointed at edit action=replace instead. What remained was the capability gap, and closing it the other way is cheaper and removes the trap: the error string lives in fileops and is shared by both surfaces, so while the surfaces differ it is a lie in one of them and cannot be fixed locally for both. Parity makes it true everywhere.

Scope: bind OldB64/NewB64 on WorkflowEdit and route both construction sites (the legacy edits list and the guarded_replace step) through one replaceRequest method so they cannot drift again; validate every encoding in prepareWorkflow, before the first step runs, using the decoder ApplyGuardedReplace itself uses.

The wider defect behind this one — bindStepArgs discarding any unknown key for any step tool — is tracked separately in workflow-step-args-must-not-drop-unknown-keys.

## Acceptance Criteria

- guarded_replace workflow steps accept old_b64/new_b64 with parity to edit action=replace
- The legacy edits list honours the same arguments as the step form
- Schema and binder name the same fields, checked in both directions against the struct the step decodes into
- Regression test edits source containing literal escape sequences through b64 arguments
- Malformed base64 fails the request before any workflow step writes

## Verification Plan

1. Run go test ./internal/pipeline ./internal/fileops ./internal/mcp
2. Mutation-check each new test by removing the code it covers
3. Run go vet and gofmt
