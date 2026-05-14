package mcp

import (
	"context"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type recordingTaskBackend struct {
	items     map[string]tasks.Task
	statusSet bool
	batched   bool
}

func newRecordingTaskBackend() *recordingTaskBackend {
	return &recordingTaskBackend{items: map[string]tasks.Task{"task-1": {ID: "task-1", Status: "todo", Title: "First", ProjectionSource: "custom_backend"}}}
}

func (b *recordingTaskBackend) ListCurrent(context.Context, string) ([]tasks.Task, string, error) {
	return []tasks.Task{b.items["task-1"]}, "custom_backend", nil
}

func (b *recordingTaskBackend) ListAll(ctx context.Context, repoPath string) ([]tasks.Task, string, error) {
	return b.ListCurrent(ctx, repoPath)
}

func (b *recordingTaskBackend) Get(_ context.Context, _ string, id string) (tasks.Task, string, error) {
	return b.items[id], "custom_backend", nil
}

func (b *recordingTaskBackend) Upsert(_ context.Context, req tasks.AddRequest) (taskMutationResult, error) {
	item := tasks.Task{ID: req.ID, Status: req.Status, Title: req.Title, ProjectionSource: "custom_backend"}
	b.items[item.ID] = item
	return taskMutationResult{Task: item, Source: "custom_backend"}, nil
}

func (b *recordingTaskBackend) SetStatus(_ context.Context, req tasks.StatusRequest) (taskMutationResult, error) {
	item := b.items[req.ID]
	item.Status = req.Status
	b.items[item.ID] = item
	b.statusSet = true
	return taskMutationResult{Task: item, Source: "custom_backend"}, nil
}

func (b *recordingTaskBackend) BatchUpsert(_ context.Context, req tasks.BatchUpsertRequest) (taskBatchMutationResult, error) {
	b.batched = true
	upserted := make([]tasks.Task, 0, len(req.Tasks))
	for _, reqTask := range req.Tasks {
		item := tasks.Task{ID: reqTask.ID, Status: reqTask.Status, Title: reqTask.Title, ProjectionSource: "custom_backend"}
		b.items[item.ID] = item
		upserted = append(upserted, item)
	}
	return taskBatchMutationResult{Upserted: upserted, Source: "custom_backend", Validation: "test"}, nil
}

func (b *recordingTaskBackend) Delete(_ context.Context, req tasks.DeleteRequest) (taskMutationResult, error) {
	item := b.items[req.ID]
	delete(b.items, req.ID)
	return taskMutationResult{Task: item, Source: "custom_backend"}, nil
}

func TestWorkflowTaskBackendUsesInjectedTaskBackend(t *testing.T) {
	backend := newRecordingTaskBackend()
	adapter := workflowTaskBackend{backend: backend}

	got, err := adapter.Get(context.Background(), "/repo", "task-1")
	if err != nil || got.ProjectionSource != "custom_backend" {
		t.Fatalf("Get = %+v, err=%v", got, err)
	}
	listed, err := adapter.List(context.Background(), "/repo")
	if err != nil || len(listed) != 1 || listed[0].ID != "task-1" {
		t.Fatalf("List = %+v, err=%v", listed, err)
	}
	updated, err := adapter.SetStatus(context.Background(), tasks.StatusRequest{RepoPath: "/repo", ID: "task-1", Status: "done"})
	if err != nil || updated.Status != "done" || !backend.statusSet {
		t.Fatalf("SetStatus = %+v, err=%v, backend=%+v", updated, err, backend)
	}
	batch, err := adapter.BatchUpsert(context.Background(), tasks.BatchUpsertRequest{RepoPath: "/repo", Tasks: []tasks.AddRequest{{ID: "task-2", Status: "todo", Title: "Second"}}})
	if err != nil || !backend.batched || batch.Source != "custom_backend" || len(batch.Upserted) != 1 {
		t.Fatalf("BatchUpsert = %+v, err=%v, backend=%+v", batch, err, backend)
	}
}
