package mcp

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/lake"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

const activeTasksLeanPath = "MCPAIHelperProject/ActiveTasks.lean"

//go:embed task_registry_templates/* task_registry_templates/MCPAIHelperProject/*
var taskRegistryBootstrapTemplates embed.FS

type taskMutationResult struct {
	Task         tasks.Task `json:"task"`
	Source       string     `json:"source"`
	Validation   string     `json:"validation"`
	ChangedFiles []string   `json:"changed_files,omitempty"`
}

type taskBatchMutationResult struct {
	Upserted   []tasks.Task `json:"upserted"`
	Closed     []tasks.Task `json:"closed"`
	Source     string       `json:"source"`
	Validation string       `json:"validation"`
}

type leanRegistryDiagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type leanRegistryValidation struct {
	Checked bool   `json:"checked"`
	Summary string `json:"summary"`
}

type leanRegistryEnvelope struct {
	SchemaVersion int                      `json:"schema_version"`
	OK            bool                     `json:"ok"`
	Operation     string                   `json:"operation"`
	Data          json.RawMessage          `json:"data"`
	Diagnostics   []leanRegistryDiagnostic `json:"diagnostics"`
	ChangedFiles  []string                 `json:"changed_files"`
	Validation    leanRegistryValidation   `json:"validation"`
}

type leanTaskTransitionApplyPayload struct {
	Task           leanTaskProjection `json:"task"`
	PreviousSource string             `json:"previous_source"`
}

type leanTaskBatchApplyPayload struct {
	Upserted       []leanTaskProjection `json:"upserted"`
	Closed         []leanTaskProjection `json:"closed"`
	PreviousSource string               `json:"previous_source"`
}

type leanTaskUpsertRPCRequest struct {
	ID                 string   `json:"id"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Priority           string   `json:"priority"`
	ModelLevel         string   `json:"model_level"`
	Tags               []string `json:"tags"`
	TaskType           string   `json:"task_type"`
	Branch             string   `json:"branch"`
	WorktreePath       string   `json:"worktree_path"`
	ParentID           string   `json:"parent_id"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	VerificationPlan   []string `json:"verification_plan"`
}

type leanTaskBatchUpsertRPCRequest struct {
	Tasks          []leanTaskUpsertRPCRequest `json:"tasks"`
	CloseMissing   bool                       `json:"close_missing"`
	MissingStatus  string                     `json:"missing_status"`
	ActiveStatuses []string                   `json:"active_statuses"`
}

type leanTaskDeleteRPCRequest struct {
	ID string `json:"id"`
}

func setTaskStatus(ctx context.Context, req tasks.StatusRequest, commands *command.Runner, _ *tasks.Store) (taskMutationResult, error) {
	if strings.TrimSpace(req.Status) == "" {
		return taskMutationResult{}, errors.New("status is required")
	}
	if err := ensureLeanTaskRegistryBootstrap(ctx, req.RepoPath, commands); err != nil {
		return taskMutationResult{}, err
	}
	payload, envelope, err := applyLeanTaskTransition(ctx, req.RepoPath, req, commands)
	if err != nil {
		return taskMutationResult{}, err
	}
	if err := validateLeanRegistryBuild(ctx, req.RepoPath, commands, "after transition"); err != nil {
		_ = writeLeanActiveTasksSource(ctx, req.RepoPath, payload.PreviousSource)
		return taskMutationResult{}, err
	}
	task, err := payload.Task.toTask()
	if err != nil {
		return taskMutationResult{}, err
	}
	return taskMutationResult{Task: task, Source: "lean_registry", Validation: envelope.Validation.Summary + " + lake build", ChangedFiles: envelope.ChangedFiles}, nil
}

