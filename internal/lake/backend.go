package lake

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/command"
)

// Workspace is a repo-local Lean/Lake project root.
type Workspace struct {
	Dir string `json:"dir"`
}

// CommandResult is the compact, MCP-facing shape returned by the backend.
type CommandResult struct {
	WorkspaceDetected bool     `json:"workspace_detected"`
	WorkspaceDir      string   `json:"workspace_dir,omitempty"`
	Command           []string `json:"command,omitempty"`
	ExitCode          int      `json:"exit_code"`
	Output            []string `json:"output,omitempty"`
	Diagnostics       []string `json:"diagnostics,omitempty"`
	Blocker           string   `json:"blocker,omitempty"`
}

// Runner executes a Lake command through the helper's bounded command layer.
type Runner interface {
	Run(ctx context.Context, workspaceDir string, args []string) (CommandResult, error)
}

// RPCRequest describes one transient Lean server RPC call through lake serve.
type RPCRequest struct {
	SourceFile     string `json:"source_file"`
	Method         string `json:"method"`
	Params         any    `json:"params,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// RPCResult is the compact result of one Lean server RPC call.
type RPCResult struct {
	WorkspaceDetected bool            `json:"workspace_detected"`
	WorkspaceDir      string          `json:"workspace_dir,omitempty"`
	Method            string          `json:"method,omitempty"`
	Result            json.RawMessage `json:"result,omitempty"`
	Diagnostics       []string        `json:"diagnostics,omitempty"`
	Blocker           string          `json:"blocker,omitempty"`
}

// CommandRunner adapts internal/command.Runner for Lake checks.
type CommandRunner struct {
	Commands       *command.Runner
	TimeoutSeconds int
}

func ResolveWorkspace(repoPath string) (Workspace, error) {
	if strings.TrimSpace(repoPath) == "" {
		return Workspace{}, errors.New("repo_path is required")
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return Workspace{}, fmt.Errorf("resolve repo_path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Workspace{}, fmt.Errorf("Lake workspace blocker: repo_path is not accessible: %w", err)
	}
	if !info.IsDir() {
		return Workspace{}, fmt.Errorf("Lake workspace blocker: repo_path is not a directory: %s", abs)
	}
	if _, err := os.Stat(filepath.Join(abs, "lean-toolchain")); err != nil {
		return Workspace{}, errors.New("Lake workspace blocker: missing lean-toolchain")
	}
	if _, err := os.Stat(filepath.Join(abs, "lakefile.lean")); err == nil {
		return Workspace{Dir: abs}, nil
	}
	if _, err := os.Stat(filepath.Join(abs, "lakefile.toml")); err == nil {
		return Workspace{Dir: abs}, nil
	}
	return Workspace{}, errors.New("Lake workspace blocker: missing lakefile.lean or lakefile.toml")
}

func Build(ctx context.Context, repoPath string, runner Runner) (CommandResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return blockerResult(err), nil
	}
	if runner == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake runner is not configured"}, nil
	}
	return runner.Run(ctx, ws.Dir, []string{"lake", "build"})
}

func CheckFile(ctx context.Context, repoPath string, relLeanFile string, runner Runner) (CommandResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return blockerResult(err), nil
	}
	relClean := filepath.Clean(relLeanFile)
	if filepath.IsAbs(relClean) || relClean == "." || strings.HasPrefix(relClean, ".."+string(os.PathSeparator)) || relClean == ".." {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lean file must be repo-relative and inside the workspace"}, nil
	}
	if filepath.Ext(relClean) != ".lean" {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lean file must have .lean extension"}, nil
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, relClean)); err != nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lean file is not accessible: " + relClean}, nil
	}
	if runner == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake runner is not configured"}, nil
	}
	return runner.Run(ctx, ws.Dir, []string{"lake", "env", "lean", relClean})
}

func RunExe(ctx context.Context, repoPath string, exeName string, exeArgs []string, runner Runner) (CommandResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return blockerResult(err), nil
	}
	exe := strings.TrimSpace(exeName)
	if exe == "" || strings.ContainsAny(exe, `/\`) {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake executable name must be non-empty and not contain a path"}, nil
	}
	if runner == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, ExitCode: -1, Blocker: "Lake runner is not configured"}, nil
	}
	args := append([]string{"lake", "exe", exe}, exeArgs...)
	return runner.Run(ctx, ws.Dir, args)
}

// CallServerRPC starts a bounded transient lake serve process and calls one Lean RPC method.
func CallServerRPC(ctx context.Context, repoPath string, req RPCRequest) (RPCResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return RPCResult{WorkspaceDetected: false, Blocker: err.Error()}, nil
	}
	sourceRel, sourceAbs, sourceBlocker := resolveRPCSource(ws.Dir, req.SourceFile)
	if sourceBlocker != "" {
		return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Blocker: sourceBlocker}, nil
	}
	source, err := os.ReadFile(sourceAbs)
	if err != nil {
		return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Blocker: "Lean RPC source is not accessible: " + sourceRel}, nil
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Blocker: "Lean RPC method is required"}, nil
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 20
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// #nosec G204 -- lake serve is the fixed Lean server executable for a validated Lake workspace.
	cmd := exec.CommandContext(runCtx, "lake", "serve")
	cmd.Dir = ws.Dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return RPCResult{}, fmt.Errorf("open lake serve stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return RPCResult{}, fmt.Errorf("open lake serve stdout: %w", err)
	}
	stderr := newServerLimitBuffer(40000)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return RPCResult{}, fmt.Errorf("start lake serve: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	repoURI := serverFileURI(ws.Dir)
	sourceURI := serverFileURI(sourceAbs)
	reader := bufio.NewReader(stdout)
	if err := writeServerLSPRequest(stdin, 1, "initialize", map[string]any{"processId": nil, "rootUri": repoURI, "capabilities": map[string]any{}}); err != nil {
		return RPCResult{}, err
	}
	if _, err := readServerLSPResponseBody(reader, stdin, 1); err != nil {
		return serverRPCBlocker(ws, method, stderr, err), nil
	}
	if err := writeServerLSPNotification(stdin, "initialized", map[string]any{}); err != nil {
		return RPCResult{}, err
	}
	sourceText := string(source)
	if err := writeServerLSPNotification(stdin, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": sourceURI, "languageId": "lean", "version": 1, "text": sourceText}}); err != nil {
		return RPCResult{}, err
	}
	if err := writeServerLSPRequest(stdin, 2, "$/lean/rpc/connect", map[string]any{"uri": sourceURI}); err != nil {
		return RPCResult{}, err
	}
	connectBody, err := readServerLSPResponseBody(reader, stdin, 2)
	if err != nil {
		return serverRPCBlocker(ws, method, stderr, err), nil
	}
	var connect struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(connectBody, &connect); err != nil {
		return serverRPCBlocker(ws, method, stderr, fmt.Errorf("decode rpc connect response: %w", err)), nil
	}
	if connect.Result.SessionID == "" {
		return serverRPCBlocker(ws, method, stderr, errors.New("empty Lean RPC session id")), nil
	}

	line := strings.Count(sourceText, "\n")
	if strings.HasSuffix(sourceText, "\n") && line > 0 {
		line--
	}
	if err := writeServerLSPRequest(stdin, 3, "$/lean/rpc/call", map[string]any{
		"textDocument": map[string]any{"uri": sourceURI},
		"position":     map[string]any{"line": line, "character": 0},
		"sessionId":    connect.Result.SessionID,
		"method":       method,
		"params":       req.Params,
	}); err != nil {
		return RPCResult{}, err
	}
	callBody, err := readServerLSPResponseBody(reader, stdin, 3)
	if err != nil {
		return serverRPCBlocker(ws, method, stderr, err), nil
	}
	var call struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(callBody, &call); err != nil {
		return serverRPCBlocker(ws, method, stderr, fmt.Errorf("decode rpc call response: %w", err)), nil
	}
	if len(call.Result) == 0 {
		return serverRPCBlocker(ws, method, stderr, errors.New("Lean RPC call returned no result")), nil
	}
	_ = writeServerLSPNotification(stdin, "$/lean/rpc/release", map[string]any{"uri": sourceURI, "sessionId": connect.Result.SessionID, "refs": []any{}})
	return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Method: method, Result: append(json.RawMessage(nil), call.Result...), Diagnostics: serverDiagnostics(stderr)}, nil
}

func (r CommandRunner) Run(ctx context.Context, workspaceDir string, args []string) (CommandResult, error) {
	if r.Commands == nil {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: workspaceDir, ExitCode: -1, Blocker: "command runner is not configured"}, nil
	}
	if len(args) == 0 {
		return CommandResult{WorkspaceDetected: true, WorkspaceDir: workspaceDir, ExitCode: -1, Blocker: "empty Lake command"}, nil
	}
	result, err := r.Commands.RunFilteredInRepo(ctx, shellQuote(args), workspaceDir, "", r.TimeoutSeconds, command.Filter{Keywords: []string{"error", "warning", "failed", "unknown", "invalid", "not found"}, CaseInsensitive: true, MaxLines: 40})
	if err != nil {
		return CommandResult{}, err
	}
	diagnostics := result.FilteredLines
	if len(diagnostics) == 0 {
		diagnostics = FilterDiagnostics(strings.Join(append(result.StdoutTail, result.StderrTail...), "\n"))
	}
	return CommandResult{WorkspaceDetected: true, WorkspaceDir: workspaceDir, Command: append([]string(nil), args...), ExitCode: result.ExitCode, Output: append([]string(nil), result.StdoutTail...), Diagnostics: diagnostics}, nil
}

func FilterDiagnostics(output string) []string {
	if strings.TrimSpace(output) == "" {
		return nil
	}
	lines := strings.Split(output, "\n")
	diagnostics := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" {
			continue
		}
		if strings.Contains(lower, "error") || strings.Contains(lower, "warning") || strings.Contains(lower, "failed") || strings.Contains(lower, "unknown") || strings.Contains(lower, "invalid") || strings.Contains(lower, "not found") {
			diagnostics = append(diagnostics, trimmed)
			if len(diagnostics) == 40 {
				break
			}
		}
	}
	if len(diagnostics) == 0 {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				diagnostics = append(diagnostics, trimmed)
				if len(diagnostics) == 12 {
					break
				}
			}
		}
	}
	return diagnostics
}

func blockerResult(err error) CommandResult {
	return CommandResult{WorkspaceDetected: false, ExitCode: -1, Blocker: err.Error()}
}

func shellQuote(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "" {
			parts = append(parts, "''")
			continue
		}
		if strings.IndexFunc(arg, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' || r == '/' || r == ':')
		}) == -1 {
			parts = append(parts, arg)
			continue
		}
		parts = append(parts, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
	}
	return strings.Join(parts, " ")
}

func resolveRPCSource(workspaceDir string, sourceFile string) (string, string, string) {
	rel := filepath.Clean(strings.TrimSpace(sourceFile))
	if rel == "" || rel == "." {
		return "", "", "Lean RPC source_file is required"
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", "Lean RPC source_file must be repo-relative and inside the workspace"
	}
	if filepath.Ext(rel) != ".lean" {
		return "", "", "Lean RPC source_file must have .lean extension"
	}
	abs, err := filepath.Abs(filepath.Join(workspaceDir, rel))
	if err != nil {
		return "", "", "resolve Lean RPC source_file: " + err.Error()
	}
	if !pathInside(workspaceDir, abs) {
		return "", "", "Lean RPC source_file escapes workspace"
	}
	return filepath.ToSlash(rel), abs, ""
}

func pathInside(root string, child string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	return childAbs == rootAbs || strings.HasPrefix(childAbs, rootAbs+string(os.PathSeparator))
}

func serverRPCBlocker(ws Workspace, method string, stderr *serverLimitBuffer, err error) RPCResult {
	return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Method: method, Diagnostics: serverDiagnostics(stderr), Blocker: err.Error()}
}

type serverJSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeServerLSPRequest(w io.Writer, id int, method string, params any) error {
	return writeServerLSPMessage(w, map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
}

func writeServerLSPNotification(w io.Writer, method string, params any) error {
	return writeServerLSPMessage(w, map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func writeServerLSPResponse(w io.Writer, id json.RawMessage, result any) error {
	return writeServerLSPMessage(w, map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func writeServerLSPMessage(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readServerLSPResponseBody(r *bufio.Reader, w io.Writer, id int) ([]byte, error) {
	for {
		body, err := readServerLSPBody(r)
		if err != nil {
			return nil, err
		}
		var probe struct {
			ID     json.RawMessage     `json:"id"`
			Method string              `json:"method"`
			Error  *serverJSONRPCError `json:"error"`
		}
		if err := json.Unmarshal(body, &probe); err != nil {
			return nil, fmt.Errorf("decode lsp response probe: %w", err)
		}
		if probe.Method == "" && serverJSONIDEqualsInt(probe.ID, id) {
			if probe.Error != nil {
				return nil, fmt.Errorf("lsp request %d failed: %d %s", id, probe.Error.Code, probe.Error.Message)
			}
			return body, nil
		}
		if probe.Method != "" && len(probe.ID) > 0 {
			if err := writeServerLSPResponse(w, probe.ID, serverLSPRequestResult(probe.Method)); err != nil {
				return nil, err
			}
		}
	}
}

func serverJSONIDEqualsInt(raw json.RawMessage, expected int) bool {
	var actual int
	if err := json.Unmarshal(raw, &actual); err != nil {
		return false
	}
	return actual == expected
}

func readServerLSPBody(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read lsp header: %w", err)
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
			return nil, fmt.Errorf("parse content length %q: %w", value, err)
		}
		contentLength = parsed
	}
	if contentLength < 0 {
		return nil, errors.New("lsp message missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read lsp body: %w", err)
	}
	return body, nil
}

func serverFileURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func serverLSPRequestResult(method string) any {
	if method == "workspace/configuration" {
		return []any{}
	}
	return nil
}

func serverDiagnostics(stderr *serverLimitBuffer) []string {
	if stderr == nil {
		return nil
	}
	diagnostics := FilterDiagnostics(stderr.String())
	if stderr.truncated {
		diagnostics = append(diagnostics, "lake serve stderr truncated")
	}
	return diagnostics
}

type serverLimitBuffer struct {
	bytes.Buffer
	maxBytes  int
	truncated bool
}

func newServerLimitBuffer(maxBytes int) *serverLimitBuffer {
	if maxBytes <= 0 {
		maxBytes = 40000
	}
	return &serverLimitBuffer{maxBytes: maxBytes}
}

func (b *serverLimitBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) <= b.maxBytes {
		_, _ = b.Buffer.Write(p)
		return len(p), nil
	}
	remaining := b.maxBytes - b.Len()
	if remaining > 0 {
		_, _ = b.Buffer.Write(p[:remaining])
	}
	b.truncated = true
	return len(p), nil
}
