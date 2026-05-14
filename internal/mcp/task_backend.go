package mcp

import (
	"context"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// taskBackend is the MCP task persistence contract. Handlers depend on this
// interface instead of a concrete Lake/Lean transport so another task backend
// can be wired in behind the same tool surface.
type taskBackend interface {
	ListCurrent(ctx context.Context, repoPath string) ([]tasks.Task, string, error)
	ListAll(ctx context.Context, repoPath string) ([]tasks.Task, string, error)
	Get(ctx context.Context, repoPath string, id string) (tasks.Task, string, error)
	Upsert(ctx context.Context, req tasks.AddRequest) (taskMutationResult, error)
	SetStatus(ctx context.Context, req tasks.StatusRequest) (taskMutationResult, error)
	BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (taskBatchMutationResult, error)
	Delete(ctx context.Context, req tasks.DeleteRequest) (taskMutationResult, error)
}

type lakeTaskBackend struct {
	commands *command.Runner
	store    *tasks.Store
}

func newLakeTaskBackend(commands *command.Runner, store *tasks.Store) taskBackend {
	return lakeTaskBackend{commands: commands, store: store}
}

func (b lakeTaskBackend) ListCurrent(ctx context.Context, repoPath string) ([]tasks.Task, string, error) {
	return readCurrentTasks(ctx, repoPath, b.commands, b.store)
}

func (b lakeTaskBackend) ListAll(ctx context.Context, repoPath string) ([]tasks.Task, string, error) {
	return readAllTasks(ctx, repoPath, b.commands, b.store)
}

func (b lakeTaskBackend) Get(ctx context.Context, repoPath string, id string) (tasks.Task, string, error) {
	return readTask(ctx, repoPath, id, b.commands, b.store)
}

func (b lakeTaskBackend) Upsert(ctx context.Context, req tasks.AddRequest) (taskMutationResult, error) {
	return upsertTask(ctx, req, b.commands, b.store)
}

func (b lakeTaskBackend) SetStatus(ctx context.Context, req tasks.StatusRequest) (taskMutationResult, error) {
	return setTaskStatus(ctx, req, b.commands, b.store)
}

func (b lakeTaskBackend) BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (taskBatchMutationResult, error) {
	return batchUpsertTasks(ctx, req, b.commands, b.store)
}

func (b lakeTaskBackend) Delete(ctx context.Context, req tasks.DeleteRequest) (taskMutationResult, error) {
	return deleteTask(ctx, req, b.commands, b.store)
}

type workflowTaskBackend struct {
	backend taskBackend
}

func (b workflowTaskBackend) Get(ctx context.Context, repoPath string, id string) (tasks.Task, error) {
	task, _, err := b.backend.Get(ctx, repoPath, id)
	return task, err
}

func (b workflowTaskBackend) List(ctx context.Context, repoPath string) ([]tasks.Task, error) {
	items, _, err := b.backend.ListCurrent(ctx, repoPath)
	return items, err
}

func (b workflowTaskBackend) SetStatus(ctx context.Context, req tasks.StatusRequest) (tasks.Task, error) {
	result, err := b.backend.SetStatus(ctx, req)
	return result.Task, err
}

func (b workflowTaskBackend) BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error) {
	result, err := b.backend.BatchUpsert(ctx, req)
	return tasks.BatchUpsertResult{Upserted: result.Upserted, Closed: result.Closed, Source: result.Source, Validation: result.Validation}, err
}
