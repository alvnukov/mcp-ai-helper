// Package pipeline coordinates workflows, command checks, guarded edits, and commits.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

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
	cfg        *config.Config
	commands   *command.Runner
	chatClient provider.ChatClient
	tasks      *tasks.Store
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
}

// Result is the command-analysis pipeline output.
type Result struct {
	Status     string              `json:"status"`
	Command    command.Result      `json:"command"`
	Summary    evidence.Summary    `json:"summary"`
	Analysis   string              `json:"analysis,omitempty"`
	Validation evidence.Validation `json:"validation"`
	Handoff    string              `json:"handoff"`
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

// WorkflowResult is the complete workflow execution record.
type WorkflowResult struct {
	Status       string                  `json:"status"`
	StepResults  []WorkflowStepResult    `json:"step_results,omitempty"`
	EditResults  []fileops.ReplaceResult `json:"edit_results"`
	CheckResults []command.Result        `json:"check_results"`
	CommitResult *gitops.CommitResult    `json:"commit_result,omitempty"`
	ChangedFiles []string                `json:"changed_files"`
	Reason       string                  `json:"reason,omitempty"`
}

// NewRunner creates a workflow runner.
func NewRunner(cfg *config.Config, chatClient provider.ChatClient) *Runner {
	projectStore, err := project.NewStore(cfg.CommandPolicy.LogDir)
	if err != nil {
		projectStore, _ = project.NewStore(".mcp-ai-helper")
	}
	return &Runner{cfg: cfg, commands: command.NewRunner(cfg.CommandPolicy), chatClient: chatClient, tasks: tasks.NewStore(projectStore)}
}

// RunWorkflow executes either the stable steps DSL or the legacy edit/check/commit workflow.
func (r *Runner) RunWorkflow(ctx context.Context, req WorkflowRequest) (result WorkflowResult, err error) {
	if err := r.updateTaskStatus(req.CurrentTaskID, taskStatusOrDefault(req.TaskOnStart, "in_progress"), req.RepoPath); err != nil {
		return WorkflowResult{}, err
	}
	defer func() {
		finalStatus := taskStatusOrDefault(req.TaskOnSuccess, "done")
		if err != nil || result.Status != "ok" {
			finalStatus = taskStatusOrDefault(req.TaskOnFailure, "blocked")
		}
		if updateErr := r.updateTaskStatus(req.CurrentTaskID, finalStatus, req.RepoPath); updateErr != nil && err == nil {
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
	for _, step := range req.Steps {
		if !evalStepCondition(step.If, result, stepResults) {
			stepResult := WorkflowStepResult{ID: step.ID, Tool: step.Tool, Status: "skipped", Reason: "condition is false"}
			result.StepResults = append(result.StepResults, stepResult)
			if step.ID != "" {
				stepResults[step.ID] = stepResult
			}
			continue
		}
		stepResult, err := r.executeWorkflowStep(ctx, req.RepoPath, step, changedSet)
		if err != nil {
			return WorkflowResult{}, err
		}
		result.StepResults = append(result.StepResults, stepResult)
		if step.ID != "" {
			stepResults[step.ID] = stepResult
		}
		if editResult, ok := stepResult.Output.(fileops.ReplaceResult); ok {
			result.EditResults = append(result.EditResults, editResult)
		}
		if checkResult, ok := stepResult.Output.(command.Result); ok {
			result.CheckResults = append(result.CheckResults, checkResult)
		}
		if commitResult, ok := stepResult.Output.(gitops.CommitResult); ok {
			result.CommitResult = &commitResult
		}
		result.ChangedFiles = sortedKeys(changedSet)
		if stepResult.Status != "ok" && stepResult.Status != "skipped" {
			result.Status = stepResult.Status
			result.Reason = stepResult.Reason
			if step.OnFailure != "continue" {
				break
			}
		}
	}
	result.ChangedFiles = sortedKeys(changedSet)
	return result, nil
}

func (r *Runner) executeWorkflowStep(ctx context.Context, repoPath string, step WorkflowStep, changedSet map[string]struct{}) (WorkflowStepResult, error) {
	base := WorkflowStepResult{ID: step.ID, Tool: step.Tool, Status: "ok"}
	switch step.Tool {
	case "guarded_replace":
		var args WorkflowEdit
		if err := bindStepArgs(step.Args, &args); err != nil {
			return WorkflowStepResult{}, err
		}
		replaceReq := fileops.ReplaceRequest{RepoPath: repoPath, Path: args.Path, ExpectedHash: args.ExpectedHash, Old: args.Old, New: args.New}
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
		}
		return base, nil
	case "run_command":
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
		commitResult, err := gitops.CommitOwned(ctx, gitops.CommitRequest{RepoPath: repoPath, Files: files, Message: args.Message})
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Status = commitResult.Status
		base.Reason = commitResult.Reason
		base.Output = commitResult
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
		taskResult, err := r.tasks.BatchUpsert(args)
		if err != nil {
			return WorkflowStepResult{}, err
		}
		base.Output = taskResult
		return base, nil
	default:
		base.Status = "failed"
		base.Reason = "unknown workflow tool: " + step.Tool
		return base, nil
	}
}

func bindStepArgs(args map[string]any, target any) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func evalStepCondition(condition string, result WorkflowResult, steps map[string]WorkflowStepResult) bool {
	condition = strings.TrimSpace(condition)
	if condition == "" || condition == "always" {
		return true
	}
	if condition == "changed_files_count > 0" {
		return len(result.ChangedFiles) > 0
	}
	if strings.HasPrefix(condition, "steps.") {
		parts := strings.Split(condition, " ")
		if len(parts) != 3 || parts[1] != "==" {
			return false
		}
		left := strings.TrimPrefix(parts[0], "steps.")
		path := strings.Split(left, ".")
		if len(path) != 2 || path[1] != "status" {
			return false
		}
		step, ok := steps[path[0]]
		return ok && step.Status == parts[2]
	}
	return false
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

func (r *Runner) updateTaskStatus(taskID string, status string, repoPath string) error {
	if taskID == "" || status == "" {
		return nil
	}
	if !r.cfg.LayerEnabled("tasks") {
		return fmt.Errorf("task layer is disabled")
	}
	_, err := r.tasks.SetStatus(tasks.StatusRequest{
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

func (r *Runner) Run(ctx context.Context, req Request) (result Result, err error) {
	if err := r.updateTaskStatus(req.CurrentTaskID, taskStatusOrDefault(req.TaskOnStart, "in_progress"), req.RepoPath); err != nil {
		return Result{}, err
	}
	defer func() {
		if err != nil {
			if updateErr := r.updateTaskStatus(req.CurrentTaskID, taskStatusOrDefault(req.TaskOnFailure, "blocked"), req.RepoPath); updateErr != nil {
				err = updateErr
			}
			return
		}
		if updateErr := r.updateTaskStatus(req.CurrentTaskID, taskStatusOrDefault(req.TaskOnSuccess, "done"), req.RepoPath); updateErr != nil {
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
	handoff := composeHandoff(status, cmdResult, summary, analysis, validation, r.cfg.PipelinePolicy.MaxReturnChars)
	return Result{Status: status, Command: cmdResult, Summary: summary, Analysis: analysis, Validation: validation, Handoff: handoff}, nil
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
