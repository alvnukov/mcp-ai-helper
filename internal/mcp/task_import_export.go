package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
)

// ErrLossyField indicates that a task field cannot be represented by the target registry.
var ErrLossyField = errors.New("lossy field detected")

// ErrDuplicateID indicates that the target registry contains a duplicate task identifier.
var ErrDuplicateID = errors.New("duplicate task ID in target")

// ErrStaleTarget indicates that an import target changed after it was inspected.
var ErrStaleTarget = errors.New("stale target registry")

// ExportRequest defines the repository and destination options for task export.
type ExportRequest struct {
	RepoPath  string `json:"repo_path"`
	TargetDir string `json:"target_dir"`
	DryRun    bool   `json:"dry_run"`
	Overwrite bool   `json:"overwrite"`
}

// ImportExportRequest defines shared safety options for a registry transfer.
type ImportExportRequest struct {
	RepoPath  string
	DryRun    bool
	Overwrite bool
}

// ImportExportResult reports transferred tasks, conflicts, and detected losses.
type ImportExportResult struct {
	Added     []tasks.Task `json:"added"`
	Updated   []tasks.Task `json:"updated"`
	Conflicts []string     `json:"conflicts"`
	Losses    []LossReport `json:"losses,omitempty"`
	DryRun    bool         `json:"dry_run"`
}

// LossReport describes one field that could not be transferred losslessly.
type LossReport struct {
	TaskID string `json:"task_id"`
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func exportTasks(ctx context.Context, source taskBackend, target taskBackend, repoPath string, req ImportExportRequest) (*ImportExportResult, error) {
	sourceTasks, _, err := source.ListAll(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("read source tasks: %w", err)
	}
	result := &ImportExportResult{DryRun: req.DryRun}
	duplicateSourceIDs := duplicateTaskIDs(sourceTasks)
	if len(duplicateSourceIDs) > 0 {
		result.Conflicts = append(result.Conflicts, duplicateSourceIDs...)
		for _, id := range duplicateSourceIDs {
			result.Losses = append(result.Losses, LossReport{TaskID: id, Field: "id", Reason: "duplicate source ID cannot be represented losslessly in target"})
		}
		if req.DryRun {
			return result, nil
		}
		return result, fmt.Errorf("%w in source: %s", ErrDuplicateID, strings.Join(duplicateSourceIDs, ", "))
	}
	planned := make([]tasks.Task, 0, len(sourceTasks))
	updates := make(map[string]bool, len(sourceTasks))
	for _, srcTask := range sourceTasks {
		_, _, getErr := target.Get(ctx, repoPath, srcTask.ID)
		if getErr == nil {
			if !req.Overwrite {
				result.Conflicts = append(result.Conflicts, srcTask.ID)
				continue
			}
			updates[srcTask.ID] = true
		}
		planned = append(planned, srcTask)
		if req.DryRun {
			if updates[srcTask.ID] {
				result.Updated = append(result.Updated, srcTask)
			} else {
				result.Added = append(result.Added, srcTask)
			}
		}
	}
	if len(result.Conflicts) > 0 {
		if req.DryRun {
			return result, nil
		}
		return result, fmt.Errorf("%w: %s", ErrDuplicateID, strings.Join(result.Conflicts, ", "))
	}
	if req.DryRun {
		return result, nil
	}
	for _, srcTask := range planned {
		upsertReq := taskToAddRequest(srcTask, repoPath)
		mutResult, err := target.Upsert(ctx, upsertReq)
		if err != nil {
			return nil, fmt.Errorf("write task %s to target: %w", srcTask.ID, err)
		}
		if updates[srcTask.ID] {
			result.Updated = append(result.Updated, mutResult.Task)
		} else {
			result.Added = append(result.Added, mutResult.Task)
		}
	}
	return result, nil
}

func duplicateTaskIDs(items []tasks.Task) []string {
	counts := make(map[string]int, len(items))
	duplicates := []string{}
	for _, item := range items {
		counts[item.ID]++
		if counts[item.ID] == 2 {
			duplicates = append(duplicates, item.ID)
		}
	}
	return duplicates
}

func taskToAddRequest(t tasks.Task, repoPath string) tasks.AddRequest {
	return tasks.AddRequest{
		RepoPath: repoPath, ID: t.ID, Title: t.Title, Status: t.Status,
		Priority: t.Priority, ModelLevel: t.ModelLevel,
		TaskType: t.TaskType, ParentID: t.ParentID,
		Tags: t.Tags, Branch: t.Branch, WorktreePath: t.WorktreePath,
		AcceptanceCriteria: t.AcceptanceCriteria,
		VerificationPlan:   t.VerificationPlan,
		Body:               t.Body,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
		PreserveTimestamps: true,
	}
}
