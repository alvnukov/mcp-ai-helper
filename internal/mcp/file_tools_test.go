package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func newTestSrv(t *testing.T) *server.MCPServer {
	t.Helper()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	return New(cfg)
}

func readFilesToolHandler(t *testing.T, srv *server.MCPServer) func(context.Context, basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	t.Helper()
	st, ok := srv.ListTools()["read_files"]
	if !ok {
		t.Fatal("read_files tool not registered")
	}
	return st.Handler
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resultMap(t *testing.T, r *basemcp.CallToolResult) map[string]any {
	t.Helper()
	data, err := json.Marshal(r.StructuredContent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestReadFilesRegistered(t *testing.T) {
	t.Parallel()
	srv := newTestSrv(t)
	st, ok := srv.ListTools()["read_files"]
	if !ok {
		t.Fatal("read_files tool not registered")
	}
	if !strings.Contains(st.Tool.Description, "multiple") {
		t.Fatalf("description = %q", st.Tool.Description)
	}
	schemaBytes, err := json.Marshal(st.Tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBytes)
	for _, field := range []string{"repo_path", "paths"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("schema missing %q: %s", field, schema)
		}
	}
}

func TestReadFilesTwoValid(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "hello\nworld\n")
	writeTestFile(t, dir, "b.txt", "foo\nbar\nbaz\n")

	srv := newTestSrv(t)
	handler := readFilesToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"paths":     []any{"a.txt", "b.txt"},
	}
	r, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		t.Fatalf("unexpected error")
	}

	m := resultMap(t, r)
	if m["total_files"].(float64) != 2 {
		t.Fatalf("total_files = %v", m["total_files"])
	}
	if m["returned_files"].(float64) != 2 {
		t.Fatalf("returned_files = %v", m["returned_files"])
	}

	files := m["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("files len = %d", len(files))
	}
	f0 := files[0].(map[string]any)
	f1 := files[1].(map[string]any)
	if f0["relative_path"] != "a.txt" || f1["relative_path"] != "b.txt" {
		t.Fatalf("order: [0]=%q [1]=%q", f0["relative_path"], f1["relative_path"])
	}
	lines0 := f0["lines"].([]any)
	if len(lines0) != 2 {
		t.Fatalf("a.txt lines = %d, want 2", len(lines0))
	}
}

func TestReadFilesOneMissing(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "present.txt", "data\n")

	srv := newTestSrv(t)
	handler := readFilesToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"paths":     []any{"present.txt", "missing.txt"},
	}
	r, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		t.Fatal("should not fail for mixed results")
	}

	m := resultMap(t, r)
	files := m["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("files len = %d", len(files))
	}
	f0 := files[0].(map[string]any)
	f1 := files[1].(map[string]any)
	if f0["exists"] != true {
		t.Fatal("present file should exist")
	}
	if f1["exists"] != false {
		t.Fatal("missing file should not exist")
	}
	if f1["error"] == nil || f1["error"] == "" {
		t.Fatal("missing file should have error")
	}
	if m["returned_files"].(float64) != 1 {
		t.Fatalf("returned_files = %v", m["returned_files"])
	}
}

func TestReadFilesEmptyPaths(t *testing.T) {
	dir := t.TempDir()
	srv := newTestSrv(t)
	handler := readFilesToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"paths":     []any{},
	}
	r, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError {
		t.Fatal("expected error for empty paths")
	}
}

func TestReadFilesTooManyPaths(t *testing.T) {
	dir := t.TempDir()
	srv := newTestSrv(t)
	handler := readFilesToolHandler(t, srv)

	paths := make([]any, 9)
	for i := range 9 {
		paths[i] = "x.txt"
	}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"paths":     paths,
	}
	r, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError {
		t.Fatal("expected error for >8 paths")
	}
}

func TestReadFilesPerFileByteLimit(t *testing.T) {
	dir := t.TempDir()
	bigContent := strings.Repeat("x", 65*1024)
	writeTestFile(t, dir, "big.txt", bigContent)
	writeTestFile(t, dir, "small.txt", "tiny\n")

	srv := newTestSrv(t)
	handler := readFilesToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"paths":     []any{"big.txt", "small.txt"},
	}
	r, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		t.Fatalf("unexpected error")
	}

	m := resultMap(t, r)
	files := m["files"].([]any)
	f0 := files[0].(map[string]any)
	if f0["truncated"] != true {
		t.Fatal("big file should be truncated")
	}
	if f0["omitted_reason"] == nil || f0["omitted_reason"] == "" {
		t.Fatal("truncated file should have omitted_reason")
	}
	f1 := files[1].(map[string]any)
	if f1["truncated"] != nil {
		t.Fatal("small file should not be truncated")
	}
}

func TestReadFilesTotalByteLimit(t *testing.T) {
	dir := t.TempDir()
	content := strings.Repeat("y", 50*1024)
	writeTestFile(t, dir, "f1.txt", content)
	writeTestFile(t, dir, "f2.txt", content)
	writeTestFile(t, dir, "f3.txt", content)

	srv := newTestSrv(t)
	handler := readFilesToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"paths":     []any{"f1.txt", "f2.txt", "f3.txt"},
	}
	r, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		t.Fatalf("unexpected error")
	}

	m := resultMap(t, r)
	if m["truncated"] != true {
		t.Fatal("result should be truncated when total bytes exceed limit")
	}
	returned := m["returned_files"].(float64)
	if returned > 2 || returned < 1 {
		t.Fatalf("returned_files = %v, want 2", returned)
	}
}

func TestReadFileStillRegistered(t *testing.T) {
	t.Parallel()
	srv := newTestSrv(t)
	if _, ok := srv.ListTools()["read_file"]; !ok {
		t.Fatal("read_file tool no longer registered")
	}
}
