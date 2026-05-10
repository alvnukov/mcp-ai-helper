// Package pipeline coordinates workflows, command checks, guarded edits, and commits.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/evidence"
	"github.com/zol/mcp-ai-helper/internal/fileops"
	"github.com/zol/mcp-ai-helper/internal/gitops"
	"github.com/zol/mcp-ai-helper/internal/project"
	"github.com/zol/mcp-ai-helper/internal/provider"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// Runner executes analysis and edit workflows against a repository.
type Runner struct {
	cfg         *config.Config
	commands    *command.Runner
	chatClient  provider.ChatClient
	tasks       *tasks.Store
	taskBackend TaskBackend
}

// TaskBackend is the task persistence surface used by workflow steps.
type TaskBackend interface {
	Get(ctx context.Context, repoPath string, id string) (tasks.Task, error)
	List(ctx context.Context, repoPath string) ([]tasks.Task, error)
	SetStatus(ctx context.Context, req tasks.StatusRequest) (tasks.Task, error)
	BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error)
}

type legacyTaskBackend struct {
	store *tasks.Store
}

func (b legacyTaskBackend) Get(_ context.Context, repoPath string, id string) (tasks.Task, error) {
	return b.store.Get(tasks.GetRequest{RepoPath: repoPath, ID: id})
}

func (b legacyTaskBackend) List(_ context.Context, repoPath string) ([]tasks.Task, error) {
	return b.store.List(tasks.ListRequest{RepoPath: repoPath})
}

func (b legacyTaskBackend) SetStatus(_ context.Context, req tasks.StatusRequest) (tasks.Task, error) {
	return b.store.SetStatus(req)
}

func (b legacyTaskBackend) BatchUpsert(_ context.Context, req tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error) {
	result, err := b.store.BatchUpsert(req)
	result.Source = "legacy_registry"
	return result, err
}

// Request describes the legacy command-analysis pipeline input.
type Request struct {
	CurrentTaskID  string `json:"current_task_id,omitempty"`
	TaskOnStart    string `json:"task_on_start,omitempty"`
	TaskOnSuccess  string `json:"task_on_success,omitempty"`
	TaskOnFailure  string `json:"task_on_failure,omitempty"`
	Command        string `json:"command"`
	RepoPath       string `json:"repo_path"`
	CWD            string `json:"cwd"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Analyze        bool   `json:"analyze"`
	Task           string `json:"task"`
	ModelID        string `json:"model_id"`
	CompactOutput  *bool  `json:"compact_output,omitempty"`
}

// Result is the command-analysis pipeline output.
type Result struct {
	Status     string              `json:"status"`
	Compact    bool                `json:"compact"`
	Command    command.Result      `json:"command"`
	Summary    evidence.Summary    `json:"summary"`
	Analysis   string              `json:"analysis,omitempty"`
	Validation evidence.Validation `json:"validation"`
	Handoff    string              `json:"handoff"`
}

func (r Result) MarshalJSON() ([]byte, error) {
	if r.Compact && r.Status == "ok" && r.Command.ExitCode == 0 {
		return json.Marshal(struct {
			Status    string `json:"status"`
			CommandID string `json:"command_id,omitempty"`
			ExitCode  int    `json:"exit_code"`
			Compact   bool   `json:"compact"`
			Handoff   string `json:"handoff"`
		}{
			Status:    r.Status,
			CommandID: r.Command.CommandID,
			ExitCode:  r.Command.ExitCode,
			Compact:   true,
			Handoff:   r.Handoff,
		})
	}
	type full Result
	return json.Marshal(full(r))
}

// WorkflowRequest describes a complete repository workflow request.
type WorkflowRequest struct {
	CurrentTaskID string            `json:"current_task_id,omitempty"`
	TaskOnStart   string            `json:"task_on_start,omitempty"`
	TaskOnSuccess string            `json:"task_on_success,omitempty"`
	TaskOnFailure string            `json:"task_on_failure,omitempty"`
	RepoPath      string            `json:"repo_path"`
	Steps         []WorkflowStep    `json:"steps"`
	Edits         []WorkflowEdit    `json:"edits"`
	Checks        []WorkflowCommand `json:"checks"`
	Commit        WorkflowCommit    `json:"commit"`
}

// WorkflowStep is one deterministic workflow DSL step.
type WorkflowStep struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	DependsOn []string       `json:"depends_on,omitempty"`
	If        string         `json:"if"`
	OnFailure string         `json:"on_failure"`
	Args      map[string]any `json:"args"`
}

// WorkflowStepResult records the structured result of one workflow step.
type WorkflowStepResult struct {
	ID     string `json:"id"`
	Tool   string `json:"tool"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Output any    `json:"output,omitempty"`
}

