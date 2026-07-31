package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Steps with no dependencies between them run in one wave, concurrently. They
// all report into the same changed-file set and the same map of post-edit
// hashes, and the file locks that guard the edits themselves do nothing here
// because every step touches a different path.
//
// Two steps were enough to leave this unnoticed; a dozen makes the window wide
// enough that an unsynchronised map is a fatal "concurrent map writes" rather
// than a rare one. Run under -race it also catches the read side, which
// assembles ChangedFiles while other steps are still writing.
func TestParallelEditsInOneWaveAgreeOnTheFilesTheyChanged(t *testing.T) {
	dir := t.TempDir()
	const count = 12

	steps := make([]WorkflowStep, 0, count)
	want := make([]string, 0, count)
	for i := range count {
		name := fmt.Sprintf("file-%02d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		steps = append(steps, WorkflowStep{
			ID:   fmt.Sprintf("edit-%02d", i),
			Tool: "guarded_replace",
			Args: map[string]any{"path": name, "old": "old", "new": fmt.Sprintf("new-%02d", i)},
		})
		want = append(want, name)
	}

	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{RepoPath: dir, Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if len(result.StepResults) != count {
		t.Fatalf("got %d step results, want %d", len(result.StepResults), count)
	}

	got := append([]string(nil), result.ChangedFiles...)
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("changed files = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("changed files = %v, want %v", got, want)
		}
	}

	for i, name := range want {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if wantContent := fmt.Sprintf("new-%02d\n", i); string(content) != wantContent {
			t.Errorf("%s = %q, want %q", name, content, wantContent)
		}
	}
}

// A commit step with no explicit file list falls back to whatever the workflow
// changed, so the fallback has to see every parallel edit — not the subset that
// happened to land before the commit step read the set.
func TestACommitWithNoFileListPicksUpEveryParallelEdit(t *testing.T) {
	dir := t.TempDir()
	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "test@example.invalid")
	runTestGit(t, dir, "config", "user.name", "Test User")
	const count = 8

	steps := make([]WorkflowStep, 0, count+1)
	for i := range count {
		name := fmt.Sprintf("file-%02d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		steps = append(steps, WorkflowStep{
			ID:   fmt.Sprintf("edit-%02d", i),
			Tool: "guarded_replace",
			Args: map[string]any{"path": name, "old": "old", "new": fmt.Sprintf("new-%02d", i)},
		})
	}
	steps = append(steps, WorkflowStep{
		ID:   "commit",
		Tool: "git_commit_owned",
		Args: map[string]any{"message": "parallel edits"},
	})

	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{RepoPath: dir, Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if result.CommitResult == nil {
		t.Fatal("workflow produced no commit result")
	}
	if len(result.CommitResult.StagedFiles) != count {
		t.Fatalf("staged %v, want all %d edited files", result.CommitResult.StagedFiles, count)
	}
}
