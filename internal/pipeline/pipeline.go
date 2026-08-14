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

	"github.com/alvnukov/mcp-ai-helper/internal/command"
	"github.com/alvnukov/mcp-ai-helper/internal/config"
	"github.com/alvnukov/mcp-ai-helper/internal/evidence"
	"github.com/alvnukov/mcp-ai-helper/internal/fileops"
	"github.com/alvnukov/mcp-ai-helper/internal/gitops"
	"github.com/alvnukov/mcp-ai-helper/internal/provider"
	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
	"github.com/alvnukov/mcp-ai-helper/internal/webfetch"
)

// Runner executes analysis and edit workflows against a repository.
type Runner struct {
	cfg         *config.Config
	commands    *command.Runner
	chatClient  provider.ChatClient
	taskBackend TaskBackend
}

// TaskBackend is the task persistence surface used by workflow steps.
type TaskBackend interface {
	Get(ctx context.Context, repoPath string, id string) (tasks.Task, error)
	List(ctx context.Context, repoPath string) ([]tasks.Task, error)
	SetStatus(ctx context.Context, req tasks.StatusRequest) (tasks.Task, error)
	BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error)
}

// TaskStatusMutation preserves the repository files changed by a task status update.
type TaskStatusMutation struct {
	Task         tasks.Task
	ChangedFiles []string
}

type taskStatusMutationBackend interface {
	SetStatusWithResult(ctx context.Context, req tasks.StatusRequest) (TaskStatusMutation, error)
}

// Request describes the legacy command-analysis pipeline input.
type Request struct {
	CurrentTaskID  string   `json:"current_task_id,omitempty"`
	TaskOnStart    string   `json:"task_on_start,omitempty"`
	TaskOnSuccess  string   `json:"task_on_success,omitempty"`
	TaskOnFailure  string   `json:"task_on_failure,omitempty"`
	Command        string   `json:"command"`
	RepoPath       string   `json:"repo_path"`
	CWD            string   `json:"cwd"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MCPWaitSeconds int      `json:"mcp_wait_seconds"`
	Analyze        bool     `json:"analyze"`
	Task           string   `json:"task"`
	ModelID        string   `json:"model_id"`
	CompactOutput  *bool    `json:"compact_output,omitempty"`
	SecretHandles  []string `json:"secret_handles,omitempty"`
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

// MarshalJSON emits a compact success envelope when compact output is enabled.
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
	SecretHandles []string          `json:"secret_handles,omitempty"`
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

// WorkflowEdit describes one guarded text replacement. OldB64 and NewB64 carry
// the same text base64-encoded, for spans that are awkward to quote in JSON;
// a workflow step accepts them exactly as edit action=replace does, so a caller
// need not remember which of the two surfaces it is writing for.
type WorkflowEdit struct {
	Path         string `json:"path"`
	ExpectedHash string `json:"expected_hash"`
	Old          string `json:"old"`
	New          string `json:"new"`
	OldB64       string `json:"old_b64,omitempty"`
	NewB64       string `json:"new_b64,omitempty"`
}

// replaceRequest builds the replacement both workflow forms apply. Every
// argument a caller can set travels through here, so the legacy edits list and
// the guarded_replace step cannot drift apart over which ones they honour.
func (e WorkflowEdit) replaceRequest(repoPath string) fileops.ReplaceRequest {
	return fileops.ReplaceRequest{
		RepoPath:     repoPath,
		Path:         e.Path,
		ExpectedHash: e.ExpectedHash,
		Old:          e.Old,
		New:          e.New,
		OldB64:       e.OldB64,
		NewB64:       e.NewB64,
	}
}

// WorkflowWriteFile describes one whole-file create or replacement.
type WorkflowWriteFile struct {
	Path         string `json:"path"`
	Content      string `json:"content,omitempty"`
	ContentB64   string `json:"content_b64,omitempty"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	Mode         int    `json:"mode,omitempty"`
}

