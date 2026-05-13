package lake

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestTaskRegistryExporterGetTaskThroughLakeExe(t *testing.T) {
	result := runTaskRegistryExporter(t, filepath.Clean("../.."), "--get", "task-034")
	if result.ExitCode != 0 {
		t.Fatalf("expected exporter success, got %+v", result)
	}

	var task map[string]any
	decodeJSONOutput(t, result, &task)
	if task["id"] != "task-034" {
		t.Fatalf("unexpected task id: %#v", task["id"])
	}
	if task["status"] != "done" {
		t.Fatalf("unexpected task status: %#v", task["status"])
	}
	if _, ok := task["tags"].([]any); !ok {
		t.Fatalf("tags field missing or not an array: %#v", task["tags"])
	}
	if _, ok := task["model_level"]; !ok {
		t.Fatalf("model_level field missing: %#v", task)
	}
}

func TestTaskRegistryGetThroughLakeServeRPC(t *testing.T) {
	repoRoot := filepath.Clean("../..")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lake", "serve")
	cmd.Dir = repoRoot
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("open lake serve stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("open lake serve stdout: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start lake serve: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		if ctx.Err() != nil && stderr.Len() > 0 {
			t.Logf("lake serve stderr: %s", stderr.String())
		}
	}()

	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		t.Fatalf("repo abs: %v", err)
	}
	leanFile := filepath.Join(repoAbs, "MCPAIHelperProject", "TaskRegistryExport.lean")
	leanText, err := os.ReadFile(leanFile)
	if err != nil {
		t.Fatalf("read Lean RPC source: %v", err)
	}
	uri := fileURI(leanFile)
	reader := bufio.NewReader(stdout)

	writeLSPRequest(t, stdin, 1, "initialize", map[string]any{"processId": nil, "rootUri": fileURI(repoAbs), "capabilities": map[string]any{}})
	_ = readLSPResponseBody(t, reader, stdin, 1)
	writeLSPNotification(t, stdin, "initialized", map[string]any{})
	writeLSPNotification(t, stdin, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "lean", "version": 1, "text": string(leanText)}})

	writeLSPRequest(t, stdin, 2, "$/lean/rpc/connect", map[string]any{"uri": uri})
	var connect struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(readLSPResponseBody(t, reader, stdin, 2), &connect); err != nil {
		t.Fatalf("decode rpc connect response: %v", err)
	}
	if connect.Result.SessionID == "" {
		t.Fatalf("empty rpc session id: %+v", connect)
	}

	line := strings.Count(string(leanText), "\n")
	if strings.HasSuffix(string(leanText), "\n") && line > 0 {
		line--
	}
	writeLSPRequest(t, stdin, 3, "$/lean/rpc/call", map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": 0},
		"sessionId":    connect.Result.SessionID,
		"method":       "MCPAIHelperProject.TaskRegistryExport.taskGet",
		"params":       map[string]any{"id": "task-034"},
	})
	var rpcCall struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(readLSPResponseBody(t, reader, stdin, 3), &rpcCall); err != nil {
		t.Fatalf("decode rpc call response: %v", err)
	}
	var envelope struct {
		OK   bool                       `json:"ok"`
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rpcCall.Result, &envelope); err != nil {
		t.Fatalf("decode registry envelope: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("registry envelope was not ok: %s", rpcCall.Result)
	}
	var id string
	if err := json.Unmarshal(envelope.Data["id"], &id); err != nil || id != "task-034" {
		t.Fatalf("unexpected task id %q err=%v envelope=%s", id, err, rpcCall.Result)
	}
	if _, ok := envelope.Data["acceptance_criteria"]; !ok {
		t.Fatalf("acceptance_criteria missing from server task payload: %s", rpcCall.Result)
	}
	if _, ok := envelope.Data["verification_plan"]; !ok {
		t.Fatalf("verification_plan missing from server task payload: %s", rpcCall.Result)
	}
	if _, ok := envelope.Data["model_level"]; !ok {
		t.Fatalf("model_level missing from server task payload: %s", rpcCall.Result)
	}
	writeLSPNotification(t, stdin, "$/lean/rpc/release", map[string]any{"uri": uri, "sessionId": connect.Result.SessionID, "refs": []any{}})
}

func TestTaskRegistryExporterListActiveThroughLakeExe(t *testing.T) {
	result := runTaskRegistryExporter(t, prepareLakeTestRepo(t), "--list-active")
	if result.ExitCode != 0 {
		t.Fatalf("expected exporter success, got %+v", result)
	}

	var payload struct {
		Tasks []map[string]any `json:"tasks"`
	}
	decodeJSONOutput(t, result, &payload)
	if len(payload.Tasks) == 0 {
		t.Fatalf("expected migrated active tasks, got none")
	}

	var migrated map[string]any
	for _, task := range payload.Tasks {
		if task["id"] == "task-006" {
			migrated = task
			break
		}
	}
	if migrated == nil {
		t.Fatalf("task-006 missing from active Lean export: %#v", payload.Tasks)
	}
	if migrated["status"] != "todo" || migrated["title"] == "" || migrated["body"] == "" {
		t.Fatalf("task-006 core fields were not preserved: %#v", migrated)
	}
	if tags, ok := migrated["tags"].([]any); !ok || len(tags) == 0 {
		t.Fatalf("task-006 tags were not preserved: %#v", migrated["tags"])
	}
}

