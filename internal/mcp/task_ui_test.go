package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type fakeTaskUIBackend struct {
	items map[string]tasks.Task
}

func newFakeTaskUIBackend() *fakeTaskUIBackend {
	updated := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	return &fakeTaskUIBackend{items: map[string]tasks.Task{"task-1": {ID: "task-1", Status: "todo", Title: "First", Body: "body", Priority: "high", Tags: []string{"ui"}, UpdatedAt: updated, ProjectionSource: "lean_registry"}}}
}

func (b *fakeTaskUIBackend) List(context.Context, string) ([]tasks.Task, string, error) {
	out := make([]tasks.Task, 0, len(b.items))
	for _, item := range b.items {
		out = append(out, item)
	}
	return out, "lean_registry", nil
}

func (b *fakeTaskUIBackend) Get(_ context.Context, _ string, id string) (tasks.Task, string, error) {
	item, ok := b.items[id]
	if !ok {
		return tasks.Task{}, "lean_registry", http.ErrMissingFile
	}
	return item, "lean_registry", nil
}

func (b *fakeTaskUIBackend) Upsert(_ context.Context, req tasks.AddRequest) (leanMutationResult, error) {
	item := b.items[req.ID]
	item.ID = req.ID
	item.Title = req.Title
	item.Body = req.Body
	item.Status = req.Status
	item.Priority = req.Priority
	item.ModelLevel = req.ModelLevel
	item.Tags = req.Tags
	item.AcceptanceCriteria = req.AcceptanceCriteria
	item.VerificationPlan = req.VerificationPlan
	item.UpdatedAt = item.UpdatedAt.Add(time.Second)
	b.items[item.ID] = item
	return leanMutationResult{Task: item, Source: "lean_registry", Validation: "test"}, nil
}

func (b *fakeTaskUIBackend) SetStatus(_ context.Context, req tasks.StatusRequest) (leanMutationResult, error) {
	item := b.items[req.ID]
	item.Status = req.Status
	item.UpdatedAt = item.UpdatedAt.Add(time.Second)
	b.items[item.ID] = item
	return leanMutationResult{Task: item, Source: "lean_registry", Validation: "test"}, nil
}

func TestTaskUIListFiltersTasks(t *testing.T) {
	handler := newTaskUIHandler(newFakeTaskUIBackend())
	req := httptest.NewRequest(http.MethodGet, "/api/tasks?repo_path=/repo&status=todo&priority=high&tag=ui&query=first", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "lean_registry") || !strings.Contains(resp.Body.String(), "task-1") {
		t.Fatalf("unexpected body: %s", resp.Body.String())
	}
}

func TestTaskUIRejectsUnknownJSONFields(t *testing.T) {
	handler := newTaskUIHandler(newFakeTaskUIBackend())
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/task-1", bytes.NewBufferString(`{"repo_path":"/repo","id":"task-1","title":"x","status":"todo","updated_at":"2026-05-12T10:00:00Z","unknown":true}`))
	req.Header.Set("content-type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTaskUIRejectsStaleEdits(t *testing.T) {
	handler := newTaskUIHandler(newFakeTaskUIBackend())
	body := map[string]any{"repo_path": "/repo", "id": "task-1", "title": "x", "status": "todo", "updated_at": "2026-05-12T09:59:59Z"}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/task-1", bytes.NewReader(data))
	req.Header.Set("content-type", "application/json")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "stale task edit") {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTaskUIStartServesInProcessHTTP(t *testing.T) {
	cfg := &config.Config{}
	deps := &Server{cfg: cfg}
	deps.chat, deps.commands, deps.pipelines, deps.taskStore = buildDeps(cfg)

	result, err := deps.startTaskUI(context.Background(), "/repo", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = deps.stopTaskUI(ctx)
	})
	if result.AlreadyRunning {
		t.Fatalf("first start reported already running")
	}
	if !strings.HasPrefix(result.URL, result.BaseURL+"?") || !strings.Contains(result.URL, "repo_path=%2Frepo") {
		t.Fatalf("unexpected URL: %#v", result)
	}

	resp, err := http.Get(result.BaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "mcp-ai-helper tasks") {
		t.Fatalf("status=%d body=%s", resp.StatusCode, string(body))
	}

	again, err := deps.startTaskUI(context.Background(), "/repo2", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadyRunning || again.BaseURL != result.BaseURL || !strings.Contains(again.URL, "repo_path=%2Frepo2") {
		t.Fatalf("unexpected idempotent start: %#v", again)
	}
}

func TestTaskUIStartRejectsNonLoopbackAddr(t *testing.T) {
	cfg := &config.Config{}
	deps := &Server{cfg: cfg}
	deps.chat, deps.commands, deps.pipelines, deps.taskStore = buildDeps(cfg)
	if _, err := deps.startTaskUI(context.Background(), "/repo", "0.0.0.0:0"); err == nil {
		t.Fatal("expected non-loopback bind to be rejected")
	}
}
