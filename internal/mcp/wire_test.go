package mcp

import (
	"strings"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/command"
	"github.com/alvnukov/mcp-ai-helper/internal/evidence"
	"github.com/alvnukov/mcp-ai-helper/internal/gitops"
	"github.com/alvnukov/mcp-ai-helper/internal/pipeline"
)

func TestWorkflowWireDropsMirroredStepRecords(t *testing.T) {
	check := command.Result{CommandID: "c1", Command: "go test ./..."}
	commit := gitops.CommitResult{Status: "ok"}
	got := workflowForWire(pipeline.WorkflowResult{
		Status: "ok",
		StepResults: []pipeline.WorkflowStepResult{
			{ID: "test", Tool: "command", Status: "ok", Output: check},
			{ID: "commit", Tool: "git_commit_owned", Status: "ok", Output: commit},
		},
		CheckResults: []command.Result{check},
		CommitResult: &commit,
	})
	if got.CheckResults != nil {
		t.Fatalf("check_results repeats step_results: %#v", got.CheckResults)
	}
	if got.CommitResult != nil {
		t.Fatalf("commit_result repeats step_results: %#v", got.CommitResult)
	}
	if len(got.StepResults) != 2 {
		t.Fatalf("step_results = %d, want the 2 records that carry the payload", len(got.StepResults))
	}
}

// The checks/edits request shape produces no step results at all, so there the
// per-category fields are the only report of the run.
func TestWorkflowWireKeepsUnmirroredResults(t *testing.T) {
	got := workflowForWire(pipeline.WorkflowResult{
		Status:       "ok",
		CheckResults: []command.Result{{CommandID: "c1", Command: "go vet ./..."}},
	})
	if len(got.CheckResults) != 1 {
		t.Fatalf("check_results = %#v, want the legacy path's only report kept", got.CheckResults)
	}
}

func TestCommandWireDropsEvidenceCopyingTails(t *testing.T) {
	got := commandForWire(command.Result{
		StdoutTail:    []string{"ok  github.com/x/y  0.6s"},
		EvidenceLines: []evidence.Line{{ID: "E1", Source: "command_output", Text: "ok  github.com/x/y  0.6s"}},
	})
	if got.EvidenceLines != nil {
		t.Fatalf("evidence_lines repeats stdout_tail: %#v", got.EvidenceLines)
	}
}

// A short failing command puts its failure line in the tail as well, so text
// comparison alone would drop the one line worth reading. The selection is the
// only thing that knows it distilled, and its answer wins.
func TestCommandWireKeepsDistilledEvidenceFoundInTail(t *testing.T) {
	got := commandForWire(command.Result{
		ExitCode:          1,
		StdoutTail:        []string{"--- FAIL: TestThing", "FAIL"},
		EvidenceLines:     []evidence.Line{{ID: "E1", Source: "command_output", Text: "--- FAIL: TestThing"}},
		EvidenceDistilled: true,
	})
	if len(got.EvidenceLines) != 1 {
		t.Fatalf("evidence_lines = %#v, want the distilled failure kept", got.EvidenceLines)
	}
}

// A check that runs as a workflow step repeats its tail in its evidence just
// like one run on its own, and workflows are how checks usually run.
func TestWorkflowWireShapesStepOutputs(t *testing.T) {
	got := workflowForWire(pipeline.WorkflowResult{
		Status: "ok",
		StepResults: []pipeline.WorkflowStepResult{{
			ID: "test", Tool: "command", Status: "ok",
			Output: command.Result{
				StdoutTail:    []string{"ok  github.com/x/y  0.6s"},
				EvidenceLines: []evidence.Line{{ID: "E1", Source: "command_output", Text: "ok  github.com/x/y  0.6s"}},
			},
		}},
	})
	step, ok := got.StepResults[0].Output.(command.Result)
	if !ok {
		t.Fatalf("step output changed type: %#v", got.StepResults[0].Output)
	}
	if step.EvidenceLines != nil {
		t.Fatalf("nested evidence_lines repeats stdout_tail: %#v", step.EvidenceLines)
	}
}

// Shaping runs on the way out, while the runner still holds the records its
// closeout decision read.
func TestWorkflowWireLeavesTheRunnersRecordsAlone(t *testing.T) {
	original := pipeline.WorkflowResult{
		Status: "ok",
		StepResults: []pipeline.WorkflowStepResult{{
			ID: "test", Tool: "command", Status: "ok",
			Output: command.Result{
				StdoutTail:    []string{"done"},
				EvidenceLines: []evidence.Line{{ID: "E1", Source: "command_output", Text: "done"}},
			},
		}},
	}
	_ = workflowForWire(original)
	step, ok := original.StepResults[0].Output.(command.Result)
	if !ok || len(step.EvidenceLines) != 1 {
		t.Fatalf("shaping wrote through to the runner's own record: %#v", original.StepResults[0].Output)
	}
}

func TestPipelineWireDropsTheCommandsCopyOfTheSummary(t *testing.T) {
	lines := []evidence.Line{{ID: "E1", Source: "command_output", Text: "--- FAIL: TestThing"}}
	got := pipelineForWire(pipeline.Result{
		Status:  "error",
		Command: command.Result{ExitCode: 1, StdoutTail: []string{"--- FAIL: TestThing", "FAIL"}, EvidenceLines: lines, EvidenceDistilled: true},
		Summary: evidence.Summary{EvidenceLines: lines},
	})
	if got.Command.EvidenceLines != nil {
		t.Fatalf("command repeats summary.evidence_lines: %#v", got.Command.EvidenceLines)
	}
	if len(got.Summary.EvidenceLines) != 1 {
		t.Fatalf("summary = %#v, want the copy the analysis cites kept", got.Summary.EvidenceLines)
	}
}

// The still-running result has no summary lines, so the command's copy is the
// only one there is and an emptied summary must not be read as a duplicate.
func TestPipelineWireKeepsEvidenceAnEmptySummaryCannotCarry(t *testing.T) {
	got := pipelineForWire(pipeline.Result{
		Status:  "running",
		Command: command.Result{StdoutTail: []string{"still building"}, EvidenceLines: []evidence.Line{{ID: "E1", Source: "command_output", Text: "cannot find package"}}, EvidenceDistilled: true},
	})
	if len(got.Command.EvidenceLines) != 1 {
		t.Fatalf("command evidence = %#v, want it kept when the summary has none", got.Command.EvidenceLines)
	}
}

// The shaping has to happen inside structured(), because a call site that
// forgets it ships the duplicate silently and there are dozens of them.
func TestStructuredAppliesWireShaping(t *testing.T) {
	res, err := structured(command.Result{
		CommandID:     "c1",
		StdoutTail:    []string{"done"},
		EvidenceLines: []evidence.Line{{ID: "E1", Source: "command_output", Text: "done"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if text := resultText(t, res); strings.Contains(text, "evidence_lines") {
		t.Fatalf("structured() shipped the duplicate: %s", text)
	}
}