func (w WorkflowWriteFile) writeRequest(repoPath string) fileops.WriteFileRequest {
	return fileops.WriteFileRequest{RepoPath: repoPath, Path: w.Path, Content: w.Content, ContentB64: w.ContentB64, ExpectedHash: w.ExpectedHash, Mode: w.Mode}
}

// WorkflowCommand describes one repo-scoped command check.
type WorkflowCommand struct {
	Command        string         `json:"command"`
	CWD            string         `json:"cwd"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	MCPWaitSeconds int            `json:"mcp_wait_seconds"`
	Filter         command.Filter `json:"filter"`
	WebDocID       string         `json:"web_doc_id,omitempty"`
	WebDocSource   string         `json:"web_doc_source,omitempty"`
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
	EditResults  []fileops.ReplaceResult `json:"edit_results,omitempty"`
	CheckResults []command.Result        `json:"check_results,omitempty"`
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
	return &Runner{cfg: cfg, commands: command.NewRunnerWithMask(cfg.CommandPolicy, cfg.SecretMask()), chatClient: chatClient, taskBackend: taskBackend}
}

func (r *Runner) requireTaskBackend() TaskBackend {
	if r.taskBackend != nil {
		return r.taskBackend
	}
	return missingTaskBackend{}
}

type missingTaskBackend struct{}

func (missingTaskBackend) Get(context.Context, string, string) (tasks.Task, error) {
	return tasks.Task{}, errors.New("task backend is required; legacy task file store is disabled")
}

func (missingTaskBackend) List(context.Context, string) ([]tasks.Task, error) {
	return nil, errors.New("task backend is required; legacy task file store is disabled")
}

func (missingTaskBackend) SetStatus(context.Context, tasks.StatusRequest) (tasks.Task, error) {
	return tasks.Task{}, errors.New("task backend is required; legacy task file store is disabled")
}

func (missingTaskBackend) BatchUpsert(context.Context, tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error) {
	return tasks.BatchUpsertResult{}, errors.New("task backend is required; legacy task file store is disabled")
}

func (r *Runner) withWebArtifact(args WorkflowCommand) (WorkflowCommand, error) {
	if strings.TrimSpace(args.WebDocID) == "" {
		return args, nil
	}
	source := strings.TrimSpace(args.WebDocSource)
	if source == "" {
		source = "normalized"
	}
	path, err := webfetch.ArtifactPath(r.cfg.WebPolicy, args.WebDocID, source)
	if err != nil {
		return WorkflowCommand{}, err
	}
	args.Command = "HELPER_WEB_DOC_PATH=" + strconv.Quote(path) + "; export HELPER_WEB_DOC_PATH; " + args.Command
	return args, nil
}

// prepareWorkflow validates all request-wide preconditions before lifecycle mutation.
func (r *Runner) prepareWorkflow(ctx context.Context, req WorkflowRequest) (context.Context, [][]WorkflowStep, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return ctx, nil, errors.New("repo_path is required")
	}
	if len(req.SecretHandles) > 0 {
		envs, mask, err := r.cfg.ResolveSecretEnv(req.SecretHandles)
		if err != nil {
			return ctx, nil, err
		}
		ctx = command.ContextWithSecrets(ctx, envs, mask)
	}
	if err := validateEditEncodings(req); err != nil {
		return ctx, nil, err
	}
	waves, err := buildStepWaves(req.Steps)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, waves, nil
}

// validateEditEncodings decodes every base64 edit argument in the request before
// any of the request runs. ApplyGuardedReplace rejects a malformed encoding as
// well, but by the time the tenth step reaches it the first nine have already
// written; a request that cannot be decoded must fail before it changes
// anything.
func validateEditEncodings(req WorkflowRequest) error {
	for i, edit := range req.Edits {
		if err := fileops.ValidateReplaceEncoding(edit.replaceRequest(req.RepoPath)); err != nil {
			return fmt.Errorf("edits[%d] (%s): %w", i, edit.Path, err)
		}
	}
	for _, step := range req.Steps {
		switch step.Tool {
		case "guarded_replace":
			var args WorkflowEdit
			if err := bindStepArgs(step.Args, &args); err != nil {
				return fmt.Errorf("workflow step %q: %w", step.ID, err)
			}
			if err := fileops.ValidateReplaceEncoding(args.replaceRequest(req.RepoPath)); err != nil {
				return fmt.Errorf("workflow step %q: %w", step.ID, err)
			}
		case "write_file":
			var args WorkflowWriteFile
			if err := bindStepArgs(step.Args, &args); err != nil {
				return fmt.Errorf("workflow step %q: %w", step.ID, err)
			}
			if err := fileops.ValidateWriteFileEncoding(args.writeRequest(req.RepoPath)); err != nil {
				return fmt.Errorf("workflow step %q: %w", step.ID, err)
			}
		}
	}
	return nil
}

// RunWorkflow executes either the stable steps DSL or the legacy edit/check/commit workflow.
func (r *Runner) RunWorkflow(ctx context.Context, req WorkflowRequest) (result WorkflowResult, err error) {
	ctx, waves, err := r.prepareWorkflow(ctx, req)
	if err != nil {
		return WorkflowResult{}, err
	}
	lifecycleChangedFiles, err := r.updateTaskStatusWithChanges(ctx, req.CurrentTaskID, taskStatusOrDefault(req.TaskOnStart, "in_progress"), req.RepoPath)
	if err != nil {
		return WorkflowResult{}, err
	}
	defer func() {
		finalStatus := taskStatusOrDefault(req.TaskOnSuccess, "done")
		if result.Status == "running" {
			finalStatus = taskStatusOrDefault(req.TaskOnStart, "in_progress")
		} else if !workflowTaskCloseoutSucceeded(req, result, err) {
			finalStatus = taskStatusOrDefault(req.TaskOnFailure, "blocked")
		}
		changedFiles, updateErr := r.updateTaskStatusWithChanges(ctx, req.CurrentTaskID, finalStatus, req.RepoPath)
		lifecycleChangedFiles = mergeChangedFiles(lifecycleChangedFiles, changedFiles)
		result.ChangedFiles = mergeChangedFiles(result.ChangedFiles, lifecycleChangedFiles)
		if updateErr != nil && err == nil {
			err = updateErr
		}
	}()

	if len(req.Steps) > 0 {
		return r.runWorkflowSteps(ctx, req, waves), nil
	}
	result = WorkflowResult{Status: "ok"}
	changedSet := map[string]struct{}{}
	for _, edit := range req.Edits {
		replaceReq := edit.replaceRequest(req.RepoPath)
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
		check, err := r.withWebArtifact(check)
		if err != nil {
			return WorkflowResult{}, err
		}
		checkResult, err := r.commands.RunFilteredInRepoWithWait(ctx, check.Command, req.RepoPath, check.CWD, check.TimeoutSeconds, check.MCPWaitSeconds, check.Filter)
		if err != nil {
			return WorkflowResult{}, err
		}
		result.CheckResults = append(result.CheckResults, checkResult)
		if checkResult.Status == "running" {
			result.Status = "running"
			result.Reason = "check still running: " + check.Command
			return result, nil
		}
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

func (r *Runner) runWorkflowSteps(ctx context.Context, req WorkflowRequest, waves [][]WorkflowStep) WorkflowResult {
	result := WorkflowResult{Status: "ok"}
	stepResults := map[string]WorkflowStepResult{}
	files := newWorkflowFiles()

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
				sr, execErr := r.executeWorkflowStep(ctx, req.RepoPath, *step, files, commitPtr(req.Commit))
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
				result.ChangedFiles = files.changedFiles()
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

	result.ChangedFiles = files.changedFiles()
	return result
}

// buildStepWaves validates and topologically sorts steps into parallel-execution waves.
func buildStepWaves(steps []WorkflowStep) ([][]WorkflowStep, error) {
	if len(steps) == 0 {
		return nil, nil
	}

	indices := make(map[string]int, len(steps))
	for i, step := range steps {
		id := strings.TrimSpace(step.ID)
		if id == "" {
			continue
		}
		if _, exists := indices[id]; exists {
			return nil, fmt.Errorf("duplicate workflow step id %q", id)
		}
		indices[id] = i
	}

	reverse := make([][]int, len(steps))
	inDegree := make([]int, len(steps))
	seenDependencies := make([]map[int]struct{}, len(steps))
	addDependency := func(stepIndex, dependencyIndex int) {
		if seenDependencies[stepIndex] == nil {
			seenDependencies[stepIndex] = map[int]struct{}{}
		}
		if _, exists := seenDependencies[stepIndex][dependencyIndex]; exists {
			return
		}
		seenDependencies[stepIndex][dependencyIndex] = struct{}{}
		inDegree[stepIndex]++
		reverse[dependencyIndex] = append(reverse[dependencyIndex], stepIndex)
	}

	var editIndices []int
	for i, step := range steps {
		if isWorkflowFileEdit(&step) {
			editIndices = append(editIndices, i)
		}
	}

	// Explicit and condition dependencies define caller intent and must be
	// complete before implicit serialization edges are considered.
	for i, step := range steps {
		for _, rawDependencyID := range step.DependsOn {
			dependencyID := strings.TrimSpace(rawDependencyID)
			dependencyIndex, exists := indices[dependencyID]
			if !exists {
				return nil, fmt.Errorf("workflow step %q depends on unknown step %q", step.ID, rawDependencyID)
			}
			addDependency(i, dependencyIndex)
		}
		for _, dependencyID := range parseStepConditionDeps(step.If) {
			dependencyIndex, exists := indices[dependencyID]
			if !exists {
				return nil, fmt.Errorf("workflow step %q condition references unknown step %q", step.ID, dependencyID)
			}
			addDependency(i, dependencyIndex)
		}
	}

	dependencyPathExists := func(from, to int) bool {
		visited := make([]bool, len(steps))
		stack := []int{from}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if current == to {
				return true
			}
			if visited[current] {
				continue
			}
			visited[current] = true
			stack = append(stack, reverse[current]...)
		}
		return false
	}

	// Serialize edits to the same file unless the caller already ordered the
	// pair in the opposite direction through explicit dependencies.
	for i, step := range steps {
		if !isWorkflowFileEdit(&step) {
			continue
		}
		path := stepFilePath(&step)
		for j := i - 1; j >= 0 && path != ""; j-- {
			if !isWorkflowFileEdit(&steps[j]) || stepFilePath(&steps[j]) != path {
				continue
			}
			if !dependencyPathExists(i, j) {
				addDependency(i, j)
			}
			break
		}
	}

	// Steps that inspect changed files must observe every edit result.
	for i, step := range steps {
		if !readsChangedSet(&step) {
			continue
		}
		for _, editIndex := range editIndices {
			if editIndex != i {
				addDependency(i, editIndex)
			}
		}
	}

	var waves [][]WorkflowStep
	processed := 0
	for processed < len(steps) {
		var wave []WorkflowStep
		var waveIndices []int
		for i := range steps {
			if inDegree[i] != 0 {
				continue
			}
			wave = append(wave, steps[i])
			waveIndices = append(waveIndices, i)
			inDegree[i] = -1
			processed++
		}
		if len(wave) == 0 {
			unresolved := make([]string, 0, len(steps)-processed)
			for i, degree := range inDegree {
				if degree <= 0 {
					continue
				}
				label := strings.TrimSpace(steps[i].ID)
				if label == "" {
					label = fmt.Sprintf("#%d", i+1)
				}
				unresolved = append(unresolved, label)
			}
			sort.Strings(unresolved)
			return nil, fmt.Errorf("workflow dependency cycle detected among steps: %s", strings.Join(unresolved, ", "))
		}
		for _, stepIndex := range waveIndices {
			for _, dependentIndex := range reverse[stepIndex] {
				if inDegree[dependentIndex] > 0 {
					inDegree[dependentIndex]--
				}
			}
		}
		waves = append(waves, wave)
	}
	return waves, nil
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

func isWorkflowFileEdit(s *WorkflowStep) bool {
	return s.Tool == "guarded_replace" || s.Tool == "write_file"
}

func stepFilePath(s *WorkflowStep) string {
	if isWorkflowFileEdit(s) {
		if p, ok := s.Args["path"].(string); ok {
			return normalizeStepPath(p)
		}
	}
	return ""
}

// normalizeStepPath keys same-file serialization, locking, and the hash
// chain on the cleaned path, so "a.go" and "./a.go" are one file rather
// than two concurrent writers.
func normalizeStepPath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(trimmed))
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

// workflowFiles is what the steps of a workflow learn about each other's edits:
// which files have changed, and the hash each one now carries so a later guarded
// replace can chain onto it without re-reading the file.
//
// Steps inside a wave run concurrently and the file locks only serialise the
// ones touching the same path, so two steps editing different files reach this
// at the same time. It carries its own lock rather than borrowing the caller's,
// because it is also read from the result-assembly path.
type workflowFiles struct {
	mu      sync.Mutex
	changed map[string]struct{}
	hashes  map[string]string
}

func newWorkflowFiles() *workflowFiles {
	return &workflowFiles{changed: map[string]struct{}{}, hashes: map[string]string{}}
}

// recordEdit notes that path now holds hash, and that the workflow changed it.
func (f *workflowFiles) recordEdit(path string, hash string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.changed[path] = struct{}{}
	f.hashes[path] = hash
}

func (f *workflowFiles) recordChanged(paths ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, path := range paths {
		if path != "" {
			f.changed[path] = struct{}{}
		}
	}
}

// hash reports the hash an earlier step left on path, if any.
func (f *workflowFiles) hash(path string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.hashes[path]
	return value, ok
}

// changedFiles lists the files edited so far, sorted so a commit built from them
// is reproducible.
func (f *workflowFiles) changedFiles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return sortedKeys(f.changed)
}

func (r *Runner) executeWorkflowStep(ctx context.Context, repoPath string, step WorkflowStep, files *workflowFiles, topLevelCommit *WorkflowCommit) (WorkflowStepResult, error) {
	base := WorkflowStepResult{ID: step.ID, Tool: step.Tool, Status: "ok"}
	switch step.Tool {
	case "write_file":
		var args WorkflowWriteFile
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		path := normalizeStepPath(args.Path)
		writeReq := args.writeRequest(repoPath)
		if currentHash, ok := files.hash(path); ok {
			writeReq.ExpectedHash = currentHash
		}
		writeResult, err := fileops.WriteFile(writeReq)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Status = writeResult.Status
		base.Reason = writeResult.Reason
		base.Output = writeResult
		if writeResult.Changed {
			files.recordEdit(path, writeResult.NewHash)
		}
		return base, nil
	case "guarded_replace":
		var args WorkflowEdit
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		path := normalizeStepPath(args.Path)
		replaceReq := args.replaceRequest(repoPath)
		if currentHash, ok := files.hash(path); ok {
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
			files.recordEdit(path, editResult.NewHash)
		}
		return base, nil
	case "command":
		var args WorkflowCommand
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		args, err := r.withWebArtifact(args)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		checkResult, err := r.commands.RunInRepoWithWait(ctx, args.Command, repoPath, args.CWD, args.TimeoutSeconds, args.MCPWaitSeconds)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Output = checkResult
		if checkResult.Status == "running" {
			base.Status = "running"
			base.Reason = "command still running: " + args.Command
		} else if checkResult.ExitCode != 0 {
			base.Status = "failed"
			base.Reason = "command failed: " + args.Command
		}
		return base, nil
	case "git_commit_owned":
		var args WorkflowCommit
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		commitFiles := args.Files
		if len(commitFiles) == 0 && topLevelCommit != nil {
			commitFiles = topLevelCommit.Files
		}
		if len(commitFiles) == 0 {
			commitFiles = files.changedFiles()
		}
		message := args.Message
		if message == "" && topLevelCommit != nil {
			message = topLevelCommit.Message
		}
		commitResult, err := gitops.CommitOwned(ctx, gitops.CommitRequest{RepoPath: repoPath, Files: commitFiles, Message: message})
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
		taskResult, err := r.requireTaskBackend().BatchUpsert(ctx, args)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		files.recordChanged(taskResult.ChangedFiles...)
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
		mutation, err := r.transitionTasks(ctx, repoPath, args)
		if err != nil {
			base.Status = "failed"
			base.Reason = err.Error()
			return base, nil
		}
		files.recordChanged(mutation.ChangedFiles...)
		base.Output = mutation.Tasks
		return base, nil
	default:
		base.Status = "failed"
		base.Reason = "unknown workflow tool: " + step.Tool
		return base, nil
	}
}

type taskTransitionMutation struct {
	Tasks        []tasks.Task
	ChangedFiles []string
}

func (r *Runner) transitionTasks(ctx context.Context, repoPath string, req WorkflowTaskTransition) (taskTransitionMutation, error) {
	to := strings.TrimSpace(req.To)
	if to == "" {
		return taskTransitionMutation{}, errors.New("to status is required")
	}
	if len(req.TaskIDs) == 0 {
		return taskTransitionMutation{}, errors.New("task_ids is required")
	}
	if err := r.validateTaskGraph(ctx, repoPath); err != nil {
		return taskTransitionMutation{}, err
	}

	current := make([]tasks.Task, 0, len(req.TaskIDs))
	for _, id := range req.TaskIDs {
		taskID := strings.TrimSpace(id)
		if taskID == "" {
			return taskTransitionMutation{}, errors.New("task id is empty")
		}
		task, err := r.requireTaskBackend().Get(ctx, repoPath, taskID)
		if err != nil {
			return taskTransitionMutation{}, err
		}
		if req.From != "" && task.Status != req.From {
			return taskTransitionMutation{}, fmt.Errorf("task %s status is %q, want %q", task.ID, task.Status, req.From)
		}
		if strings.HasPrefix(task.ID, "goal-") && isClosingTaskStatus(to) {
			return taskTransitionMutation{}, fmt.Errorf("goal task %s cannot transition to %s", task.ID, to)
		}
		current = append(current, task)
	}

	result := taskTransitionMutation{Tasks: make([]tasks.Task, 0, len(current))}
	for _, task := range current {
		mutation, err := r.setTaskStatus(ctx, tasks.StatusRequest{RepoPath: repoPath, ID: task.ID, Status: to})
		if err != nil {
			if rollbackErr := r.rollbackTaskTransitions(ctx, repoPath, current[:len(result.Tasks)]); rollbackErr != nil {
				return taskTransitionMutation{}, fmt.Errorf("transition failed: %w; rollback also failed: %w", err, rollbackErr)
			}
			return taskTransitionMutation{}, fmt.Errorf("transition failed and was rolled back: %w", err)
		}
		result.Tasks = append(result.Tasks, mutation.Task)
		result.ChangedFiles = mergeChangedFiles(result.ChangedFiles, mutation.ChangedFiles)
	}
	return result, nil
}

// rollbackTaskTransitions restores tasks a partially failed transition
// already moved, so a retry passes the From guard again.
func (r *Runner) rollbackTaskTransitions(ctx context.Context, repoPath string, applied []tasks.Task) error {
	var problems []error
	for i := len(applied) - 1; i >= 0; i-- {
		if _, err := r.setTaskStatus(ctx, tasks.StatusRequest{RepoPath: repoPath, ID: applied[i].ID, Status: applied[i].Status}); err != nil {
			problems = append(problems, fmt.Errorf("restore %s to %s: %w", applied[i].ID, applied[i].Status, err))
		}
	}
	return errors.Join(problems...)
}

func (r *Runner) validateTaskGraph(ctx context.Context, repoPath string) error {
	items, err := r.requireTaskBackend().List(ctx, repoPath)
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
	if r.taskBackend == nil {
		return false
	}
	task, err := r.taskBackend.Get(context.Background(), repoPath, path[0])
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

func (r *Runner) setTaskStatus(ctx context.Context, req tasks.StatusRequest) (TaskStatusMutation, error) {
	if req.ID == "" || req.Status == "" {
		return TaskStatusMutation{}, nil
	}
	if !r.cfg.LayerEnabled("tasks") {
		return TaskStatusMutation{}, errors.New("task layer is disabled")
	}
	backend := r.requireTaskBackend()
	if mutationBackend, ok := backend.(taskStatusMutationBackend); ok {
		return mutationBackend.SetStatusWithResult(ctx, req)
	}
	task, err := backend.SetStatus(ctx, req)
	return TaskStatusMutation{Task: task}, err
}

func (r *Runner) updateTaskStatus(ctx context.Context, taskID string, status string, repoPath string) error {
	_, err := r.setTaskStatus(ctx, tasks.StatusRequest{RepoPath: repoPath, ID: taskID, Status: status})
	return err
}

func (r *Runner) updateTaskStatusWithChanges(ctx context.Context, taskID string, status string, repoPath string) ([]string, error) {
	mutation, err := r.setTaskStatus(ctx, tasks.StatusRequest{RepoPath: repoPath, ID: taskID, Status: status})
	return mutation.ChangedFiles, err
}

func mergeChangedFiles(groups ...[]string) []string {
	changed := map[string]struct{}{}
	for _, group := range groups {
		for _, path := range group {
			if path != "" {
				changed[path] = struct{}{}
			}
		}
	}
	return sortedKeys(changed)
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

// Run executes one command-analysis pipeline and keeps its task lifecycle synchronized with the outcome.
func (r *Runner) Run(ctx context.Context, req Request) (result Result, err error) {
	if err := r.updateTaskStatus(ctx, req.CurrentTaskID, taskStatusOrDefault(req.TaskOnStart, "in_progress"), req.RepoPath); err != nil {
		return Result{}, err
	}
	defer func() {
		finalStatus := taskStatusOrDefault(req.TaskOnSuccess, "done")
		if result.Status == "running" {
			finalStatus = taskStatusOrDefault(req.TaskOnStart, "in_progress")
		} else if !pipelineTaskCloseoutSucceeded(result, err) {
			finalStatus = taskStatusOrDefault(req.TaskOnFailure, "blocked")
		}
		if updateErr := r.updateTaskStatus(ctx, req.CurrentTaskID, finalStatus, req.RepoPath); updateErr != nil && err == nil {
			err = updateErr
		}
	}()
	if len(req.SecretHandles) > 0 {
		envs, mask, err := r.cfg.ResolveSecretEnv(req.SecretHandles)
		if err != nil {
			return Result{}, err
		}
		ctx = command.ContextWithSecrets(ctx, envs, mask)
	}

	cmdResult, err := r.commands.RunInRepoWithWait(ctx, req.Command, req.RepoPath, req.CWD, req.TimeoutSeconds, req.MCPWaitSeconds)
	if err != nil {
		return Result{}, err
	}
	compact := req.CompactOutput == nil || *req.CompactOutput
	if cmdResult.Status == "running" {
		return Result{Status: "running", Compact: compact, Command: cmdResult, Summary: evidence.Summary{Truncated: cmdResult.Truncated}, Analysis: "command still running; use command action=get with command_id", Validation: evidence.Validation{Valid: false, Status: "running"}, Handoff: runningHandoff(cmdResult)}, nil
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

func runningHandoff(result command.Result) string {
	var b strings.Builder
	b.WriteString("status: running")
	fmt.Fprintf(&b, "\ncommand_id: %s", result.CommandID)
	fmt.Fprintf(&b, "\nelapsed_ms: %d", result.DurationMS)
	b.WriteString("\nnext_call: command action=get")
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
		b.WriteString("output: collapsed; use command action=filter with command_id for details")
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
		return command.TruncateUTF8(out, maxChars) + "\n[truncated]"
	}
	return out
}
