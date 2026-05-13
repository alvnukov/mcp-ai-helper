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

type memoryTaskBackend struct {
	items map[string]tasks.Task
}

func newMemoryTaskBackend() *memoryTaskBackend {
	return &memoryTaskBackend{items: map[string]tasks.Task{}}
}

func (b *memoryTaskBackend) Add(req tasks.AddRequest) (tasks.Task, error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(req.Title), " ", "-"))
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "todo"
	}
	modelLevel, err := tasks.NormalizeModelLevel(req.ModelLevel)
	if err != nil {
		return tasks.Task{}, err
	}
	task := tasks.Task{ID: id, TaskType: req.TaskType, Branch: req.Branch, WorktreePath: req.WorktreePath, ParentID: req.ParentID, Status: status, Title: req.Title, Body: req.Body, Priority: req.Priority, ModelLevel: modelLevel, Tags: append([]string(nil), req.Tags...), AcceptanceCriteria: append([]string(nil), req.AcceptanceCriteria...), VerificationPlan: append([]string(nil), req.VerificationPlan...)}
	if err := tasks.NormalizeWorktreeFields(&task); err != nil {
		return tasks.Task{}, err
	}
	b.items[task.ID] = task
	return task, nil
}

func (b *memoryTaskBackend) Get(_ context.Context, _ string, id string) (tasks.Task, error) {
	item, ok := b.items[id]
	if !ok {
		return tasks.Task{}, os.ErrNotExist
	}
	return item, nil
}

func (b *memoryTaskBackend) List(context.Context, string) ([]tasks.Task, error) {
	items := make([]tasks.Task, 0, len(b.items))
	for _, item := range b.items {
		items = append(items, item)
	}
	return items, nil
}

func (b *memoryTaskBackend) setStatus(_ context.Context, req tasks.StatusRequest) (tasks.Task, error) {
	item, ok := b.items[req.ID]
	if !ok {
		return tasks.Task{}, os.ErrNotExist
	}
	item.Status = req.Status
	b.items[req.ID] = item
	return item, nil
}

func (b *memoryTaskBackend) BatchUpsert(_ context.Context, req tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error) {
	result := tasks.BatchUpsertResult{Source: "test_memory"}
	seen := map[string]struct{}{}
	for _, item := range req.Tasks {
		created, err := b.Add(item)
		if err != nil {
			return tasks.BatchUpsertResult{}, err
		}
		seen[created.ID] = struct{}{}
		result.Upserted = append(result.Upserted, created)
	}
	if req.CloseMissing {
		missingStatus := req.MissingStatus
		if missingStatus == "" {
			missingStatus = "done"
		}
		for id, item := range b.items {
			if _, ok := seen[id]; ok || item.Status == "done" {
				continue
			}
			item.Status = missingStatus
			b.items[id] = item
			result.Closed = append(result.Closed, item)
		}
	}
	return result, nil
}

func (b *memoryTaskBackend) SetStatus(ctx context.Context, req tasks.StatusRequest) (tasks.Task, error) {
	return b.setStatus(ctx, req)
}

func newTaskTestRunner(cfg *config.Config) (*Runner, *memoryTaskBackend) {
	backend := newMemoryTaskBackend()
	return NewRunnerWithTaskBackend(cfg, nil, backend), backend
}

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

