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

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

func newTestSrv(t *testing.T) *server.MCPServer {
	t.Helper()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	return New(cfg)
}

func fileToolHandler(t *testing.T, srv *server.MCPServer) func(context.Context, basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
	t.Helper()
	st, ok := srv.ListTools()["file"]
	if !ok {
		t.Fatal("file tool not registered")
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
	// An error result carries no JSON payload, so without this the caller
	// reads a nil map and panics — which takes the whole package down and hides
	// every test after it. Reporting the tool's own message instead says what
	// actually went wrong.
	if r == nil {
		t.Fatal("tool returned no result")
		return nil
	}
	if r.IsError {
		t.Fatalf("tool returned an error: %s", resultText(t, r))
	}
	text := resultText(t, r)
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m == nil {
		t.Fatalf("tool returned no JSON payload: %s", text)
	}
	return m
}

func TestFileToolRegistered(t *testing.T) {
	t.Parallel()
	srv := newTestSrv(t)
	st, ok := srv.ListTools()["file"]
	if !ok {
		t.Fatal("file tool not registered")
	}
	if !strings.Contains(st.Tool.Description, "read_many") {
		t.Fatalf("description = %q", st.Tool.Description)
	}
	schemaBytes, err := json.Marshal(st.Tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	schema := string(schemaBytes)
	for _, field := range []string{"repo_path", "paths", "action"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("schema missing %q: %s", field, schema)
		}
	}
}

func TestEditToolRegistered(t *testing.T) {
	t.Parallel()
	srv := newTestSrv(t)
	if _, ok := srv.ListTools()["edit"]; !ok {
		t.Fatal("edit tool not registered")
	}
}

func TestReadFilesTwoValid(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "hello\nworld\n")
	writeTestFile(t, dir, "b.txt", "foo\nbar\nbaz\n")

	srv := newTestSrv(t)
	handler := fileToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"action":    "read_many",
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
	handler := fileToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"action":    "read_many",
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
	handler := fileToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"action":    "read_many",
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
	handler := fileToolHandler(t, srv)

	paths := make([]any, 9)
	for i := range 9 {
		paths[i] = "x.txt"
	}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"action":    "read_many",
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
	handler := fileToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"action":    "read_many",
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
	handler := fileToolHandler(t, srv)

	req := basemcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"repo_path": dir,
		"action":    "read_many",
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

func TestFileReadActionStillRegistered(t *testing.T) {
	t.Parallel()
	srv := newTestSrv(t)
	if _, ok := srv.ListTools()["file"]; !ok {
		t.Fatal("file tool no longer registered")
	}
}

func searchArgs(dir string, extra map[string]any) basemcp.CallToolRequest {
	args := map[string]any{
		"repo_path": dir,
		"action":    "search",
		"pattern":   "needle",
	}
	for k, v := range extra {
		args[k] = v
	}
	req := basemcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// The rg-like options must reach the fileops search through the handler:
// regex and glob filters answer differently from the literal default.
func TestFileSearchWithOptions(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.go", "func  spaced needle()\n")
	writeTestFile(t, dir, "b.md", "func\\s+spaced needle literal\n")

	srv := newTestSrv(t)
	handler := fileToolHandler(t, srv)

	r, err := handler(context.Background(), searchArgs(dir, map[string]any{
		"pattern": "func\\s+spaced",
		"regex":   true,
		"glob":    []any{"*.go"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	m := resultMap(t, r)
	if m["total"].(float64) != 1 {
		t.Fatalf("regex+glob total = %v, matches = %v", m["total"], m["matches"])
	}
	matches := m["matches"].([]any)
	if matches[0].(string) != "a.go:1:func  spaced needle()" {
		t.Fatalf("match = %q", matches[0])
	}

	r, err = handler(context.Background(), searchArgs(dir, map[string]any{
		"pattern": "func\\s+spaced",
	}))
	if err != nil {
		t.Fatal(err)
	}
	m = resultMap(t, r)
	if m["total"].(float64) != 1 || !strings.HasPrefix(m["matches"].([]any)[0].(string), "b.md:1:") {
		t.Fatalf("literal default total = %v, matches = %v", m["total"], m["matches"])
	}
}

// An invalid regex must come back as a readable tool error rather than a
// silent empty result.
func TestFileSearchInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "needle\n")

	srv := newTestSrv(t)
	handler := fileToolHandler(t, srv)

	r, err := handler(context.Background(), searchArgs(dir, map[string]any{
		"pattern": "needle(",
		"regex":   true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError {
		t.Fatal("expected a tool error for an invalid regex")
	}
	if !strings.Contains(resultText(t, r), "invalid regex") {
		t.Fatalf("error text = %q, want it to name the invalid regex", resultText(t, r))
	}
}