func TestTaskRegistryExporterGetMissingTaskFails(t *testing.T) {
	result := runTaskRegistryExporter(t, filepath.Clean("../.."), "--get", "missing-task")
	if result.ExitCode == 0 {
		t.Fatalf("expected missing task failure, got %+v", result)
	}
	if !strings.Contains(strings.Join(result.Diagnostics, "\n"), "task not found") {
		t.Fatalf("expected not-found diagnostic, got %#v", result.Diagnostics)
	}
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeLSPRequest(t testing.TB, w io.Writer, id int, method string, params any) {
	t.Helper()
	writeLSPMessage(t, w, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

func writeLSPNotification(t testing.TB, w io.Writer, method string, params any) {
	t.Helper()
	writeLSPMessage(t, w, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func writeLSPResponse(t testing.TB, w io.Writer, id json.RawMessage, result any) {
	t.Helper()
	writeLSPMessage(t, w, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func lspServerRequestResult(method string) any {
	if method == "workspace/configuration" {
		return []any{}
	}
	return nil
}

func writeLSPMessage(t testing.TB, w io.Writer, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal lsp message: %v", err)
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		t.Fatalf("write lsp header: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write lsp body: %v", err)
	}
}

func readLSPResponseBody(t testing.TB, r *bufio.Reader, w io.Writer, id int) []byte {
	t.Helper()
	for {
		body := readLSPBody(t, r)
		var probe struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Error  *jsonRPCError   `json:"error"`
		}
		if err := json.Unmarshal(body, &probe); err != nil {
			t.Fatalf("decode lsp response probe %q: %v", string(body), err)
		}
		if probe.Method == "" && jsonIDEqualsInt(probe.ID, id) {
			if probe.Error != nil {
				t.Fatalf("lsp request %d failed: %d %s", id, probe.Error.Code, probe.Error.Message)
			}
			return body
		}
		if probe.Method != "" && len(probe.ID) > 0 {
			writeLSPResponse(t, w, probe.ID, lspServerRequestResult(probe.Method))
		}
	}
}

func jsonIDEqualsInt(raw json.RawMessage, expected int) bool {
	var actual int
	if err := json.Unmarshal(raw, &actual); err != nil {
		return false
	}
	return actual == expected
}

func readLSPBody(t testing.TB, r *bufio.Reader) []byte {
	t.Helper()
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read lsp header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			t.Fatalf("parse content length %q: %v", value, err)
		}
		contentLength = parsed
	}
	if contentLength < 0 {
		t.Fatal("lsp message missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("read lsp body: %v", err)
	}
	return body
}

func fileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func runTaskRegistryExporter(t *testing.T, repoRoot string, args ...string) CommandResult {
	t.Helper()
	runner := CommandRunner{
		Commands: command.NewRunner(config.CommandPolicy{
			AllowedCWDs:           []string{repoRoot},
			DefaultTimeoutSeconds: 20,
			MaxOutputBytes:        200000,
			MaxLines:              80,
		}),
		TimeoutSeconds: 20,
	}
	result, err := RunExe(context.Background(), repoRoot, "task_registry_export", args, runner)
	if err != nil {
		t.Fatalf("RunExe returned error: %v", err)
	}
	return result
}


func prepareLakeTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	realRoot := filepath.Clean("../..")
	for _, dir := range []string{"MCPAIHelperProject"} {
		if err := os.MkdirAll(filepath.Join(repo, dir), 0o700); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
	}
	for _, file := range []string{"lean-toolchain", "lakefile.lean", "MCPAIHelperProject.lean", "MCPAIHelperProject/ProjectState.lean", "MCPAIHelperProject/Samples.lean", "MCPAIHelperProject/Registry.lean", "MCPAIHelperProject/TaskRegistryExport.lean", "MCPAIHelperProject/ActiveTasks.lean"} {
		data, err := os.ReadFile(filepath.Join(realRoot, file))
		if err != nil {
			t.Fatalf("read fixture source %s: %v", file, err)
		}
		if err := os.WriteFile(filepath.Join(repo, file), data, 0o600); err != nil {
			t.Fatalf("write fixture file %s: %v", file, err)
		}
	}
	path := filepath.Join(repo, "MCPAIHelperProject", "ActiveTasks.lean")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ActiveTasks.lean fixture: %v", err)
	}
	source := string(data)
	source = strings.Replace(source, "status := .blocked,", "status := .proposed,", 1)
	source = strings.Replace(source, "modelLevel := some .high,\n", "", 1)
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write ActiveTasks.lean fixture: %v", err)
	}
	return repo
}

func decodeJSONOutput(t *testing.T, result CommandResult, target any) {

	t.Helper()
	output := strings.Join(result.Output, "\n")
	if output == "" {
		t.Fatalf("exporter produced no JSON output: %+v", result)
	}
	if err := json.Unmarshal([]byte(output), target); err != nil {
		t.Fatalf("decode exporter JSON %q: %v", output, err)
	}
}
