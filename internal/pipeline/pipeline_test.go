package pipeline

import (
	"context"
	"encoding/json"
	"github.com/zol/mcp-ai-helper/internal/tasks"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestPipelineReturnsGroundedHandoff(t *testing.T) {
	dir := t.TempDir()
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.Run(t.Context(), Request{Command: "printf 'error: bad\\n'; exit 2", RepoPath: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, handoff: %s", result.Status, result.Handoff)
	}
}

func TestRunWorkflowEditsThenChecks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Edits: []WorkflowEdit{{
			Path: "x.txt",
			Old:  "old",
			New:  "new",
		}},
		Checks: []WorkflowCommand{{Command: "grep -q new x.txt"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	// #nosec G304 -- test reads a file created inside t.TempDir.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new\n" {
		t.Fatalf("file = %q", string(data))
	}
}

func TestRunWorkflowStopsBeforeCommitOnFailedCheck(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Edits:    []WorkflowEdit{{Path: "x.txt", Old: "old", New: "new"}},
		Checks:   []WorkflowCommand{{Command: "exit 3"}},
		Commit:   WorkflowCommit{Enabled: true, Message: "should not commit"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.CommitResult != nil {
		t.Fatalf("commit should not run: %+v", result.CommitResult)
	}
}

func TestRunWorkflowStepsEditCheckCommit(t *testing.T) {
	dir := t.TempDir()
	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "test@example.invalid")
	runTestGit(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "edit", Tool: "guarded_replace", Args: map[string]any{"path": "x.txt", "old": "old", "new": "new"}},
			{ID: "check", Tool: "run_command", If: "steps.edit.status == ok", Args: map[string]any{"command": "grep -q new x.txt"}},
			{ID: "commit", Tool: "git_commit_owned", If: "changed_files_count > 0", Args: map[string]any{"message": "Update x"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if result.CommitResult == nil || result.CommitResult.Status != "ok" {
		t.Fatalf("commit did not run successfully: %+v", result.CommitResult)
	}
	if got := runTestGit(t, dir, "status", "--short"); got != "" {
		t.Fatalf("unexpected dirty status: %q", got)
	}
}

func TestRunWorkflowStepsTaskBatchUpsert(t *testing.T) {
	dir := t.TempDir()
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{{
			ID:   "tasks",
			Tool: "task_batch_upsert",
			Args: map[string]any{
				"tasks": []map[string]any{{
					"id":     "batch-api",
					"title":  "Batch API",
					"status": "todo",
				}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if len(result.StepResults) != 1 || result.StepResults[0].Status != "ok" {
		t.Fatalf("step results = %#v", result.StepResults)
	}
}

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	// #nosec G204 -- tests execute fixed git commands only.
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
	return string(out)
}

func testConfig(dir string) *config.Config {
	return &config.Config{
		Providers: map[string]config.ProviderConfig{},
		Models:    map[string]config.ModelConfig{},
		CommandPolicy: config.CommandPolicy{
			AllowedCWDs:           []string{dir},
			DefaultTimeoutSeconds: 1,
			MaxOutputBytes:        1000,
			MaxLines:              20,
		},
		PipelinePolicy: config.PipelinePolicy{RequireEvidenceForAnalysis: true, MaxReturnChars: 4000},
	}
}

func TestRunWorkflowTaskStepRespectsDisabledLayer(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	disabled := false
	cfg.Layers.Tasks.Enabled = &disabled
	runner := NewRunner(cfg, nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{{
			ID:   "tasks",
			Tool: "task_batch_upsert",
			Args: map[string]any{"tasks": []map[string]any{{"id": "x", "title": "X"}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || result.Reason != "task layer is disabled" {
		t.Fatalf("status = %q reason = %q", result.Status, result.Reason)
	}
}

func TestRunPipelineUpdatesCurrentTaskStatus(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	runner := NewRunner(testConfig(repoPath), nil)
	store := runner.tasks

	created, err := store.Add(tasks.AddRequest{
		RepoPath: repoPath,
		ID:       "task-pipeline-status",
		Title:    "pipeline status",
		Status:   "todo",
	})
	if err != nil {
		t.Fatalf("add task: %v", err)
	}

	result, err := runner.Run(t.Context(), Request{
		RepoPath:      repoPath,
		CurrentTaskID: created.ID,
		TaskOnSuccess: "done",
		Command:       "printf ok",
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if result.Command.ExitCode != 0 {
		t.Fatalf("exit code = %d", result.Command.ExitCode)
	}

	got, err := store.Get(tasks.GetRequest{RepoPath: repoPath, ID: created.ID})
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "done" {
		t.Fatalf("status = %q, want done", got.Status)
	}
}

func TestRunMarshalJSONCompactsSuccessfulOutputByDefault(t *testing.T) {
	repoPath := t.TempDir()
	runner := NewRunner(testConfig(repoPath), nil)
	result, err := runner.Run(context.Background(), Request{RepoPath: repoPath, Command: "printf 'large output should not be returned'"})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, "\"compact\":true") || !strings.Contains(text, "output: collapsed") {
		t.Fatalf("compact json missing markers: %s", text)
	}
	for _, forbidden := range []string{"large output should not be returned", "stdout_tail", "stderr_tail", "evidence_lines", "command\":"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("compact json leaked %q: %s", forbidden, text)
		}
	}
}

func TestRunMarshalJSONKeepsFailureDetails(t *testing.T) {
	repoPath := t.TempDir()
	runner := NewRunner(testConfig(repoPath), nil)
	result, err := runner.Run(context.Background(), Request{RepoPath: repoPath, Command: "sh -c 'echo fail >&2; exit 7'"})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, "fail") || !strings.Contains(text, "\"command\"") {
		t.Fatalf("failure json should retain details: %s", text)
	}
}

func TestRunMarshalJSONKeepsDetailsWhenCompactDisabled(t *testing.T) {
	repoPath := t.TempDir()
	runner := NewRunner(testConfig(repoPath), nil)
	compact := false
	result, err := runner.Run(context.Background(), Request{RepoPath: repoPath, Command: "printf 'needed detail'", CompactOutput: &compact})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, "needed detail") || !strings.Contains(text, "\"command\"") {
		t.Fatalf("non-compact json should retain details: %s", text)
	}
}
