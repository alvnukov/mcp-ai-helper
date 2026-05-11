// Package main starts a stable stdio MCP wrapper for local development.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type toolCallParams struct {
	Name string `json:"name"`
}

type childManager struct {
	mu          sync.Mutex
	writeMu     sync.Mutex
	repoRoot    string
	configPath  string
	binaryPath  string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	initialized bool
	initLine    []byte
	readyLine   []byte
	suppress    map[string]bool
	injectTools map[string]bool
}

func main() {
	repoRoot := flag.String("repo", ".", "mcp-ai-helper repository root")
	configPath := flag.String("config", config.DefaultConfigPath(), "path to config yaml")
	buildOnStart := flag.Bool("build-on-start", true, "build child server before first start")
	flag.Parse()

	absRepo, err := filepath.Abs(*repoRoot)
	if err != nil {
		log.Fatalf("resolve repo: %v", err)
	}
	manager := &childManager{
		repoRoot:    absRepo,
		configPath:  *configPath,
		binaryPath:  filepath.Join(absRepo, "bin", "mcp-ai-helper"),
		suppress:    map[string]bool{},
		injectTools: map[string]bool{},
	}
	if *buildOnStart {
		if err := manager.build(); err != nil {
			log.Printf("initial build failed: %v", err)
		}
	}
	if err := manager.start(); err != nil {
		log.Printf("initial child start failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		manager.stop()
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if manager.handleLocal(line) {
			continue
		}
		if err := manager.forward(line); err != nil {
			writeRPCError(os.Stdout, nil, -32000, err.Error())
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("read stdin: %v", err)
	}
	manager.stop()
}

func (m *childManager) handleLocal(line []byte) bool {
	var msg rpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return false
	}
	idKey := string(msg.ID)
	switch msg.Method {
	case "initialize":
		m.mu.Lock()
		m.initLine = append([]byte(nil), line...)
		m.mu.Unlock()
	case "notifications/initialized":
		m.mu.Lock()
		m.initialized = true
		m.readyLine = append([]byte(nil), line...)
		m.mu.Unlock()
	case "tools/list":
		if idKey != "" {
			m.mu.Lock()
			m.injectTools[idKey] = true
			m.mu.Unlock()
		}
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return false
		}
		switch params.Name {
		case "dev_status":
			m.respondText(msg.ID, m.status())
			return true
		case "dev_rebuild_server":
			start := time.Now()
			err := m.rebuildAndRestart()
			if err != nil {
				m.respondText(msg.ID, fmt.Sprintf("rebuild failed: %v", err))
				return true
			}
			m.respondText(msg.ID, fmt.Sprintf("rebuilt and restarted child in %s", time.Since(start).Round(time.Millisecond)))
			return true
		case "dev_restart_server":
			start := time.Now()
			err := m.restart(false)
			if err != nil {
				m.respondText(msg.ID, fmt.Sprintf("restart failed: %v", err))
				return true
			}
			m.respondText(msg.ID, fmt.Sprintf("restarted child in %s", time.Since(start).Round(time.Millisecond)))
			return true
		}
	}
	return false
}

func (m *childManager) build() error {
	// #nosec G204 -- internal go build with fixed args
	cmd := exec.Command("go", "build", "-o", m.binaryPath, "./cmd/mcp-ai-helper")
	cmd.Dir = m.repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stringsTrim(stderr.String()))
	}
	return nil
}

func (m *childManager) rebuildAndRestart() error {
	m.stop()
	if err := m.build(); err != nil {
		return err
	}
	return m.start()
}

func (m *childManager) restart(rebuild bool) error {
	m.stop()
	if rebuild {
		if err := m.build(); err != nil {
			return err
		}
	}
	return m.start()
}

func (m *childManager) start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		return nil
	}
	// #nosec G204 -- internal binary restart with trusted config path
	cmd := exec.Command(m.binaryPath, "--config", m.configPath)
	cmd.Dir = m.repoRoot
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	m.cmd = cmd
	m.stdin = stdin
	go m.copyChildStdout(stdout)
	go copyWithPrefix(os.Stderr, stderr, "mcp-ai-helper child: ")
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
			m.stdin = nil
		}
		m.mu.Unlock()
		if err != nil {
			log.Printf("child exited: %v", err)
		}
	}()
	return m.bootstrapLocked()
}

