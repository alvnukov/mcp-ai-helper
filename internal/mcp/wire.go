package mcp

import (
	"strings"

	"github.com/alvnukov/mcp-ai-helper/internal/command"
	"github.com/alvnukov/mcp-ai-helper/internal/evidence"
	"github.com/alvnukov/mcp-ai-helper/internal/fileops"
	"github.com/alvnukov/mcp-ai-helper/internal/gitops"
	"github.com/alvnukov/mcp-ai-helper/internal/pipeline"
)

// forWire drops what a payload already states elsewhere in the same response.
//
// This is the rule that keeps results out of structuredContent, applied one
// level down. The reader is a model: a second copy of a fact costs it context
// while telling it nothing, and a field that is usually a copy teaches it to
// skip that field on the runs where the field does carry something.
//
// Nothing here drops information. Every branch removes a repetition of
// something that survives verbatim elsewhere in the very same payload.
//
// It hangs off structured() rather than off each call site because a call site
// that forgets it ships the duplicate silently, and there are dozens of them.
// Container cases route their nested payloads back through forWire rather than
// repeating any rule, so a result carried inside a workflow step is shaped the
// same as one returned on its own.
func forWire(value any) any {
	switch v := value.(type) {
	case command.Result:
		return commandForWire(v)
	case pipeline.Result:
		return pipelineForWire(v)
	case pipeline.WorkflowResult:
		return workflowForWire(v)
	}
	return value
}

// pipelineForWire drops the command's copy of the evidence lines.
//
// A pipeline result builds its summary from exactly the command's selection, so
// the same lines ship twice. The summary keeps them: the analysis cites their
// ids, and ValidateLinks resolves those citations against the summary alone.
// When the summary has no lines to keep — the still-running result — the
// command's own copy is all there is, and the ordinary tail rule decides it.
func pipelineForWire(result pipeline.Result) pipeline.Result {
	result.Command = commandForWire(result.Command)
	if sameEvidence(result.Command.EvidenceLines, result.Summary.EvidenceLines) {
		result.Command.EvidenceLines = nil
	}
	return result
}

// sameEvidence reports whether kept carries all of dropped, id and text alike.
// An empty kept is never an answer: there would be nothing left to read.
func sameEvidence(dropped, kept []evidence.Line) bool {
	if len(kept) == 0 || len(dropped) != len(kept) {
		return false
	}
	for i := range dropped {
		if dropped[i].ID != kept[i].ID || dropped[i].Text != kept[i].Text {
			return false
		}
	}
	return true
}

// workflowForWire drops the per-category mirrors of step records.
//
// A step-graph workflow reports every edit, check and commit twice: once as a
// step in step_results, and again in edit_results, check_results or
// commit_result. The runner fills both because its own closeout decision reads
// the per-category fields. The older checks/edits request shape produces no
// step results at all, and there those fields are the only report — so each is
// dropped only once the steps are known to account for the whole of it.
//
// Each step output is shaped in turn, because a check that runs as a step is
// the same command result as one run on its own and repeats its output tail in
// its evidence exactly the same way. The step slice is copied first: this runs
// on the way out, and the runner still holds the records it decided with.
func workflowForWire(result pipeline.WorkflowResult) pipeline.WorkflowResult {
	var edits, checks, commits int
	steps := make([]pipeline.WorkflowStepResult, len(result.StepResults))
	copy(steps, result.StepResults)
	for i := range steps {
		switch steps[i].Output.(type) {
		case fileops.ReplaceResult:
			edits++
		case command.Result:
			checks++
		case gitops.CommitResult:
			commits++
		}
		steps[i].Output = forWire(steps[i].Output)
	}
	if len(steps) > 0 {
		result.StepResults = steps
	}
	if edits == len(result.EditResults) {
		result.EditResults = nil
	}
	if checks == len(result.CheckResults) {
		result.CheckResults = nil
	} else {
		kept := make([]command.Result, len(result.CheckResults))
		for i, check := range result.CheckResults {
			kept[i] = commandForWire(check)
		}
		result.CheckResults = kept
	}
	if commits > 0 {
		result.CommitResult = nil
	}
	return result
}

// commandForWire drops evidence lines that only repeat the output tails.
//
// The evidence selection distils the lines reporting a failure, but when none
// of its keywords match — which is every successful command — it falls back to
// the tail of the output, the very lines stdout_tail already carries. Keeping
// the field for the runs where it is a real distillation is the point: that is
// when a reader should look at it.
//
// Whether it distilled is asked of the selection rather than guessed by
// comparing text, because a short failing command puts its failure line in the
// tail too, and comparing text would drop exactly the line that matters.
func commandForWire(result command.Result) command.Result {
	if len(result.EvidenceLines) == 0 || result.EvidenceDistilled {
		return result
	}
	shown := make(map[string]struct{}, len(result.StdoutTail)+len(result.StderrTail))
	for _, line := range result.StdoutTail {
		shown[strings.TrimSpace(line)] = struct{}{}
	}
	for _, line := range result.StderrTail {
		shown[strings.TrimSpace(line)] = struct{}{}
	}
	for _, line := range result.EvidenceLines {
		if _, ok := shown[strings.TrimSpace(line.Text)]; !ok {
			return result
		}
	}
	result.EvidenceLines = nil
	return result
}
