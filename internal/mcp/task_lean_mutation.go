package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/command"
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

func validateLeanTaskTransitionWithServer(_ context.Context, _ string, _ tasks.StatusRequest, _ tasks.Task) ([]string, string, error) {
	return nil, "", ErrLeanRegistryMutationSurfaceMissing
}

func setTaskStatus(_ context.Context, req tasks.StatusRequest, _ *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	if strings.TrimSpace(req.Status) == "" {
		return leanMutationResult{}, errors.New("status is required")
	}
	return leanMutationResult{}, ErrLeanRegistryMutationSurfaceMissing
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
