package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/lake"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

const activeTasksLeanPath = "MCPAIHelperProject/ActiveTasks.lean"

var ErrLeanRegistryMutationSurfaceMissing = errors.New("Lean task registry mutation requires a Lean-owned lake serve mutation surface; Go-side ActiveTasks.lean mutation is disabled")

type leanMutationResult struct {
	Task         tasks.Task `json:"task"`
	Source       string     `json:"source"`
	Validation   string     `json:"validation"`
	ChangedFiles []string   `json:"changed_files,omitempty"`
}

type leanBatchMutationResult struct {
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

func setTaskStatus(ctx context.Context, req tasks.StatusRequest, commands *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	if strings.TrimSpace(req.Status) == "" {
		return leanMutationResult{}, errors.New("status is required")
	}
	payload, envelope, err := applyLeanTaskTransition(ctx, req.RepoPath, req, commands)
	if err != nil {
		return leanMutationResult{}, err
	}
	buildResult, buildErr := lake.Build(ctx, req.RepoPath, lake.CommandRunner{Commands: commands, TimeoutSeconds: 20})
	if buildErr != nil || buildResult.ExitCode != 0 {
		_ = writeLeanActiveTasksSource(ctx, req.RepoPath, payload.PreviousSource)
		if buildErr != nil {
			return leanMutationResult{}, fmt.Errorf("validate Lean task registry after transition: %w", buildErr)
		}
		diagnostic := strings.TrimSpace(strings.Join(buildResult.Diagnostics, "\n"))
		if diagnostic == "" {
			diagnostic = "lake build failed"
		}
		return leanMutationResult{}, fmt.Errorf("validate Lean task registry after transition: %s", diagnostic)
	}
	task, err := payload.Task.toTask()
	if err != nil {
		return leanMutationResult{}, err
	}
	return leanMutationResult{Task: task, Source: "lean_registry", Validation: envelope.Validation.Summary + " + lake build", ChangedFiles: envelope.ChangedFiles}, nil
}

func applyLeanTaskTransition(ctx context.Context, repoPath string, req tasks.StatusRequest, commands *command.Runner) (leanTaskTransitionApplyPayload, leanRegistryEnvelope, error) {
	result, err := lake.CallServerRPC(ctx, repoPath, lake.RPCRequest{
		SourceFile:     "MCPAIHelperProject/TaskRegistryExport.lean",
		Method:         "MCPAIHelperProject.TaskRegistryExport.taskTransitionApply",
		Params:         map[string]string{"id": req.ID, "to": req.Status},
		TimeoutSeconds: 20,
	})
	if err != nil {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, err
	}
	if result.Blocker != "" {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, fmt.Errorf("Lean task transition blocker: %s", result.Blocker)
	}
	var envelope leanRegistryEnvelope
	if err := json.Unmarshal(result.Result, &envelope); err != nil {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, fmt.Errorf("decode Lean task transition envelope: %w", err)
	}
	if envelope.SchemaVersion != 1 {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, fmt.Errorf("unsupported Lean task transition schema_version: %d", envelope.SchemaVersion)
	}
	if envelope.Operation != "task.transition.apply" {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, fmt.Errorf("unexpected Lean task transition operation: %q", envelope.Operation)
	}
	if !envelope.OK {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, fmt.Errorf("Lean task transition rejected: %s", leanRegistryDiagnosticsMessage(envelope.Diagnostics))
	}
	if !envelope.Validation.Checked {
		return leanTaskTransitionApplyPayload{}, leanRegistryEnvelope{}, errors.New("Lean task transition envelope did not report checked validation")
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
		TimeoutSeconds: 20,
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

func upsertTask(_ context.Context, req tasks.AddRequest, _ *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	if strings.TrimSpace(req.Title) == "" {
		return leanMutationResult{}, errors.New("title is required")
	}
	return leanMutationResult{}, ErrLeanRegistryMutationSurfaceMissing
}

func batchUpsertTasks(_ context.Context, _ tasks.BatchUpsertRequest, _ *command.Runner, _ *tasks.Store) (leanBatchMutationResult, error) {
	return leanBatchMutationResult{}, ErrLeanRegistryMutationSurfaceMissing
}

func deleteTask(_ context.Context, req tasks.DeleteRequest, _ *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	if strings.TrimSpace(req.ID) == "" {
		return leanMutationResult{}, errors.New("id is required")
	}
	return leanMutationResult{}, ErrLeanRegistryMutationSurfaceMissing
}
