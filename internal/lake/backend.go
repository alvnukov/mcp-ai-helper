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
	"sync"
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

var defaultServerManager = newServerManager()

type serverManager struct {
	mu      sync.Mutex
	servers map[string]*serverProcess
}

type serverProcess struct {
	mu             sync.Mutex
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	reader         *bufio.Reader
	stderr         *serverLimitBuffer
	waitDone       chan struct{}
	nextID         int
	openedVersions map[string]int
}

func newServerManager() *serverManager {
	return &serverManager{servers: map[string]*serverProcess{}}
}

// CallServerRPC calls one Lean RPC method through a shared per-workspace lake serve process.
func CallServerRPC(ctx context.Context, repoPath string, req RPCRequest) (RPCResult, error) {
	return defaultServerManager.CallRPC(ctx, repoPath, req)
}

// ResetServerRPC drops the shared lake serve process for a workspace after an RPC mutates imported Lean files.
func ResetServerRPC(repoPath string) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return
	}
	defaultServerManager.reset(ws.Dir)
}

func (m *serverManager) CallRPC(ctx context.Context, repoPath string, req RPCRequest) (RPCResult, error) {
	ws, err := ResolveWorkspace(repoPath)
	if err != nil {
		return RPCResult{WorkspaceDetected: false, Blocker: err.Error()}, nil
	}
	sourceRel, sourceAbs, sourceBlocker := resolveRPCSource(ws.Dir, req.SourceFile)
	if sourceBlocker != "" {
		return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Blocker: sourceBlocker}, nil
	}
	// #nosec G304 -- sourceAbs from resolveRPCSource, validated workspace path.
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

	server, err := m.server(runCtx, ws)
	if err != nil {
		return RPCResult{}, err
	}
	result, err := server.call(runCtx, ws, sourceAbs, string(source), method, req.Params)
	if err != nil {
		m.drop(ws.Dir, server)
		if ctxErr := runCtx.Err(); ctxErr != nil {
			err = ctxErr
		}
		return serverRPCBlocker(ws, method, server.stderr, err), nil
	}
	return result, nil
}

func (m *serverManager) server(ctx context.Context, ws Workspace) (*serverProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if server := m.servers[ws.Dir]; server != nil {
		return server, nil
	}
	server, err := startServerProcess(ctx, ws)
	if err != nil {
		return nil, err
	}
	m.servers[ws.Dir] = server
	return server, nil
}

func (m *serverManager) drop(workspaceDir string, server *serverProcess) {
	m.mu.Lock()
	if current := m.servers[workspaceDir]; current == server {
		delete(m.servers, workspaceDir)
	}
	m.mu.Unlock()
	server.terminate()
}

func (m *serverManager) reset(workspaceDir string) {
	m.mu.Lock()
	server := m.servers[workspaceDir]
	delete(m.servers, workspaceDir)
	m.mu.Unlock()
	if server != nil {
		server.terminate()
	}
}

func startServerProcess(ctx context.Context, ws Workspace) (*serverProcess, error) {
	// #nosec G204 -- lake serve is the fixed Lean server executable for a validated Lake workspace.
	cmd := exec.Command("lake", "serve")
	cmd.Dir = ws.Dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open lake serve stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open lake serve stdout: %w", err)
	}
	stderr := newServerLimitBuffer(40000)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start lake serve: %w", err)
	}
	server := &serverProcess{
		cmd:            cmd,
		stdin:          stdin,
		reader:         bufio.NewReader(stdout),
		stderr:         stderr,
		waitDone:       make(chan struct{}),
		nextID:         1,
		openedVersions: map[string]int{},
	}
	go func() {
		_ = cmd.Wait()
		close(server.waitDone)
	}()
	if err := server.initialize(ctx, ws); err != nil {
		server.terminate()
		return nil, err
	}
	return server, nil
}

