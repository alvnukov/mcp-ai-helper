---
id: workflow-step-args-must-not-drop-unknown-keys
title: Workflow steps must report unknown arguments instead of dropping them
status: todo
priority: high
model_level: high
task_type: bug
parent_id: mcp-helper-llm-reliability-audit
tags:
    - workflow
    - surface-contract
    - llm-reliability
acceptance_criteria:
    - Unknown argument keys are detected for every step tool, not one of them
    - The detection is either a request error or a recorded warning, chosen deliberately and justified in a comment
    - A regression test sends a misspelled key to each step tool and asserts it is reported
    - Any behaviour change is stated in run action=schema so a caller reads it before hitting it
verification_plan:
    - Run go test ./internal/pipeline ./internal/mcp
    - Run make quality
    - Run golangci-lint
created_at: "2026-08-03T21:00:52.855368Z"
updated_at: "2026-08-03T21:00:52.855368Z"
---

## Body

bindStepArgs (internal/pipeline/pipeline.go) marshals a step's args map and decodes it into the per-tool struct with plain json.Unmarshal. Every key the struct does not carry is discarded silently: a misspelled "mesage", an argument belonging to a different step tool, a field the engine never implemented. The step then runs with zero values and reports its own success, so the caller learns nothing and cannot tell a typo from an unsupported feature.

guarded_replace losing old_b64/new_b64 was one instance of this class. It was closed by binding those fields, which fixes the instance and leaves the class untouched: the next unbound key will disappear the same way.

The fix needs a decision, not just an implementation. json.Decoder.DisallowUnknownFields makes every drop a named error, but it is a breaking change for every downstream repo whose workflows currently pass an extra key, and this repository is the helper those repos depend on. The softer option is a pre-flight comparison in prepareWorkflow that records unknown keys as a warning on the step result while still running the step. Pick one deliberately and say why in the code.

Whichever is chosen, the check must cover every step tool, not guarded_replace alone.

## Acceptance Criteria

- Unknown argument keys are detected for every step tool, not one of them
- The detection is either a request error or a recorded warning, chosen deliberately and justified in a comment
- A regression test sends a misspelled key to each step tool and asserts it is reported
- Any behaviour change is stated in run action=schema so a caller reads it before hitting it

## Verification Plan

1. Run go test ./internal/pipeline ./internal/mcp
2. Run make quality
3. Run golangci-lint