func ensureLeanTaskRegistryBootstrap(ctx context.Context, repoPath string, commands *command.Runner) error {
	if commands == nil {
		return errors.New("Lake workspace blocker: command runner is required for Lean task registry bootstrap")
	}
	absPath, err := filepath.Abs(strings.TrimSpace(repoPath))
	if err != nil {
		return fmt.Errorf("resolve repo_path for Lean task registry bootstrap: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(absPath, "MCPAIHelperProject"), 0o755); err != nil { // #nosec G301 -- Lean source directory is intentionally repo-readable.
		return fmt.Errorf("create Lean task registry directory: %w", err)
	}
	if leanTaskRegistryBootstrapComplete(absPath) {
		return nil
	}
	if !fileExists(filepath.Join(absPath, "lakefile.lean")) && !fileExists(filepath.Join(absPath, "lakefile.toml")) {
		if err := copyBootstrapTemplateIfMissing(absPath, "lakefile.lean", "lakefile.lean"); err != nil {
			return err
		}
	}
	copies := map[string]string{
		"lean-toolchain":                             "lean-toolchain",
		"MCPAIHelperProject.lean":                    "MCPAIHelperProject.lean",
		"MCPAIHelperProject/ProjectState.lean":       "MCPAIHelperProject/ProjectState.lean",
		"MCPAIHelperProject/Samples.lean":            "MCPAIHelperProject/Samples.lean",
		"MCPAIHelperProject/Registry.lean":           "MCPAIHelperProject/Registry.lean",
		"MCPAIHelperProject/TaskRegistryExport.lean": "MCPAIHelperProject/TaskRegistryExport.lean",
	}
	for targetRel, sourceRel := range copies {
		if targetRel == "MCPAIHelperProject/TaskRegistryExport.lean" && fileExists(filepath.Join(absPath, filepath.FromSlash(targetRel))) && !leanTaskExporterCurrent(absPath) {
			if err := copyBootstrapTemplate(absPath, sourceRel, targetRel); err != nil {
				return err
			}
			continue
		}
		if err := copyBootstrapTemplateIfMissing(absPath, sourceRel, targetRel); err != nil {
			return err
		}
	}
	activePath := filepath.Join(absPath, activeTasksLeanPath)
	if !fileExists(activePath) {
		if err := os.WriteFile(activePath, []byte(emptyActiveTasksLeanSource), 0o644); err != nil { // #nosec G306 -- Lean task source is intentionally repo-readable.
			return fmt.Errorf("write %s: %w", activeTasksLeanPath, err)
		}
	}
	if commands != nil {
		return validateLeanRegistryBuild(ctx, absPath, commands, "after bootstrap")
	}
	return nil
}

func leanTaskRegistryBootstrapComplete(repoPath string) bool {
	return fileExists(filepath.Join(repoPath, "lean-toolchain")) &&
		(fileExists(filepath.Join(repoPath, "lakefile.lean")) || fileExists(filepath.Join(repoPath, "lakefile.toml"))) &&
		fileExists(filepath.Join(repoPath, "MCPAIHelperProject.lean")) &&
		fileExists(filepath.Join(repoPath, "MCPAIHelperProject/ProjectState.lean")) &&
		fileExists(filepath.Join(repoPath, "MCPAIHelperProject/ActiveTasks.lean")) &&
		fileExists(filepath.Join(repoPath, "MCPAIHelperProject/Registry.lean")) &&
		leanTaskExporterCurrent(repoPath)
}

func leanTaskExporterCurrent(repoPath string) bool {
	data, err := os.ReadFile(filepath.Join(repoPath, "MCPAIHelperProject", "TaskRegistryExport.lean")) // #nosec G304 -- path is inside the caller-selected local repo task registry.
	if err != nil {
		return false
	}
	source := string(data)
	for _, marker := range []string{
		"def taskList ",
		"def taskGet ",
		"def taskTransitionApply ",
		"def taskUpsertApply ",
		"def taskBatchUpsertApply ",
		"def taskDeleteApply ",
		"def activeTasksWrite ",
	} {
		if !strings.Contains(source, marker) {
			return false
		}
	}
	return true
}

func copyBootstrapTemplateIfMissing(targetRoot string, sourceRel string, targetRel string) error {
	targetPath := filepath.Join(targetRoot, filepath.FromSlash(targetRel))
	if fileExists(targetPath) {
		return nil
	}
	return copyBootstrapTemplate(targetRoot, sourceRel, targetRel)
}

func copyBootstrapTemplate(targetRoot string, sourceRel string, targetRel string) error {
	targetPath := filepath.Join(targetRoot, filepath.FromSlash(targetRel))
	data, err := taskRegistryBootstrapTemplates.ReadFile("task_registry_templates/" + sourceRel)
	if err != nil {
		return fmt.Errorf("read embedded task registry bootstrap template %s: %w", sourceRel, err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil { // #nosec G301 -- Lean source directories are intentionally repo-readable.
		return fmt.Errorf("create parent for %s: %w", targetRel, err)
	}
	if err := os.WriteFile(targetPath, data, 0o644); err != nil { // #nosec G306 -- Lean source files are intentionally repo-readable.
		return fmt.Errorf("write %s: %w", targetRel, err)
	}
	return nil
}

const emptyActiveTasksLeanSource = `import MCPAIHelperProject.ProjectState

namespace MCPAIHelperProject
namespace ActiveTasks

def activeArtifacts : List Artifact :=
  []

def activeRelations : List ArtifactRelation :=
  []

end ActiveTasks
end MCPAIHelperProject
`

func validateLeanRegistryBuild(ctx context.Context, repoPath string, commands *command.Runner, phase string) error {
	buildResult, buildErr := lake.Build(ctx, repoPath, lake.CommandRunner{Commands: commands, TimeoutSeconds: leanTaskRegistryTimeoutSeconds})
	if buildErr != nil {
		return fmt.Errorf("validate Lean task registry %s: %w", phase, buildErr)
	}
	if buildResult.ExitCode == 0 {
		return nil
	}
	diagnostic := strings.TrimSpace(strings.Join(buildResult.Diagnostics, "\n"))
	if diagnostic == "" {
		diagnostic = "lake build failed"
	}
	if strings.Contains(diagnostic, "<<<<<<<") || strings.Contains(diagnostic, ">>>>>>>") || strings.Contains(diagnostic, "unexpected token '<<<'") {
		diagnostic = "conflict markers in Lean task registry: " + diagnostic
	}
	return fmt.Errorf("validate Lean task registry %s: %s", phase, diagnostic)
}

func callLeanTaskMutation(ctx context.Context, repoPath string, commands *command.Runner, method string, operation string, params any) (leanRegistryEnvelope, error) {
	if commands != nil {
		if err := validateLeanRegistryBuild(ctx, repoPath, commands, leanMutationPreflightPhase(operation)); err != nil {
			return leanRegistryEnvelope{}, err
		}
	}
	result, err := lake.CallServerRPC(ctx, repoPath, lake.RPCRequest{SourceFile: "MCPAIHelperProject/TaskRegistryExport.lean", Method: method, Params: params, TimeoutSeconds: leanTaskRegistryTimeoutSeconds, ResetAfterCall: true})
	if err != nil {
		return leanRegistryEnvelope{}, err
	}
	if result.Blocker != "" {
		return leanRegistryEnvelope{}, fmt.Errorf("Lean task mutation blocker: %s", result.Blocker)
	}
	var envelope leanRegistryEnvelope
	if err := json.Unmarshal(result.Result, &envelope); err != nil {
		return leanRegistryEnvelope{}, fmt.Errorf("decode Lean task mutation envelope: %w", err)
	}
	if envelope.SchemaVersion != 1 {
		return leanRegistryEnvelope{}, fmt.Errorf("unsupported Lean task mutation schema_version: %d", envelope.SchemaVersion)
	}
	if envelope.Operation != operation {
		return leanRegistryEnvelope{}, fmt.Errorf("unexpected Lean task mutation operation: %q", envelope.Operation)
	}
	if !envelope.OK {
		return leanRegistryEnvelope{}, fmt.Errorf("Lean task mutation rejected: %s", leanRegistryDiagnosticsMessage(envelope.Diagnostics))
	}
	if !envelope.Validation.Checked {
		return leanRegistryEnvelope{}, errors.New("Lean task mutation envelope did not report checked validation")
	}
	return envelope, nil
}

func leanMutationPreflightPhase(operation string) string {
	switch operation {
	case "task.transition.apply":
		return "before transition"
	case "task.upsert.apply":
		return "before upsert"
	case "task.batch_upsert.apply":
		return "before batch upsert"
	case "task.delete.apply":
		return "before delete"
	default:
		return "before mutation"
	}
}

func applyLeanTaskTransition(ctx context.Context, repoPath string, req tasks.StatusRequest, commands *command.Runner) (leanTaskTransitionApplyPayload, leanRegistryEnvelope, error) {
	envelope, err := callLeanTaskMutation(ctx, repoPath, commands, "MCPAIHelperProject.TaskRegistryExport.taskTransitionApply", "task.transition.apply", map[string]string{"id": req.ID, "to": req.Status})
	if err != nil {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, err
	}
	var payload leanTaskTransitionApplyPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, fmt.Errorf("decode Lean task transition payload: %w", err)
	}
	return payload, envelope, nil
}

func writeLeanActiveTasksSource(ctx context.Context, repoPath string, source string) error {
	result, err := lake.CallServerRPC(ctx, repoPath, lake.RPCRequest{
		SourceFile:     "MCPAIHelperProject/TaskRegistryExport.lean",
		Method:         "MCPAIHelperProject.TaskRegistryExport.activeTasksWrite",
		Params:         map[string]string{"source": source},
		TimeoutSeconds: leanTaskRegistryTimeoutSeconds,
		ResetAfterCall: true,
	})
	if err != nil {
		return err
	}
	if result.Blocker != "" {
		return fmt.Errorf("Lean active tasks rollback blocker: %s", result.Blocker)
	}
	return nil
}

func leanRegistryDiagnosticsMessage(diagnostics []leanRegistryDiagnostic) string {
	if len(diagnostics) == 0 {
		return "no diagnostics"
	}
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		code := strings.TrimSpace(diagnostic.Code)
		message := strings.TrimSpace(diagnostic.Message)
		switch {
		case code != "" && message != "":
			parts = append(parts, code+": "+message)
		case message != "":
			parts = append(parts, message)
		case code != "":
			parts = append(parts, code)
		}
	}
	if len(parts) == 0 {
		return "empty diagnostics"
	}
	return strings.Join(parts, "; ")
}

func validateLeanTaskTransitionWithServer(ctx context.Context, repoPath string, req tasks.StatusRequest, projected tasks.Task) ([]string, string, error) {
	payload, envelope, err := applyLeanTaskTransition(ctx, repoPath, req, nil)
	if err != nil {
		return nil, "", err
	}
	_ = writeLeanActiveTasksSource(ctx, repoPath, payload.PreviousSource)
	serverTask, err := payload.Task.toTask()
	if err != nil {
		return nil, "", err
	}
	if serverTask.ID != projected.ID || serverTask.Status != projected.Status {
		return nil, "", fmt.Errorf("Lean task transition mismatch: server=%s/%s projected=%s/%s", serverTask.ID, serverTask.Status, projected.ID, projected.Status)
	}
	return append([]string(nil), envelope.ChangedFiles...), envelope.Validation.Summary, nil
}

func leanTaskUpsertParams(req tasks.AddRequest) leanTaskUpsertRPCRequest {
	return leanTaskUpsertRPCRequest{ID: req.ID, Status: req.Status, Title: req.Title, Body: req.Body, Priority: req.Priority, ModelLevel: req.ModelLevel, Tags: nonNilStrings(req.Tags), TaskType: req.TaskType, Branch: req.Branch, WorktreePath: req.WorktreePath, ParentID: req.ParentID, AcceptanceCriteria: nonNilStrings(req.AcceptanceCriteria), VerificationPlan: nonNilStrings(req.VerificationPlan)}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func taskProjectionsToTasks(projections []leanTaskProjection) ([]tasks.Task, error) {
	out := make([]tasks.Task, 0, len(projections))
	for _, projection := range projections {
		task, err := projection.toTask()
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}

func upsertTask(ctx context.Context, req tasks.AddRequest, commands *command.Runner, _ *tasks.Store) (taskMutationResult, error) {
	if strings.TrimSpace(req.Title) == "" {
		return taskMutationResult{}, errors.New("title is required")
	}
	if err := ensureLeanTaskRegistryBootstrap(ctx, req.RepoPath, commands); err != nil {
		return taskMutationResult{}, err
	}
	envelope, err := callLeanTaskMutation(ctx, req.RepoPath, commands, "MCPAIHelperProject.TaskRegistryExport.taskUpsertApply", "task.upsert.apply", leanTaskUpsertParams(req))
	if err != nil {
		return taskMutationResult{}, err
	}
	var payload leanTaskTransitionApplyPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return taskMutationResult{}, fmt.Errorf("decode Lean task upsert payload: %w", err)
	}
	if err := validateLeanRegistryBuild(ctx, req.RepoPath, commands, "after upsert"); err != nil {
		_ = writeLeanActiveTasksSource(ctx, req.RepoPath, payload.PreviousSource)
		return taskMutationResult{}, err
	}
	task, err := payload.Task.toTask()
	if err != nil {
		return taskMutationResult{}, err
	}
	return taskMutationResult{Task: task, Source: "lean_registry", Validation: envelope.Validation.Summary + " + lake build", ChangedFiles: envelope.ChangedFiles}, nil
}

func batchUpsertTasks(ctx context.Context, req tasks.BatchUpsertRequest, commands *command.Runner, _ *tasks.Store) (taskBatchMutationResult, error) {
	if err := ensureLeanTaskRegistryBootstrap(ctx, req.RepoPath, commands); err != nil {
		return taskBatchMutationResult{}, err
	}
	items := make([]leanTaskUpsertRPCRequest, 0, len(req.Tasks))
	for _, item := range req.Tasks {
		items = append(items, leanTaskUpsertParams(item))
	}
	envelope, err := callLeanTaskMutation(ctx, req.RepoPath, commands, "MCPAIHelperProject.TaskRegistryExport.taskBatchUpsertApply", "task.batch_upsert.apply", leanTaskBatchUpsertRPCRequest{Tasks: items, CloseMissing: req.CloseMissing, MissingStatus: req.MissingStatus, ActiveStatuses: nonNilStrings(req.ActiveStatuses)})
	if err != nil {
		return taskBatchMutationResult{}, err
	}
	var payload leanTaskBatchApplyPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return taskBatchMutationResult{}, fmt.Errorf("decode Lean task batch upsert payload: %w", err)
	}
	if err := validateLeanRegistryBuild(ctx, req.RepoPath, commands, "after batch upsert"); err != nil {
		_ = writeLeanActiveTasksSource(ctx, req.RepoPath, payload.PreviousSource)
		return taskBatchMutationResult{}, err
	}
	upserted, err := taskProjectionsToTasks(payload.Upserted)
	if err != nil {
		return taskBatchMutationResult{}, err
	}
	closed, err := taskProjectionsToTasks(payload.Closed)
	if err != nil {
		return taskBatchMutationResult{}, err
	}
	return taskBatchMutationResult{Upserted: upserted, Closed: closed, Source: "lean_registry", Validation: envelope.Validation.Summary + " + lake build"}, nil
}

func deleteTask(ctx context.Context, req tasks.DeleteRequest, commands *command.Runner, _ *tasks.Store) (taskMutationResult, error) {
	if strings.TrimSpace(req.ID) == "" {
		return taskMutationResult{}, errors.New("id is required")
	}
	if err := ensureLeanTaskRegistryBootstrap(ctx, req.RepoPath, commands); err != nil {
		return taskMutationResult{}, err
	}
	envelope, err := callLeanTaskMutation(ctx, req.RepoPath, commands, "MCPAIHelperProject.TaskRegistryExport.taskDeleteApply", "task.delete.apply", leanTaskDeleteRPCRequest{ID: req.ID})
	if err != nil {
		return taskMutationResult{}, err
	}
	var payload leanTaskTransitionApplyPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return taskMutationResult{}, fmt.Errorf("decode Lean task delete payload: %w", err)
	}
	if err := validateLeanRegistryBuild(ctx, req.RepoPath, commands, "after delete"); err != nil {
		_ = writeLeanActiveTasksSource(ctx, req.RepoPath, payload.PreviousSource)
		return taskMutationResult{}, err
	}
	task, err := payload.Task.toTask()
	if err != nil {
		return taskMutationResult{}, err
	}
	return taskMutationResult{Task: task, Source: "lean_registry", Validation: envelope.Validation.Summary + " + lake build", ChangedFiles: envelope.ChangedFiles}, nil
}