func (p *serverProcess) initialize(ctx context.Context, ws Workspace) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	stop := context.AfterFunc(ctx, p.terminate)
	defer stop()

	id := p.nextRequestID()
	if err := writeServerLSPRequest(p.stdin, id, "initialize", map[string]any{"processId": nil, "rootUri": serverFileURI(ws.Dir), "capabilities": map[string]any{}}); err != nil {
		return err
	}
	if _, err := readServerLSPResponseBody(p.reader, p.stdin, id); err != nil {
		return err
	}
	return writeServerLSPNotification(p.stdin, "initialized", map[string]any{})
}

func (p *serverProcess) call(ctx context.Context, ws Workspace, sourceAbs string, sourceText string, method string, params any) (RPCResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stop := context.AfterFunc(ctx, p.terminate)
	defer stop()
	if err := ctx.Err(); err != nil {
		return RPCResult{}, err
	}

	sourceURI := serverFileURI(sourceAbs)
	if err := p.syncDocument(sourceURI, sourceText); err != nil {
		return RPCResult{}, err
	}
	connectID := p.nextRequestID()
	if err := writeServerLSPRequest(p.stdin, connectID, "$/lean/rpc/connect", map[string]any{"uri": sourceURI}); err != nil {
		return RPCResult{}, err
	}
	connectBody, err := readServerLSPResponseBody(p.reader, p.stdin, connectID)
	if err != nil {
		return RPCResult{}, err
	}
	var connect struct {
		Result struct {
			SessionID string `json:"sessionId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(connectBody, &connect); err != nil {
		return RPCResult{}, fmt.Errorf("decode rpc connect response: %w", err)
	}
	if connect.Result.SessionID == "" {
		return RPCResult{}, errors.New("empty Lean RPC session id")
	}

	line := strings.Count(sourceText, "\n")
	if strings.HasSuffix(sourceText, "\n") && line > 0 {
		line--
	}
	callID := p.nextRequestID()
	if err := writeServerLSPRequest(p.stdin, callID, "$/lean/rpc/call", map[string]any{
		"textDocument": map[string]any{"uri": sourceURI},
		"position":     map[string]any{"line": line, "character": 0},
		"sessionId":    connect.Result.SessionID,
		"method":       method,
		"params":       params,
	}); err != nil {
		return RPCResult{}, err
	}
	callBody, err := readServerLSPResponseBody(p.reader, p.stdin, callID)
	if err != nil {
		return RPCResult{}, err
	}
	var call struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(callBody, &call); err != nil {
		return RPCResult{}, fmt.Errorf("decode rpc call response: %w", err)
	}
	if len(call.Result) == 0 {
		return RPCResult{}, errors.New("Lean RPC call returned no result")
	}
	_ = writeServerLSPNotification(p.stdin, "$/lean/rpc/release", map[string]any{"uri": sourceURI, "sessionId": connect.Result.SessionID, "refs": []any{}})
	return RPCResult{WorkspaceDetected: true, WorkspaceDir: ws.Dir, Method: method, Result: append(json.RawMessage(nil), call.Result...), Diagnostics: serverDiagnostics(p.stderr)}, nil
}

func (p *serverProcess) syncDocument(sourceURI string, sourceText string) error {
	version := p.openedVersions[sourceURI] + 1
	if version == 1 {
		if err := writeServerLSPNotification(p.stdin, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": sourceURI, "languageId": "lean", "version": version, "text": sourceText}}); err != nil {
			return err
		}
	} else {
		if err := writeServerLSPNotification(p.stdin, "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": sourceURI, "version": version}, "contentChanges": []map[string]string{{"text": sourceText}}}); err != nil {
			return err
		}
	}
	p.openedVersions[sourceURI] = version
	return nil
}

func (p *serverProcess) nextRequestID() int {
	id := p.nextID
	p.nextID++
	return id
}

func (p *serverProcess) terminate() {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.waitDone:
	case <-time.After(2 * time.Second):
	}
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