func (m *childManager) bootstrapLocked() error {
	if !m.initialized || len(m.initLine) == 0 {
		return nil
	}
	initLine, err := rewriteRPCID(m.initLine, "__mcp_ai_helper_dev_init")
	if err != nil {
		return err
	}
	m.suppress[`"__mcp_ai_helper_dev_init"`] = true
	if _, err := m.stdin.Write(append(initLine, '\n')); err != nil {
		return err
	}
	if len(m.readyLine) > 0 {
		if _, err := m.stdin.Write(append(m.readyLine, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func (m *childManager) stop() {
	m.mu.Lock()
	cmd := m.cmd
	stdin := m.stdin
	m.cmd = nil
	m.stdin = nil
	m.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

func (m *childManager) forward(line []byte) error {
	m.mu.Lock()
	if m.cmd == nil || m.stdin == nil {
		if err := m.startLockedFromForward(); err != nil {
			m.mu.Unlock()
			return err
		}
	}
	stdin := m.stdin
	m.mu.Unlock()
	if _, err := stdin.Write(append(line, '\n')); err != nil {
		if restartErr := m.restart(false); restartErr != nil {
			return fmt.Errorf("write to child failed: %w; restart failed: %v", err, restartErr)
		}
		m.mu.Lock()
		stdin = m.stdin
		m.mu.Unlock()
		if stdin == nil {
			return fmt.Errorf("write to child failed: %w; child stdin unavailable after restart", err)
		}
		_, err = stdin.Write(append(line, '\n'))
		return err
	}
	return nil
}

func (m *childManager) startLockedFromForward() error {
	m.mu.Unlock()
	err := m.start()
	m.mu.Lock()
	return err
}

func (m *childManager) copyChildStdout(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		line = m.processChildLine(line)
		if line == nil {
			continue
		}
		m.writeLine(line)
	}
}

func (m *childManager) processChildLine(line []byte) []byte {
	var msg rpcMessage
	if err := json.Unmarshal(line, &msg); err != nil || len(msg.ID) == 0 {
		return line
	}
	idKey := string(msg.ID)
	m.mu.Lock()
	if m.suppress[idKey] {
		delete(m.suppress, idKey)
		m.mu.Unlock()
		return nil
	}
	injectTools := m.injectTools[idKey]
	if injectTools {
		delete(m.injectTools, idKey)
	}
	m.mu.Unlock()
	if !injectTools {
		return line
	}
	return appendDevTools(line)
}

func (m *childManager) writeLine(line []byte) {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	_, _ = os.Stdout.Write(append(line, '\n'))
}

func (m *childManager) respondText(id json.RawMessage, text string) {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result": map[string]any{
			"content": []map[string]string{{"type": "text", "text": text}},
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		writeRPCError(os.Stdout, id, -32603, err.Error())
		return
	}
	m.writeLine(data)
}

func (m *childManager) status() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	pid := 0
	if m.cmd != nil && m.cmd.Process != nil {
		pid = m.cmd.Process.Pid
	}
	return fmt.Sprintf("wrapper=ok child_pid=%d repo=%s binary=%s", pid, m.repoRoot, m.binaryPath)
}

func appendDevTools(line []byte) []byte {
	var response map[string]any
	if err := json.Unmarshal(line, &response); err != nil {
		return line
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		return line
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		return line
	}
	result["tools"] = append(tools, devTool("dev_status", "Return dev wrapper and child server status."), devTool("dev_restart_server", "Restart the child MCP server without closing wrapper stdio."), devTool("dev_rebuild_server", "Rebuild bin/mcp-ai-helper from source and restart the child MCP server."))
	data, err := json.Marshal(response)
	if err != nil {
		return line
	}
	return data
}

func devTool(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func rewriteRPCID(line []byte, id string) ([]byte, error) {
	var msg map[string]any
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, err
	}
	msg["id"] = id
	return json.Marshal(msg)
}

func writeRPCError(w io.Writer, id json.RawMessage, code int, message string) {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if len(id) > 0 {
		response["id"] = json.RawMessage(id)
	}
	data, _ := json.Marshal(response)
	_, _ = w.Write(append(data, '\n'))
}

func copyWithPrefix(dst io.Writer, src io.Reader, prefix string) {
	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		_, _ = fmt.Fprintf(dst, "%s%s\n", prefix, scanner.Text())
	}
}

func stringsTrim(value string) string {
	value = bytes.NewBufferString(value).String()
	for len(value) > 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == '\r' || value[len(value)-1] == '\t' || value[len(value)-1] == ' ') {
		value = value[:len(value)-1]
	}
	return value
}