// WorkflowEdit describes one guarded text replacement.
type WorkflowEdit struct {
	Path         string `json:"path"`
	ExpectedHash string `json:"expected_hash"`
	Old          string `json:"old"`
	New          string `json:"new"`
}

// WorkflowCommand describes one repo-scoped command check.
type WorkflowCommand struct {
	Command        string         `json:"command"`
	CWD            string         `json:"cwd"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	Filter         command.Filter `json:"filter"`
}

// WorkflowCommit controls optional owned-file commit behavior.
type WorkflowCommit struct {
	Enabled bool     `json:"enabled"`
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

// WorkflowTaskTransition describes one guarded task status transition step.
type WorkflowTaskTransition struct {
	TaskIDs []string `json:"task_ids"`
	From    string   `json:"from"`
	To      string   `json:"to"`
}

// WorkflowResult is the complete workflow execution record.
type WorkflowResult struct {
	Status       string                  `json:"status"`
	FailedStepID string                  `json:"failed_step_id,omitempty"`
	StepResults  []WorkflowStepResult    `json:"step_results,omitempty"`
	EditResults  []fileops.ReplaceResult `json:"edit_results"`
	CheckResults []command.Result        `json:"check_results"`
	CommitResult *gitops.CommitResult    `json:"commit_result,omitempty"`
	ChangedFiles []string                `json:"changed_files"`
	Reason       string                  `json:"reason,omitempty"`
}

// NewRunner creates a workflow runner.
func NewRunner(cfg *config.Config, chatClient provider.ChatClient) *Runner {
	return NewRunnerWithTaskBackend(cfg, chatClient, nil)
}

// NewRunnerWithTaskBackend creates a workflow runner with an explicit task backend.
func NewRunnerWithTaskBackend(cfg *config.Config, chatClient provider.ChatClient, taskBackend TaskBackend) *Runner {
	projectStore, err := project.NewStore(cfg.CommandPolicy.LogDir)
	if err != nil {
		projectStore, _ = project.NewStore(".mcp-ai-helper")
	}
	store := tasks.NewStore(projectStore)
	if taskBackend == nil {
		taskBackend = legacyTaskBackend{store: store}
	}
	return &Runner{cfg: cfg, commands: command.NewRunner(cfg.CommandPolicy), chatClient: chatClient, tasks: store, taskBackend: taskBackend}
}

// RunWorkflow executes either the stable steps DSL or the legacy edit/check/commit workflow.
func (r *Runner) RunWorkflow(ctx context.Context, req WorkflowRequest) (result WorkflowResult, err error) {
	if err := r.updateTaskStatus(ctx, req.CurrentTaskID, taskStatusOrDefault(req.TaskOnStart, "in_progress"), req.RepoPath); err != nil {
		return WorkflowResult{}, err
	}
	defer func() {
		finalStatus := taskStatusOrDefault(req.TaskOnSuccess, "done")
		if !workflowTaskCloseoutSucceeded(req, result, err) {
			finalStatus = taskStatusOrDefault(req.TaskOnFailure, "blocked")
		}
		if updateErr := r.updateTaskStatus(ctx, req.CurrentTaskID, finalStatus, req.RepoPath); updateErr != nil && err == nil {
			err = updateErr
		}
	}()

	if strings.TrimSpace(req.RepoPath) == "" {
		return WorkflowResult{}, errors.New("repo_path is required")
	}
	if len(req.Steps) > 0 {
		return r.runWorkflowSteps(ctx, req)
	}
	result = WorkflowResult{Status: "ok"}
	changedSet := map[string]struct{}{}
	for _, edit := range req.Edits {
		replaceReq := fileops.ReplaceRequest{RepoPath: req.RepoPath, Path: edit.Path, ExpectedHash: edit.ExpectedHash, Old: edit.Old, New: edit.New}
		if replaceReq.ExpectedHash == "" {
			snapshot, err := fileops.ReadSnapshotInRepo(req.RepoPath, edit.Path)
			if err != nil {
				return WorkflowResult{}, err
			}
			if !snapshot.Exists {
				return WorkflowResult{Status: "conflict", Reason: "file does not exist: " + edit.Path}, nil
			}
			replaceReq.ExpectedHash = snapshot.Hash
		}
		editResult, err := fileops.ApplyGuardedReplace(replaceReq)
		if err != nil {
			return WorkflowResult{}, err
		}
		result.EditResults = append(result.EditResults, editResult)
		if editResult.Status != "ok" {
			result.Status = editResult.Status
			result.Reason = editResult.Reason
			return result, nil
		}
		if editResult.Changed {
			changedSet[edit.Path] = struct{}{}
		}
	}
	for file := range changedSet {
		result.ChangedFiles = append(result.ChangedFiles, file)
	}
	for _, check := range req.Checks {
		checkResult, err := r.commands.RunFilteredInRepo(ctx, check.Command, req.RepoPath, check.CWD, check.TimeoutSeconds, check.Filter)
		if err != nil {
			return WorkflowResult{}, err
		}
		result.CheckResults = append(result.CheckResults, checkResult)
		if checkResult.ExitCode != 0 {
			result.Status = "failed"
			result.Reason = "check failed: " + check.Command
			return result, nil
		}
	}
	if req.Commit.Enabled {
		files := req.Commit.Files
		if len(files) == 0 {
			files = result.ChangedFiles
		}
		commitResult, err := gitops.CommitOwned(ctx, gitops.CommitRequest{RepoPath: req.RepoPath, Files: files, Message: req.Commit.Message})
		if err != nil {
			return WorkflowResult{}, err
		}
		result.CommitResult = &commitResult
		if commitResult.Status != "ok" && commitResult.Status != "skipped" {
			result.Status = commitResult.Status
			result.Reason = commitResult.Reason
			return result, nil
		}
	}
	return result, nil
}

func (r *Runner) runWorkflowSteps(ctx context.Context, req WorkflowRequest) (WorkflowResult, error) {
	result := WorkflowResult{Status: "ok"}
	stepResults := map[string]WorkflowStepResult{}
	changedSet := map[string]struct{}{}
	fileHashes := map[string]string{}

	waves := buildStepWaves(req.Steps)

	var stateMu sync.Mutex
	fileLocks := newFileLockSet()

	for _, wave := range waves {
		var wg sync.WaitGroup

		for i := range wave {
			s := &wave[i]
			wg.Add(1)
			go func(step *WorkflowStep) {
				defer wg.Done()

				stateMu.Lock()
				if !r.evalStepCondition(req.RepoPath, step.If, result, stepResults) {
					sr := WorkflowStepResult{ID: step.ID, Tool: step.Tool, Status: "skipped", Reason: "condition is false"}
					result.StepResults = append(result.StepResults, sr)
					if step.ID != "" {
						stepResults[step.ID] = sr
					}
					stateMu.Unlock()
					return
				}
				paths := stepFilePaths(step)
				stateMu.Unlock()

				fileLocks.lock(paths)
				sr, execErr := r.executeWorkflowStep(ctx, req.RepoPath, *step, changedSet, fileHashes, commitPtr(req.Commit))
				fileLocks.unlock(paths)

				if execErr != nil {
					sr = WorkflowStepResult{ID: step.ID, Tool: step.Tool, Status: "failed", Reason: execErr.Error()}
				}

				stateMu.Lock()
				result.StepResults = append(result.StepResults, sr)
				if step.ID != "" {
					stepResults[step.ID] = sr
				}
				if editResult, ok := sr.Output.(fileops.ReplaceResult); ok {
					result.EditResults = append(result.EditResults, editResult)
				}
				if checkResult, ok := sr.Output.(command.Result); ok {
					result.CheckResults = append(result.CheckResults, checkResult)
				}
				if commitResult, ok := sr.Output.(gitops.CommitResult); ok {
					result.CommitResult = &commitResult
				}
				result.ChangedFiles = sortedKeys(changedSet)
				if sr.Status != "ok" && sr.Status != "skipped" && step.OnFailure != "continue" && result.Status == "ok" {
					result.Status = sr.Status
					result.Reason = sr.Reason
					result.FailedStepID = step.ID
				}
				stateMu.Unlock()
			}(s)
		}
		wg.Wait()

		if result.Status != "ok" {
			break
		}
	}

	result.ChangedFiles = sortedKeys(changedSet)
	return result, nil
}

// buildStepWaves topologically sorts steps into parallel-execution waves.
func buildStepWaves(steps []WorkflowStep) [][]WorkflowStep {
	if len(steps) == 0 {
		return nil
	}
	idx := map[string]int{}
	for i, s := range steps {
		if s.ID != "" {
			idx[s.ID] = i
		}
	}
	deps := make([][]int, len(steps))
	reverse := make([][]int, len(steps))
	// Gather all guarded_replace step indices for implicit changedSet dependencies.
	var editIndices []int
	for i, s := range steps {
		if s.Tool == "guarded_replace" {
			editIndices = append(editIndices, i)
		}
	}
	for i, s := range steps {
		for _, depID := range s.DependsOn {
			if j, ok := idx[depID]; ok {
				deps[i] = append(deps[i], j)
				reverse[j] = append(reverse[j], i)
			}
		}
		for _, depID := range parseStepConditionDeps(s.If) {
			if j, ok := idx[depID]; ok {
				deps[i] = append(deps[i], j)
				reverse[j] = append(reverse[j], i)
			}
		}
		if s.Tool == "guarded_replace" {
			p := stepFilePath(&s)
			for j := i - 1; j >= 0 && p != ""; j-- {
				if steps[j].Tool == "guarded_replace" && stepFilePath(&steps[j]) == p {
					deps[i] = append(deps[i], j)
					reverse[j] = append(reverse[j], i)
					break
				}
			}
		}
		// Steps that read changedSet implicitly depend on all guarded_replace steps.
		if readsChangedSet(&s) {
			for _, j := range editIndices {
				if j != i {
					deps[i] = append(deps[i], j)
					reverse[j] = append(reverse[j], i)
				}
			}
		}
	}
	inDegree := make([]int, len(steps))
	for i := range steps {
		inDegree[i] = len(deps[i])
	}
	var waves [][]WorkflowStep
	processed := 0
	for processed < len(steps) {
		var wave []WorkflowStep
		for i := range steps {
			if inDegree[i] == 0 {
				wave = append(wave, steps[i])
				inDegree[i] = -1
				processed++
			}
		}
		if len(wave) == 0 {
			return [][]WorkflowStep{steps}
		}
		for _, s := range wave {
			if j, ok := idx[s.ID]; ok {
				for _, k := range reverse[j] {
					if inDegree[k] > 0 {
						inDegree[k]--
					}
				}
			}
		}
		waves = append(waves, wave)
	}
	return waves
}

func parseStepConditionDeps(cond string) []string {
	fields := strings.Fields(strings.TrimSpace(cond))
	deps := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		dep := stepConditionDependency(field)
		if dep == "" {
			continue
		}
		if _, ok := seen[dep]; ok {
			continue
		}
		seen[dep] = struct{}{}
		deps = append(deps, dep)
	}
	return deps
}

func stepConditionDependency(field string) string {
	field = strings.TrimLeft(strings.TrimSpace(field), "!")
	if !strings.HasPrefix(field, "steps.") {
		return ""
	}
	path := strings.SplitN(strings.TrimPrefix(field, "steps."), ".", 2)
	if len(path) != 2 {
		return ""
	}
	switch path[1] {
	case "status", "exit_code", "output_contains", "validation":
		return path[0]
	default:
		return ""
	}
}

func stepFilePath(s *WorkflowStep) string {
	if s.Tool == "guarded_replace" {
		if p, ok := s.Args["path"].(string); ok {
			return p
		}
	}
	return ""
}

func stepFilePaths(s *WorkflowStep) []string {
	if p := stepFilePath(s); p != "" {
		return []string{p}
	}
	if s.Tool == "git_commit_owned" {
		if files, ok := s.Args["files"].([]any); ok {
			paths := make([]string, 0, len(files))
			for _, f := range files {
				if fs, ok := f.(string); ok {
					paths = append(paths, fs)
				}
			}
			return paths
		}
	}
	return nil
}

func readsChangedSet(s *WorkflowStep) bool {
	if s.Tool == "git_commit_owned" {
		return true
	}
	return strings.Contains(s.If, "changed_files")
}

type fileLockSet struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newFileLockSet() *fileLockSet {
	return &fileLockSet{locks: map[string]*sync.Mutex{}}
}

func (s *fileLockSet) lock(paths []string) {
	if len(paths) == 0 {
		return
	}
	sorted := make([]string, len(paths))
	copy(sorted, paths)
	sort.Strings(sorted)
	s.mu.Lock()
	for _, p := range sorted {
		l, ok := s.locks[p]
		if !ok {
			l = &sync.Mutex{}
			s.locks[p] = l
		}
		s.mu.Unlock()
		l.Lock()
		s.mu.Lock()
	}
	s.mu.Unlock()
}

func (s *fileLockSet) unlock(paths []string) {
	s.mu.Lock()
	for i := len(paths) - 1; i >= 0; i-- {
		if l, ok := s.locks[paths[i]]; ok {
			l.Unlock()
		}
	}
	s.mu.Unlock()
}

func (r *Runner) executeWorkflowStep(ctx context.Context, repoPath string, step WorkflowStep, changedSet map[string]struct{}, fileHashes map[string]string, topLevelCommit *WorkflowCommit) (WorkflowStepResult, error) {
	base := WorkflowStepResult{ID: step.ID, Tool: step.Tool, Status: "ok"}
	switch step.Tool {
	case "guarded_replace":
		var args WorkflowEdit
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		replaceReq := fileops.ReplaceRequest{RepoPath: repoPath, Path: args.Path, ExpectedHash: args.ExpectedHash, Old: args.Old, New: args.New}
		if currentHash, ok := fileHashes[args.Path]; ok {
			replaceReq.ExpectedHash = currentHash
		}
		if replaceReq.ExpectedHash == "" {
			snapshot, err := fileops.ReadSnapshotInRepo(repoPath, args.Path)
			if err != nil {
				return WorkflowStepResult{}, err
			}
			if !snapshot.Exists {
				base.Status = "conflict"
				base.Reason = "file does not exist: " + args.Path
				return base, nil
			}
			replaceReq.ExpectedHash = snapshot.Hash
		}
		editResult, err := fileops.ApplyGuardedReplace(replaceReq)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Status = editResult.Status
		base.Reason = editResult.Reason
		base.Output = editResult
		if editResult.Changed {
			changedSet[args.Path] = struct{}{}
			fileHashes[args.Path] = editResult.NewHash
		}
		return base, nil
	case "command":
		var args WorkflowCommand
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		checkResult, err := r.commands.RunInRepo(ctx, args.Command, repoPath, args.CWD, args.TimeoutSeconds)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Output = checkResult
		if checkResult.ExitCode != 0 {
			base.Status = "failed"
			base.Reason = "command failed: " + args.Command
		}
		return base, nil
	case "git_commit_owned":
		var args WorkflowCommit
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		files := args.Files
		if len(files) == 0 {
			files = sortedKeys(changedSet)
		}
		if len(files) == 0 && topLevelCommit != nil {
			files = topLevelCommit.Files
		}
		message := args.Message
		if message == "" && topLevelCommit != nil {
			message = topLevelCommit.Message
		}
		commitResult, err := gitops.CommitOwned(ctx, gitops.CommitRequest{RepoPath: repoPath, Files: files, Message: message})
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Status = commitResult.Status
		base.Reason = commitResult.Reason
		base.Output = commitResult
		return base, nil
	case "git_prepare_task_worktree":
		var args gitops.PrepareTaskWorktreeRequest
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		args.RepoPath = repoPath
		worktreeResult, err := gitops.PrepareTaskWorktree(ctx, args)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Status = worktreeResult.Status
		base.Reason = worktreeResult.Reason
		base.Output = worktreeResult
		return base, nil
	case "task_batch_upsert":
		if !r.cfg.LayerEnabled("tasks") {
			base.Status = "failed"
			base.Reason = "task layer is disabled"
			return base, nil
		}
		var args tasks.BatchUpsertRequest
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		args.RepoPath = repoPath
		taskResult, err := r.taskBackend.BatchUpsert(ctx, args)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Output = taskResult
		return base, nil
	case "task_transition":
		if !r.cfg.LayerEnabled("tasks") {
			base.Status = "failed"
			base.Reason = "task layer is disabled"
			return base, nil
		}
		var args WorkflowTaskTransition
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		updated, err := r.transitionTasks(ctx, repoPath, args)
		if err != nil {
			base.Status = "failed"
			base.Reason = err.Error()
			return base, nil
		}
		base.Output = updated
		return base, nil
	default:
		base.Status = "failed"
		base.Reason = "unknown workflow tool: " + step.Tool
		return base, nil
	}
}

func (r *Runner) transitionTasks(ctx context.Context, repoPath string, req WorkflowTaskTransition) ([]tasks.Task, error) {
	to := strings.TrimSpace(req.To)
	if to == "" {
		return nil, errors.New("to status is required")
	}
	if len(req.TaskIDs) == 0 {
		return nil, errors.New("task_ids is required")
	}
	if err := r.validateTaskGraph(ctx, repoPath); err != nil {
		return nil, err
	}

	current := make([]tasks.Task, 0, len(req.TaskIDs))
	for _, id := range req.TaskIDs {
		taskID := strings.TrimSpace(id)
		if taskID == "" {
			return nil, errors.New("task id is empty")
		}
		task, err := r.taskBackend.Get(ctx, repoPath, taskID)
		if err != nil {
			return nil, err
		}
		if req.From != "" && task.Status != req.From {
			return nil, fmt.Errorf("task %s status is %q, want %q", task.ID, task.Status, req.From)
		}
		if strings.HasPrefix(task.ID, "goal-") && isClosingTaskStatus(to) {
			return nil, fmt.Errorf("goal task %s cannot transition to %s", task.ID, to)
		}
		current = append(current, task)
	}

	updated := make([]tasks.Task, 0, len(current))
	for _, task := range current {
		item, err := r.taskBackend.SetStatus(ctx, tasks.StatusRequest{RepoPath: repoPath, ID: task.ID, Status: to})
		if err != nil {
			return nil, err
		}
		updated = append(updated, item)
	}
	return updated, nil
}

func (r *Runner) validateTaskGraph(ctx context.Context, repoPath string) error {
	items, err := r.taskBackend.List(ctx, repoPath)
	if err != nil {
		return err
	}
	byID := make(map[string]tasks.Task, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	for _, item := range items {
		seen := map[string]struct{}{item.ID: {}}
		parentID := strings.TrimSpace(item.ParentID)
		for parentID != "" {
			parent, ok := byID[parentID]
			if !ok {
				return fmt.Errorf("task %s parent %s does not exist", item.ID, parentID)
			}
			if _, ok := seen[parent.ID]; ok {
				return fmt.Errorf("task parent cycle detected at %s", parent.ID)
			}
			seen[parent.ID] = struct{}{}
			parentID = strings.TrimSpace(parent.ParentID)
		}
	}
	return nil
}

func isClosingTaskStatus(status string) bool {
	switch status {
	case "done", "verified", "rejected", "superseded":
		return true
	default:
		return false
	}
}

func bindStepArgs(args map[string]any, target any) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func (r *Runner) evalStepCondition(repoPath string, condition string, result WorkflowResult, steps map[string]WorkflowStepResult) bool {
	condition = strings.TrimSpace(condition)
	if condition == "" || condition == "always" {
		return true
	}
	return r.evalConditionOr(repoPath, condition, result, steps)
}

func (r *Runner) evalConditionOr(repoPath string, condition string, result WorkflowResult, steps map[string]WorkflowStepResult) bool {
	for _, part := range splitConditionExpression(condition, "||") {
		if r.evalConditionAnd(repoPath, part, result, steps) {
			return true
		}
	}
	return false
}

func (r *Runner) evalConditionAnd(repoPath string, condition string, result WorkflowResult, steps map[string]WorkflowStepResult) bool {
	parts := splitConditionExpression(condition, "&&")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if !r.evalConditionNot(repoPath, part, result, steps) {
			return false
		}
	}
	return true
}

func (r *Runner) evalConditionNot(repoPath string, condition string, result WorkflowResult, steps map[string]WorkflowStepResult) bool {
	condition = strings.TrimSpace(condition)
	negated := false
	for strings.HasPrefix(condition, "!") {
		negated = !negated
		condition = strings.TrimSpace(strings.TrimPrefix(condition, "!"))
	}
	if condition == "" {
		return false
	}
	value := r.evalConditionAtom(repoPath, condition, result, steps)
	if negated {
		return !value
	}
	return value
}

func splitConditionExpression(condition string, op string) []string {
	fields := strings.Fields(condition)
	parts := []string{}
	current := []string{}
	for _, field := range fields {
		if field == op {
			part := strings.TrimSpace(strings.Join(current, " "))
			if part != "" {
				parts = append(parts, part)
			}
			current = nil
			continue
		}
		current = append(current, field)
	}
	part := strings.TrimSpace(strings.Join(current, " "))
	if part != "" {
		parts = append(parts, part)
	}
	return parts
}

func (r *Runner) evalConditionAtom(repoPath string, condition string, result WorkflowResult, steps map[string]WorkflowStepResult) bool {
	fields := strings.Fields(condition)
	if len(fields) == 0 {
		return false
	}
	switch {
	case fields[0] == "changed_files_count" && len(fields) == 3:
		return compareInt(len(result.ChangedFiles), fields[1], fields[2])
	case fields[0] == "changed_files" && len(fields) == 3 && fields[1] == "contains":
		return changedFilesContain(result.ChangedFiles, fields[2])
	case fields[0] == "file_exists" && len(fields) == 2:
		return workflowFileExists(repoPath, fields[1])
	case fields[0] == "file_missing" && len(fields) == 2:
		return !workflowFileExists(repoPath, fields[1])
	case strings.HasPrefix(fields[0], "steps."):
		return evalStepResultCondition(fields, steps)
	case strings.HasPrefix(fields[0], "tasks."):
		return r.evalTaskCondition(repoPath, fields)
	default:
		return false
	}
}

func changedFilesContain(files []string, path string) bool {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if clean == "" || clean == "." {
		return false
	}
	for _, file := range files {
		if filepath.ToSlash(filepath.Clean(file)) == clean {
			return true
		}
	}
	return false
}

func evalStepResultCondition(fields []string, steps map[string]WorkflowStepResult) bool {
	path := strings.SplitN(strings.TrimPrefix(fields[0], "steps."), ".", 2)
	if len(path) != 2 {
		return false
	}
	step, ok := steps[path[0]]
	if !ok {
		return false
	}
	switch path[1] {
	case "status":
		return len(fields) == 3 && compareString(step.Status, fields[1], fields[2])
	case "exit_code":
		cmd, ok := step.Output.(command.Result)
		return ok && len(fields) == 3 && compareInt(cmd.ExitCode, fields[1], fields[2])
	case "output_contains":
		cmd, ok := step.Output.(command.Result)
		return ok && len(fields) >= 2 && commandOutputContains(cmd, strings.Join(fields[1:], " "))
	case "validation":
		return len(fields) == 3 && compareString(stepValidationStatus(step), fields[1], fields[2])
	default:
		return false
	}
}

func stepValidationStatus(step WorkflowStepResult) string {
	cmd, ok := step.Output.(command.Result)
	if !ok {
		return "unavailable"
	}
	if len(cmd.EvidenceLines) == 0 {
		return "INSUFFICIENT_DATA"
	}
	return "ok"
}

func (r *Runner) evalTaskCondition(repoPath string, fields []string) bool {
	if len(fields) != 3 || !r.cfg.LayerEnabled("tasks") {
		return false
	}
	path := strings.SplitN(strings.TrimPrefix(fields[0], "tasks."), ".", 2)
	if len(path) != 2 || path[1] != "status" {
		return false
	}
	task, err := r.tasks.Get(tasks.GetRequest{RepoPath: repoPath, ID: path[0]})
	return err == nil && compareString(task.Status, fields[1], fields[2])
}

func compareString(left string, op string, right string) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	default:
		return false
	}
}

func workflowFileExists(repoPath string, relPath string) bool {
	path, ok := workflowConditionPath(repoPath, relPath)
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func workflowConditionPath(repoPath string, relPath string) (string, bool) {
	if strings.TrimSpace(repoPath) == "" || strings.TrimSpace(relPath) == "" || filepath.IsAbs(relPath) {
		return "", false
	}
	repoAbs, err := filepath.Abs(repoPath)
	if err != nil {
		return "", false
	}
	pathAbs, err := filepath.Abs(filepath.Join(repoAbs, filepath.Clean(relPath)))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(repoAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return pathAbs, true
}

func compareInt(left int, op string, rightText string) bool {
	right, err := strconv.Atoi(rightText)
	if err != nil {
		return false
	}
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func commandOutputContains(result command.Result, needle string) bool {
	if needle == "" {
		return false
	}
	for _, line := range result.StdoutTail {
		if strings.Contains(line, needle) {
			return true
		}
	}
	for _, line := range result.StderrTail {
		if strings.Contains(line, needle) {
			return true
		}
	}
	for _, line := range result.FilteredLines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	for _, line := range result.EvidenceLines {
		if strings.Contains(line.Text, needle) {
			return true
		}
	}
	return false
}

func commitPtr(c WorkflowCommit) *WorkflowCommit {
	if c.Enabled || len(c.Files) > 0 || c.Message != "" {
		return &c
	}
	return nil
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Run executes the command-analysis pipeline.

func (r *Runner) updateTaskStatus(ctx context.Context, taskID string, status string, repoPath string) error {
	if taskID == "" || status == "" {
		return nil
	}
	if !r.cfg.LayerEnabled("tasks") {
		return fmt.Errorf("task layer is disabled")
	}
	_, err := r.taskBackend.SetStatus(ctx, tasks.StatusRequest{
		RepoPath: repoPath,
		ID:       taskID,
		Status:   status,
	})
	return err
}

func taskStatusOrDefault(configured string, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

func pipelineTaskCloseoutSucceeded(result Result, err error) bool {
	return err == nil && result.Status == "ok" && result.Command.ExitCode == 0 && result.Validation.Valid
}

func workflowTaskCloseoutSucceeded(req WorkflowRequest, result WorkflowResult, err error) bool {
	if err != nil || result.Status != "ok" {
		return false
	}
	for _, step := range result.StepResults {
		if step.Status != "ok" {
			return false
		}
		switch step.Tool {
		case "command":
			check, ok := step.Output.(command.Result)
			if !ok || check.ExitCode != 0 {
				return false
			}
		case "git_commit_owned":
			commit, ok := step.Output.(gitops.CommitResult)
			if !ok || commit.Status != "ok" {
				return false
			}
		}
	}
	for _, check := range result.CheckResults {
		if check.ExitCode != 0 {
			return false
		}
	}
	if req.Commit.Enabled {
		return result.CommitResult != nil && result.CommitResult.Status == "ok"
	}
	return true
}

func (r *Runner) Run(ctx context.Context, req Request) (result Result, err error) {
	if err := r.updateTaskStatus(ctx, req.CurrentTaskID, taskStatusOrDefault(req.TaskOnStart, "in_progress"), req.RepoPath); err != nil {
		return Result{}, err
	}
	defer func() {
		finalStatus := taskStatusOrDefault(req.TaskOnSuccess, "done")
		if !pipelineTaskCloseoutSucceeded(result, err) {
			finalStatus = taskStatusOrDefault(req.TaskOnFailure, "blocked")
		}
		if updateErr := r.updateTaskStatus(ctx, req.CurrentTaskID, finalStatus, req.RepoPath); updateErr != nil && err == nil {
			err = updateErr
		}
	}()

	cmdResult, err := r.commands.RunInRepo(ctx, req.Command, req.RepoPath, req.CWD, req.TimeoutSeconds)
	if err != nil {
		return Result{}, err
	}
	summary := evidence.Summary{EvidenceLines: cmdResult.EvidenceLines, Truncated: cmdResult.Truncated}
	analysis := deterministicAnalysis(req.Task, cmdResult)

	if req.Analyze && r.chatClient != nil && len(r.cfg.Models) > 0 {
		modelID := req.ModelID
		if modelID == "" {
			modelID = r.cfg.Routing["evidence_analysis"]
		}
		model, ok := r.cfg.Models[modelID]
		if ok {
			resp, err := r.chatClient.Complete(ctx, provider.ChatRequest{
				ProviderID:      model.Provider,
				ModelID:         modelID,
				Model:           model.Model,
				SystemPrompt:    model.Prompt(),
				UserPrompt:      buildAnalysisPrompt(req.Task, cmdResult, summary),
				MaxOutputTokens: model.MaxOutputTokens,
				Temperature:     model.Temperature,
			})
			if err == nil {
				analysis = strings.TrimSpace(resp.Text)
			}
		}
	}

	validation := evidence.ValidateLinks(summary, analysis, r.cfg.PipelinePolicy.RequireEvidenceForAnalysis)
	status := "ok"
	if !validation.Valid {
		status = "INSUFFICIENT_DATA"
	}
	compact := req.CompactOutput == nil || *req.CompactOutput
	handoff := composeHandoff(status, cmdResult, summary, analysis, validation, r.cfg.PipelinePolicy.MaxReturnChars)
	if compact && status == "ok" && cmdResult.ExitCode == 0 {
		handoff = compactHandoff(status, cmdResult)
	}
	return Result{Status: status, Compact: compact, Command: cmdResult, Summary: summary, Analysis: analysis, Validation: validation, Handoff: handoff}, nil
}

func deterministicAnalysis(_ string, result command.Result) string {
	if len(result.EvidenceLines) == 0 {
		return "INSUFFICIENT_DATA: no evidence lines extracted"
	}
	if result.ExitCode != 0 {
		return fmt.Sprintf("Command failed with exit code %d [%s].", result.ExitCode, result.EvidenceLines[0].ID)
	}
	return fmt.Sprintf("Command completed successfully with relevant evidence [%s].", result.EvidenceLines[0].ID)
}

func buildAnalysisPrompt(task string, result command.Result, summary evidence.Summary) string {
	var b strings.Builder
	b.WriteString("Task:\n")
	b.WriteString(task)
	b.WriteString("\n\nCommand:\n")
	b.WriteString(result.Command)
	fmt.Fprintf(&b, "\n\nExit code: %d", result.ExitCode)
	b.WriteString("\n\nEvidence:\n")
	for _, line := range summary.EvidenceLines {
		b.WriteString(line.ID)
		b.WriteString(": ")
		b.WriteString(line.Text)
		b.WriteString("\n")
	}
	b.WriteString("\nReturn concise analysis. Cite evidence ids like [E1]. If evidence is insufficient, return INSUFFICIENT_DATA.")
	return b.String()
}

func compactHandoff(status string, result command.Result) string {
	var b strings.Builder
	b.WriteString("status: ")
	b.WriteString(status)
	fmt.Fprintf(&b, "\ncommand_id: %s", result.CommandID)
	fmt.Fprintf(&b, "\nexit_code: %d", result.ExitCode)
	// Include evidence lines even in compact mode — they are short by definition.
	if len(result.EvidenceLines) > 0 {
		b.WriteString("\nevidence:\n")
		for _, line := range result.EvidenceLines {
			b.WriteString("- [")
			b.WriteString(line.ID)
			b.WriteString("] ")
			b.WriteString(line.Text)
			b.WriteString("\n")
		}
	}
	if len(result.StdoutTail) > 0 || len(result.StderrTail) > 0 {
		b.WriteString("output: collapsed; use filter_command_history with command_id for details")
	}
	return b.String()
}

func composeHandoff(status string, result command.Result, summary evidence.Summary, analysis string, validation evidence.Validation, maxChars int) string {
	var b strings.Builder
	b.WriteString("status: ")
	b.WriteString(status)
	fmt.Fprintf(&b, "\nexit_code: %d", result.ExitCode)
	b.WriteString("\nanalysis: ")
	b.WriteString(strings.TrimSpace(analysis))
	b.WriteString("\nevidence:\n")
	for _, line := range summary.EvidenceLines {
		b.WriteString("- [")
		b.WriteString(line.ID)
		b.WriteString("] ")
		b.WriteString(line.Text)
		b.WriteString("\n")
	}
	if !validation.Valid {
		b.WriteString("validation_problems:\n")
		for _, problem := range validation.Problems {
			b.WriteString("- ")
			b.WriteString(problem)
			b.WriteString("\n")
		}
	}
	out := b.String()
	if maxChars > 0 && len(out) > maxChars {
		return out[:maxChars] + "\n[truncated]"
	}
	return out
}