func TestRunWorkflowStepsCommitUsesTopLevelOwnedFiles(t *testing.T) {
	dir := t.TempDir()
	runTestGit(t, dir, "init")
	runTestGit(t, dir, "config", "user.email", "test@example.invalid")
	runTestGit(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "y.txt"), []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Commit:   WorkflowCommit{Enabled: true, Files: []string{"x.txt", "y.txt"}, Message: "commit from top-level"},
		Steps: []WorkflowStep{
			{ID: "edit-x", Tool: "guarded_replace", Args: map[string]any{"path": "x.txt", "old": "old", "new": "new"}},
			{ID: "touch-y", Tool: "command", Args: map[string]any{"command": "printf after > y.txt"}},
			{ID: "commit", Tool: "git_commit_owned", If: "changed_files_count > 0", Args: map[string]any{}},
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
			{ID: "check", Tool: "command", If: "steps.edit.status == ok", Args: map[string]any{"command": "grep -q new x.txt"}},
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

func TestRunWorkflowStepsTwoEditsSameFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "edit1", Tool: "guarded_replace", Args: map[string]any{"path": "f.txt", "old": "line1", "new": "replaced1"}},
			{ID: "edit2", Tool: "guarded_replace", Args: map[string]any{"path": "f.txt", "old": "line2", "new": "replaced2"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if len(result.StepResults) != 2 || result.StepResults[0].Status != "ok" || result.StepResults[1].Status != "ok" {
		t.Fatalf("step results = %#v", result.StepResults)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "replaced1\nreplaced2\n" {
		t.Fatalf("file = %q", string(data))
	}
}

func TestRunWorkflowStepsParallelNoDeps(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "edit-a", Tool: "guarded_replace", Args: map[string]any{"path": "a.txt", "old": "old", "new": "new-a"}},
			{ID: "edit-b", Tool: "guarded_replace", Args: map[string]any{"path": "b.txt", "old": "old", "new": "new-b"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if len(result.StepResults) != 2 || result.StepResults[0].Status != "ok" || result.StepResults[1].Status != "ok" {
		t.Fatalf("step results = %#v", result.StepResults)
	}
}

func TestRunWorkflowStepsExplicitDependsOn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("A B\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "step2", Tool: "guarded_replace", DependsOn: []string{"step1"}, Args: map[string]any{"path": "f.txt", "old": "B", "new": "C"}},
			{ID: "step1", Tool: "guarded_replace", Args: map[string]any{"path": "f.txt", "old": "A", "new": "X"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	if len(result.StepResults) != 2 || result.StepResults[0].Status != "ok" || result.StepResults[1].Status != "ok" {
		t.Fatalf("step results = %#v", result.StepResults)
	}
	data, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "X C\n" {
		t.Fatalf("file = %q, want X C", string(data))
	}
}

func TestRunWorkflowStepsTaskBatchUpsert(t *testing.T) {
	dir := t.TempDir()
	runner, _ := newTaskTestRunner(testConfig(dir))
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

func TestRunWorkflowStepsTaskTransition(t *testing.T) {
	dir := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(dir))
	created, err := store.Add(tasks.AddRequest{RepoPath: dir, ID: "transition-api", Title: "Transition API", Status: "todo"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{{
			ID:   "transition",
			Tool: "task_transition",
			Args: map[string]any{"task_ids": []string{created.ID}, "from": "todo", "to": "in_progress"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	got, err := store.Get(context.Background(), dir, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
}

func TestRunWorkflowStepsTaskTransitionRejectsFromMismatch(t *testing.T) {
	dir := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(dir))
	created, err := store.Add(tasks.AddRequest{RepoPath: dir, ID: "transition-api", Title: "Transition API", Status: "todo"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{{
			ID:   "transition",
			Tool: "task_transition",
			Args: map[string]any{"task_ids": []string{created.ID}, "from": "blocked", "to": "done"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	got, err := store.Get(context.Background(), dir, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "todo" {
		t.Fatalf("status = %q, want todo", got.Status)
	}
}

func TestRunWorkflowStepsTaskTransitionRejectsClosingGoal(t *testing.T) {
	dir := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(dir))
	created, err := store.Add(tasks.AddRequest{RepoPath: dir, ID: "goal-main", Title: "Goal", Status: "in_progress"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{{
			ID:   "transition",
			Tool: "task_transition",
			Args: map[string]any{"task_ids": []string{created.ID}, "from": "in_progress", "to": "done"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestRunWorkflowStepsBranchOnCommandExitCodeAndOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "probe", Tool: "command", OnFailure: "continue", Args: map[string]any{"command": "printf needle; exit 7"}},
			{ID: "exit-branch", Tool: "guarded_replace", If: "steps.probe.exit_code == 7", Args: map[string]any{"path": "x.txt", "old": "old", "new": "exit"}},
			{ID: "output-branch", Tool: "guarded_replace", If: "steps.probe.output_contains needle", Args: map[string]any{"path": "x.txt", "old": "exit", "new": "matched"}},
			{ID: "success-branch", Tool: "guarded_replace", If: "steps.probe.exit_code == 0", Args: map[string]any{"path": "x.txt", "old": "matched", "new": "wrong"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "matched\n" {
		t.Fatalf("file = %q, want matched", string(data))
	}
}

func TestRunWorkflowStepsCompoundConditionsValidationAndChangedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(testConfig(dir), nil)

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "edit", Tool: "guarded_replace", Args: map[string]any{"path": "x.txt", "old": "old", "new": "new"}},
			{ID: "probe", Tool: "command", Args: map[string]any{"command": "printf 'error: needle\\n'"}},
			{ID: "branch", Tool: "command", If: "steps.probe.output_contains needle && steps.probe.validation == ok && changed_files contains x.txt", Args: map[string]any{"command": "printf branch > branch.txt"}},
			{ID: "else", Tool: "command", If: "! steps.probe.output_contains needle || file_missing x.txt", Args: map[string]any{"command": "printf wrong > branch.txt"}},
			{ID: "empty", Tool: "command", Args: map[string]any{"command": "true"}},
			{ID: "missing-evidence", Tool: "command", If: "steps.empty.validation == INSUFFICIENT_DATA", Args: map[string]any{"command": "printf missing-evidence > evidence.txt"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	branch, err := os.ReadFile(filepath.Join(dir, "branch.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(branch) != "branch" {
		t.Fatalf("branch.txt = %q, want branch", string(branch))
	}
	evidence, err := os.ReadFile(filepath.Join(dir, "evidence.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(evidence) != "missing-evidence" {
		t.Fatalf("evidence.txt = %q, want missing-evidence", string(evidence))
	}
}

func TestRunWorkflowStepsBranchOnFileAndTaskState(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("marker\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner, store := newTaskTestRunner(testConfig(dir))
	created, err := store.Add(tasks.AddRequest{RepoPath: dir, ID: "condition-task", Title: "Condition task", Status: "todo"})
	if err != nil {
		t.Fatal(err)
	}

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath: dir,
		Steps: []WorkflowStep{
			{ID: "missing-file", Tool: "guarded_replace", If: "file_missing marker.txt", Args: map[string]any{"path": "target.txt", "old": "old", "new": "wrong"}},
			{ID: "start-task", Tool: "task_transition", If: "file_exists marker.txt", Args: map[string]any{"task_ids": []string{created.ID}, "from": "todo", "to": "in_progress"}},
			{ID: "edit", Tool: "guarded_replace", DependsOn: []string{"start-task"}, If: "tasks.condition-task.status == in_progress", Args: map[string]any{"path": "target.txt", "old": "old", "new": "new"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	got, err := store.Get(context.Background(), dir, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "in_progress" {
		t.Fatalf("task status = %q, want in_progress", got.Status)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new\n" {
		t.Fatalf("file = %q, want new", string(data))
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
	runner, store := newTaskTestRunner(testConfig(repoPath))

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

	got, err := store.Get(context.Background(), repoPath, created.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "done" {
		t.Fatalf("status = %q, want done", got.Status)
	}
}

func TestRunPipelineBlocksTaskStatusWhenCommandFails(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(repoPath))
	created, err := store.Add(tasks.AddRequest{
		RepoPath: repoPath,
		ID:       "task-pipeline-failed-status",
		Title:    "pipeline failed status",
		Status:   "todo",
	})
	if err != nil {
		t.Fatalf("add task: %v", err)
	}

	result, err := runner.Run(t.Context(), Request{
		RepoPath:      repoPath,
		CurrentTaskID: created.ID,
		TaskOnSuccess: "done",
		TaskOnFailure: "blocked",
		Command:       "printf fail; exit 7",
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if result.Command.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.Command.ExitCode)
	}

	got, err := store.Get(context.Background(), repoPath, created.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", got.Status)
	}
}

func TestRunPipelineBlocksTaskStatusWhenEvidenceInvalid(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(repoPath))
	created, err := store.Add(tasks.AddRequest{
		RepoPath: repoPath,
		ID:       "task-pipeline-invalid-evidence",
		Title:    "pipeline invalid evidence",
		Status:   "todo",
	})
	if err != nil {
		t.Fatalf("add task: %v", err)
	}

	result, err := runner.Run(t.Context(), Request{
		RepoPath:      repoPath,
		CurrentTaskID: created.ID,
		TaskOnSuccess: "done",
		TaskOnFailure: "blocked",
		Command:       "true",
	})
	if err != nil {
		t.Fatalf("run pipeline: %v", err)
	}
	if result.Status != "INSUFFICIENT_DATA" {
		t.Fatalf("status = %q, want INSUFFICIENT_DATA", result.Status)
	}

	got, err := store.Get(context.Background(), repoPath, created.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", got.Status)
	}
}

func TestRunWorkflowBlocksTaskStatusWhenStepSkipped(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(repoPath))
	created, err := store.Add(tasks.AddRequest{
		RepoPath: repoPath,
		ID:       "task-workflow-skipped-step",
		Title:    "workflow skipped step",
		Status:   "todo",
	})
	if err != nil {
		t.Fatalf("add task: %v", err)
	}

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath:      repoPath,
		CurrentTaskID: created.ID,
		Steps: []WorkflowStep{{
			ID:   "required-gate",
			Tool: "command",
			If:   "file_exists missing.txt",
			Args: map[string]any{"command": "printf ok"},
		}},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("workflow status = %q, want ok", result.Status)
	}
	if len(result.StepResults) != 1 || result.StepResults[0].Status != "skipped" {
		t.Fatalf("step results = %#v", result.StepResults)
	}

	got, err := store.Get(context.Background(), repoPath, created.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", got.Status)
	}
}

func TestRunWorkflowBlocksTaskStatusWhenCommitSkipped(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	runner, store := newTaskTestRunner(testConfig(repoPath))
	created, err := store.Add(tasks.AddRequest{
		RepoPath: repoPath,
		ID:       "task-workflow-skipped-commit",
		Title:    "workflow skipped commit",
		Status:   "todo",
	})
	if err != nil {
		t.Fatalf("add task: %v", err)
	}

	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath:      repoPath,
		CurrentTaskID: created.ID,
		Commit:        WorkflowCommit{Enabled: true, Message: "no changes"},
	})
	if err != nil {
		t.Fatalf("run workflow: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("workflow status = %q, want ok", result.Status)
	}
	if result.CommitResult == nil || result.CommitResult.Status != "skipped" {
		t.Fatalf("commit result = %#v, want skipped", result.CommitResult)
	}

	got, err := store.Get(context.Background(), repoPath, created.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", got.Status)
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
	for _, forbidden := range []string{"stdout_tail", "stderr_tail", "command\":"} {
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

func TestSecretInjectionEnvVarReachesCommand(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Secrets = map[string]config.SecretConfig{
		"GH_TOKEN": {Value: "fake-gh-token-42a7b9c1d3e5f", Enabled: true},
	}
	runner := NewRunner(cfg, nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath:      dir,
		SecretHandles: []string{"GH_TOKEN"},
		Checks: []WorkflowCommand{{
			Command: "sh -c 'echo -n $HELPER_SECRET_GH_TOKEN'",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, reason = %q", result.Status, result.Reason)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Contains(text, "fake-gh-token-42a7b9c1d3e5f") {
		t.Error("T1-FAIL: raw secret leaked into model-facing output")
	} else if strings.Contains(text, "[HELPER_SECRET:GH_TOKEN]") {
		t.Log("T1-PASS: secret masked correctly in output")
	} else {
		t.Error("T1-FAIL: secret neither leaked nor masked — injection may have failed")
	}
}

func TestSecretOutputRedactionEchoLeak(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Secrets = map[string]config.SecretConfig{
		"NPM_TOKEN": {Value: "fake-npm-token-deadbeef12345678", Enabled: true},
	}
	runner := NewRunner(cfg, nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath:      dir,
		SecretHandles: []string{"NPM_TOKEN"},
		Checks: []WorkflowCommand{{
			Command: "sh -c 'echo $HELPER_SECRET_NPM_TOKEN'",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Contains(text, "fake-npm-token-deadbeef12345678") {
		t.Error("T2-FAIL: raw secret leaked into model-facing output")
	}
	if !strings.Contains(text, "[HELPER_SECRET:NPM_TOKEN]") {
		t.Error("T2-FAIL: masked token not found in output — redaction may have failed")
	}
}

func TestSecretUnknownHandleFailsClosed(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Secrets = map[string]config.SecretConfig{}
	runner := NewRunner(cfg, nil)
	_, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath:      dir,
		SecretHandles: []string{"NONEXISTENT"},
		Checks: []WorkflowCommand{{
			Command: "echo should-not-run",
		}},
	})
	if err == nil {
		t.Error("T5-FAIL: expected error for unknown handle, got nil")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("T5-FAIL: expected 'not found' error, got: %v", err)
	}
}

func TestSecretConfigExcludedFromJSON(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.Secrets = map[string]config.SecretConfig{
		"XYZ": {Value: "should-not-appear-0123456789ab", Enabled: true},
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "should-not-appear") {
		t.Error("T9-FAIL: secret value appeared in config JSON")
	}
	if strings.Contains(text, "Secrets") || strings.Contains(text, "secrets") {
		t.Error("T9-FAIL: Secrets field appeared in config JSON serialization")
	}
}

func TestSecretNotLeakedInResultCommandField(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Secrets = map[string]config.SecretConfig{
		"TOKEN": {Value: "secret-xyz-1111111111111111", Enabled: true},
	}
	runner := NewRunner(cfg, nil)
	result, err := runner.RunWorkflow(t.Context(), WorkflowRequest{
		RepoPath:      dir,
		SecretHandles: []string{"TOKEN"},
		Checks: []WorkflowCommand{{
			Command: "sh -c 'echo $HELPER_SECRET_TOKEN; exit 0'",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Contains(text, "secret-xyz-1111111111111111") {
		t.Error("T7-FAIL: raw secret value leaked in model-facing result")
	}
	if !strings.Contains(text, "[HELPER_SECRET:TOKEN]") {
		t.Error("T7-FAIL: masked token not found in result — command field may leak or injection failed")
	}
}
