package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

var ErrLossyField = errors.New("lossy field detected")
var ErrDuplicateID = errors.New("duplicate task ID in target")
var ErrStaleTarget = errors.New("stale target registry")

type ImportExportRequest struct {
	RepoPath string
	DryRun   bool
	Overwrite bool
}

type ImportExportResult struct {
	Added     []tasks.Task `json:"added"`
	Updated   []tasks.Task `json:"updated"`
	Conflicts []string     `json:"conflicts"`
	Losses    []LossReport `json:"losses,omitempty"`
	DryRun    bool         `json:"dry_run"`
}

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
	for _, srcTask := range sourceTasks {
		existing, _, getErr := target.Get(ctx, repoPath, srcTask.ID)
		if getErr == nil {
			if !req.Overwrite {
				result.Conflicts = append(result.Conflicts, srcTask.ID)
				continue
			}
			_ = existing
		}
		if req.DryRun {
			result.Added = append(result.Added, srcTask)
			continue
		}
		upsertReq := taskToAddRequest(srcTask, repoPath)
		mutResult, err := target.Upsert(ctx, upsertReq)
		if err != nil {
			return nil, fmt.Errorf("write task %s to target: %w", srcTask.ID, err)
		}
		if getErr == nil {
			result.Updated = append(result.Updated, mutResult.Task)
		} else {
			result.Added = append(result.Added, mutResult.Task)
		}
	}
	return result, nil
}

func taskToAddRequest(t tasks.Task, repoPath string) tasks.AddRequest {
	return tasks.AddRequest{
		RepoPath: repoPath, ID: t.ID, Title: t.Title, Status: t.Status,
		Priority: t.Priority, ModelLevel: t.ModelLevel,
		TaskType: t.TaskType, ParentID: t.ParentID,
		Tags: t.Tags, Branch: t.Branch, WorktreePath: t.WorktreePath,
		AcceptanceCriteria: t.AcceptanceCriteria,
		VerificationPlan:   t.VerificationPlan,
		Body: t.Body,
	}
}
