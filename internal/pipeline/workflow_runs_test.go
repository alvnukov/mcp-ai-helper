package pipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A workflow that outlives the caller's wait budget must stay addressable: the
// early lookup sees a running record, and a later one returns the final result
// without rerunning anything. This is the regression for MCP calls that timed
// out mid-workflow and lost the structured result.
func TestWorkflowRegistryReturnsFinalResultAfterWaitBudgetExceeded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	req := WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "slow", Tool: "command", Args: map[string]any{"command": "sleep 2", "timeout_seconds": 30, "mcp_wait_seconds": 30}},
			{ID: "edit", Tool: "guarded_replace", DependsOn: []string{"slow"}, Args: map[string]any{"path": "note.txt", "old": "old", "new": "new"}},
		},
	}

	registry := NewWorkflowRegistry()
	started := registry.Start(runner, req)
	if started.Status != "running" || started.WorkflowID == "" {
		t.Fatalf("start record = %+v, want running with a workflow id", started)
	}

	early, ok := registry.Get(started.WorkflowID)
	if !ok || early.Status != "running" || early.Result != nil {
		t.Fatalf("early record = %+v (ok=%v), want running without a result", early, ok)
	}

	final, ok := registry.WaitFor(started.WorkflowID, 30*time.Second)
	if !ok {
		t.Fatal("workflow record disappeared from the registry")
	}
	if final.FinishedAt == nil {
		t.Fatal("workflow did not finish within the wait budget")
	}
	if final.Error != "" {
		t.Fatalf("workflow error: %s", final.Error)
	}
	if final.Result == nil || final.Result.Status != "ok" {
		t.Fatalf("final result = %+v, want a completed ok result", final.Result)
	}
	if len(final.Result.ChangedFiles) == 0 {
		t.Fatal("final result carries no changed files")
	}

	if _, ok := registry.Get("wf-does-not-exist"); ok {
		t.Fatal("unknown workflow id must not resolve")
	}
}

func TestWorkflowWaitBudgetClamp(t *testing.T) {
	if got := WorkflowWaitBudget(0); got != 10*time.Second {
		t.Fatalf("default budget = %s, want 10s", got)
	}
	if got := WorkflowWaitBudget(600); got != 25*time.Second {
		t.Fatalf("budget cap = %s, want 25s", got)
	}
	if got := WorkflowWaitBudget(3); got != 3*time.Second {
		t.Fatalf("budget = %s, want the requested 3s", got)
	}
}
