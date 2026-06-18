// Package command runs bounded local shell commands and extracts compact evidence.
package command

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/evidence"
	"github.com/zol/mcp-ai-helper/internal/security"
)

// Context keys for per-request secret injection.
type contextKey string

const (
	secretEnvsKey contextKey = "secret_envs"
	secretMaskKey contextKey = "secret_mask"
)

// ContextWithSecrets stores resolved secret env vars and mask in the context.
func ContextWithSecrets(ctx context.Context, envs []string, mask *security.Mask) context.Context {
	ctx = context.WithValue(ctx, secretEnvsKey, envs)
	ctx = context.WithValue(ctx, secretMaskKey, mask)
	return ctx
}

func secretsFromContext(ctx context.Context) ([]string, *security.Mask) {
	envs, _ := ctx.Value(secretEnvsKey).([]string)
	mask, _ := ctx.Value(secretMaskKey).(*security.Mask)
	return envs, mask
}

// Runner executes shell commands under repository and output policies.
type Runner struct {
	policy   config.CommandPolicy
	history  *History
	baseMask *security.Mask
	// running tracks active command cancel functions keyed by commandID.
	// Used by Abort to kill a running process.
	running sync.Map // map[string]context.CancelFunc
}

// Result is the compact, redacted command execution record returned to callers.
type Result struct {
	Status        string          `json:"status,omitempty"`
	CommandID     string          `json:"command_id"`
	Command       string          `json:"command"`
	CWD           string          `json:"cwd"`
	ExitCode      int             `json:"exit_code"`
	DurationMS    int64           `json:"duration_ms"`
	Truncated     bool            `json:"truncated"`
	StdoutTail    []string        `json:"stdout_tail"`
	StderrTail    []string        `json:"stderr_tail"`
	FilteredLines []string        `json:"filtered_lines,omitempty"`
	EvidenceLines []evidence.Line `json:"evidence_lines"`
	OutputHash    string          `json:"output_hash"`
	NextCall      *NextCall       `json:"next_call,omitempty"`
}

// NextCall tells the caller how to inspect a still-running durable command.
type NextCall struct {
	Tool      string `json:"tool"`
	CommandID string `json:"command_id"`
	Mode      string `json:"mode,omitempty"`
}

// Filter selects a compact slice from command output before it reaches the caller.
type Filter struct {
	Include         string   `json:"include"`
	Exclude         string   `json:"exclude"`
	CaseInsensitive bool     `json:"case_insensitive"`
	MaxLines        int      `json:"max_lines"`
	ContextBefore   int      `json:"context_before"`
	ContextAfter    int      `json:"context_after"`
	Preset          string   `json:"preset"`
	Packs           []string `json:"packs"`
	Regexes         []string `json:"regexes"`
	Keywords        []string `json:"keywords"`
}

// NewRunner creates a command runner from policy limits.
func NewRunner(policy config.CommandPolicy) *Runner {
	return NewRunnerWithMask(policy, nil)
}

// NewRunnerWithMask creates a command runner that redacts configured secrets from all retained output.
func NewRunnerWithMask(policy config.CommandPolicy, mask *security.Mask) *Runner {
	if policy.LogEnabled != nil && !*policy.LogEnabled {
		return &Runner{policy: policy, history: NewInMemoryHistory(), baseMask: mask}
	}
	history, err := NewHistory(HistoryPolicy{Dir: policy.LogDir, RetentionDays: policy.LogRetentionDays, MaxRecords: policy.LogMaxRecords, Compress: policy.LogCompress})
	if err != nil {
		history = NewInMemoryHistory()
	}
	return &Runner{policy: policy, history: history, baseMask: mask}
}

// Run executes cmd in cwd after validating cwd against the configured allowlist.
func (r *Runner) Run(ctx context.Context, cmd string, cwd string, timeoutSeconds int) (Result, error) {
	return r.RunFiltered(ctx, cmd, cwd, timeoutSeconds, Filter{})
}

// RunFiltered executes cmd and applies a deterministic grep-like output filter.
func (r *Runner) RunFiltered(ctx context.Context, cmd string, cwd string, timeoutSeconds int, filter Filter) (Result, error) {
	return r.RunFilteredWithWait(ctx, cmd, cwd, timeoutSeconds, 0, filter)
}

// RunFilteredWithWait runs a command with a separate MCP wait budget.
// If mcpWaitSeconds is exceeded, the process keeps running under its execution timeout
// and the caller receives a durable command_id for command_get/filter_command_history.
func (r *Runner) RunFilteredWithWait(ctx context.Context, cmd string, cwd string, timeoutSeconds int, mcpWaitSeconds int, filter Filter) (Result, error) {
	return r.runFilteredWithWait(ctx, cmd, cwd, timeoutSeconds, mcpWaitSeconds, filter, "")
}

func (r *Runner) runFiltered(ctx context.Context, cmd string, cwd string, timeoutSeconds int, filter Filter, repoPath string) (Result, error) {
	return r.runFilteredWithWait(ctx, cmd, cwd, timeoutSeconds, 0, filter, repoPath)
}

func (r *Runner) runFilteredWithWait(
	ctx context.Context,
	cmd string,
	cwd string,
	timeoutSeconds int,
	mcpWaitSeconds int,
	filter Filter,
	repoPath string,
) (Result, error) {
	if strings.TrimSpace(cmd) == "" {
		return Result{}, errors.New("command is required")
	}
	runCWD, err := r.safeCWD(cwd)
	if err != nil {
		return Result{}, err
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = r.policy.DefaultTimeoutSeconds
	}
	return r.runPreparedWithWait(ctx, cmd, runCWD, timeoutSeconds, mcpWaitSeconds, filter, repoPath)
}

func (r *Runner) runPreparedWithWait(ctx context.Context, cmd string, runCWD string, timeoutSeconds int, mcpWaitSeconds int, filter Filter, repoPath string) (Result, error) {
	commandID := newCommandID()
	started := time.Now().UTC()
	if mcpWaitSeconds > 0 {
		if err := r.history.Put(Record{
			CommandID: commandID,
			Status:    "running",
			RepoPath:  repoPath,
			Command:   r.maskText(ctx, cmd),
			CWD:       runCWD,
			ExitCode:  -1,
			StartedAt: started,
			CreatedAt: started,
		}); err != nil {
			return Result{}, err
		}
	}

	execute := func(execCtx context.Context) (Result, error) {
		return r.executePrepared(execCtx, commandID, cmd, runCWD, timeoutSeconds, filter, repoPath, started)
	}
	if mcpWaitSeconds <= 0 {
		return execute(ctx)
	}

	cmdCtx, cmdCancel := context.WithCancel(context.WithoutCancel(ctx))
	r.running.Store(commandID, cmdCancel)

	done := make(chan struct{})
	var result Result
	var err error
	go func() {
		result, err = execute(cmdCtx)
		r.running.Delete(commandID)
		close(done)
	}()

	timer := time.NewTimer(time.Duration(mcpWaitSeconds) * time.Second)
	defer timer.Stop()
	select {
	case <-done:
		return result, err
	case <-timer.C:
		return Result{
			Status:     "running",
			CommandID:  commandID,
			Command:    r.maskText(ctx, cmd),
			CWD:        runCWD,
			ExitCode:   -1,
			DurationMS: time.Since(started).Milliseconds(),
			NextCall:   nextCallForStatus("running", commandID),
		}, nil
	}
}

func (r *Runner) executePrepared(ctx context.Context, commandID string, cmd string, runCWD string, timeoutSeconds int, filter Filter, repoPath string, started time.Time) (Result, error) {
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	stdout := newLimitBuffer(r.policy.MaxOutputBytes)
	stderr := newLimitBuffer(r.policy.MaxOutputBytes)
	// #nosec G204 -- command execution is this package's explicit MCP capability and is constrained by cwd, timeout, and output policy.
	command := exec.CommandContext(runCtx, shellBin(), shellArgs(cmd)...)
	command.Dir = runCWD
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	envs, cmdMask := secretsFromContext(ctx)
	if len(envs) > 0 {
		command.Env = append(os.Environ(), envs...)
	}
	stop := context.AfterFunc(runCtx, func() {
		if command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	})
	defer stop()
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	completed := time.Now().UTC()
	duration := completed.Sub(started)

	exitCode := 0
	status := "ok"
	if err != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			exitCode = 124
			status = "timeout"
		case errors.As(err, &exitErr):
			exitCode = exitErr.ExitCode()
			status = "failed"
		default:
			return Result{}, err
		}
	}
	if exitCode != 0 && status == "ok" {
		status = "failed"
	}

	stdoutText := redact(stdout.String())
	stderrText := redact(stderr.String())
	if r.baseMask != nil {
		stdoutText = r.baseMask.Apply(stdoutText)
		stderrText = r.baseMask.Apply(stderrText)
	}
	if cmdMask != nil {
		stdoutText = cmdMask.Apply(stdoutText)
		stderrText = cmdMask.Apply(stderrText)
	}
	stdoutLines := normalizeLines(stdoutText)
	stderrLines := normalizeLines(stderrText)
	combined := append([]string{}, stdoutLines...)
	combined = append(combined, stderrLines...)
	truncatedLines := false
	if len(combined) > r.policy.MaxLines {
		truncatedLines = true
		combined = combined[len(combined)-r.policy.MaxLines:]
	}
	sum := sha256.Sum256([]byte(stdoutText + "\n" + stderrText))
	filteredLines, filterTruncated, err := applyFilter(combined, filter)
	if err != nil {
		return Result{}, err
	}
	outputHash := hex.EncodeToString(sum[:])
	commandStr := r.maskText(ctx, cmd)
	truncated := stdout.Truncated() || stderr.Truncated() || truncatedLines
	if err := r.history.Put(Record{
		CommandID:   commandID,
		Status:      status,
		RepoPath:    repoPath,
		Command:     commandStr,
		CWD:         runCWD,
		ExitCode:    exitCode,
		DurationMS:  duration.Milliseconds(),
		Truncated:   truncated,
		Stdout:      stdoutLines,
		Stderr:      stderrLines,
		Combined:    combined,
		OutputHash:  outputHash,
		StartedAt:   started,
		CompletedAt: completed,
	}); err != nil {
		return Result{}, err
	}
	return Result{
		Status:        status,
		CommandID:     commandID,
		Command:       commandStr,
		CWD:           runCWD,
		ExitCode:      exitCode,
		DurationMS:    duration.Milliseconds(),
		Truncated:     truncated || filterTruncated,
		StdoutTail:    tail80(stdoutLines),
		StderrTail:    tail80(stderrLines),
		FilteredLines: filteredLines,
		EvidenceLines: evidence.Select(combined, 30),
		OutputHash:    outputHash,
	}, nil
}

func (r *Runner) maskText(ctx context.Context, text string) string {
	out := text
	_, cmdMask := secretsFromContext(ctx)
	if r.baseMask != nil {
		out = r.baseMask.Apply(out)
	}
	if cmdMask != nil {
		out = cmdMask.Apply(out)
	}
	return out
}

func newCommandID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	sum := sha256.Sum256([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))
	return hex.EncodeToString(sum[:8])
}

func nextCallForStatus(status string, commandID string) *NextCall {
	if status != "running" || commandID == "" {
		return nil
	}
	return &NextCall{Tool: "command_get", CommandID: commandID, Mode: "status"}
}

// RunInRepo executes cmd in repoPath or a repo-relative cwd without allowing path escape.
func (r *Runner) RunInRepo(ctx context.Context, cmd string, repoPath string, cwd string, timeoutSeconds int) (Result, error) {
	return r.RunFilteredInRepo(ctx, cmd, repoPath, cwd, timeoutSeconds, Filter{})
}

// RunInRepoWithWait executes cmd in a repo with a separate MCP wait budget.
func (r *Runner) RunInRepoWithWait(ctx context.Context, cmd string, repoPath string, cwd string, timeoutSeconds int, mcpWaitSeconds int) (Result, error) {
	return r.RunFilteredInRepoWithWait(ctx, cmd, repoPath, cwd, timeoutSeconds, mcpWaitSeconds, Filter{})
}

// RunFilteredInRepo executes cmd in a repo and applies a deterministic output filter.
func (r *Runner) RunFilteredInRepo(ctx context.Context, cmd string, repoPath string, cwd string, timeoutSeconds int, filter Filter) (Result, error) {
	return r.RunFilteredInRepoWithWait(ctx, cmd, repoPath, cwd, timeoutSeconds, 0, filter)
}

// RunFilteredInRepoWithWait executes cmd in a repo with bounded MCP wait and durable lookup.
func (r *Runner) RunFilteredInRepoWithWait(ctx context.Context, cmd string, repoPath string, cwd string, timeoutSeconds int, mcpWaitSeconds int, filter Filter) (Result, error) {
	if strings.TrimSpace(repoPath) == "" {
		return Result{}, errors.New("repo_path is required")
	}
	if strings.TrimSpace(cmd) == "" {
		return Result{}, errors.New("command is required")
	}
	if err := rejectProtectedConfigCommand(cmd, r.policy.ProtectedConfigPath); err != nil {
		return Result{}, err
	}
	if err := rejectProtectedLeanCommand(cmd); err != nil {
		return Result{}, err
	}
	repo, err := resolveDir(repoPath)
	if err != nil {
		return Result{}, err
	}
	runCWD := repo
	if strings.TrimSpace(cwd) != "" {
		if filepath.IsAbs(cwd) {
			return Result{}, errors.New("cwd must be repo-relative when repo_path is set")
		}
		runCWD = filepath.Join(repo, filepath.Clean(cwd))
		if !insideDir(repo, runCWD) {
			return Result{}, fmt.Errorf("cwd %q escapes repo_path", cwd)
		}
	}
	runCWD, err = r.safeCWD(runCWD)
	if err != nil {
		return Result{}, err
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = r.policy.DefaultTimeoutSeconds
	}
	return r.runPreparedWithWait(ctx, cmd, runCWD, timeoutSeconds, mcpWaitSeconds, filter, repo)
}

// FilterHistory applies filter to a retained command output record.
func (r *Runner) FilterHistory(commandID string, filter Filter) (Result, error) {
	return r.history.Filter(commandID, filter)
}

// ListCommands returns a bounded list of command records from history.
func (r *Runner) ListCommands(req ListRequest) (ListResult, error) {
	return r.history.List(req)
}

// AbortResult reports the outcome of an abort attempt.
type AbortResult struct {
	Status  string `json:"status"`
	CommandID string `json:"command_id"`
	Reason  string `json:"reason,omitempty"`
}

// Abort kills a running command by its commandID.
// Returns ok if the process was killed, not_found if no such running command exists,
// or already_completed if the command already finished.
func (r *Runner) Abort(commandID string) (AbortResult, error) {
	if strings.TrimSpace(commandID) == "" {
		return AbortResult{}, errors.New("command_id is required")
	}
	// Check if the command is still running.
	val, ok := r.running.Load(commandID)
	if !ok {
		// Check if the command exists in history (completed).
		_, found, err := r.history.getRecord(commandID)
		if err != nil {
			return AbortResult{}, err
		}
		if found {
			return AbortResult{Status: "already_completed", CommandID: commandID, Reason: "command already finished"}, nil
		}
		return AbortResult{Status: "not_found", CommandID: commandID, Reason: "no such command"}, nil
	}
	cancel, ok := val.(context.CancelFunc)
	if !ok {
		return AbortResult{}, errors.New("invalid cancel function in process tracker")
	}
	cancel()
	r.running.Delete(commandID)
	// Update history record status.
	if err := r.history.UpdateStatus(commandID, "aborted"); err != nil {
		// Non-fatal: process was killed but status update failed.
		return AbortResult{Status: "ok", CommandID: commandID, Reason: "process killed, status update failed: " + err.Error()}, nil
	}
	return AbortResult{Status: "ok", CommandID: commandID}, nil
}

const protectedLeanCommandMessage = "policy_denied: command appears to access protected task registry source; this is a local command denial, not a global task blocker; use task tools or exclude protected registry files"

const protectedConfigCommandMessage = "current helper config cannot be edited from pipeline/command tools; use config_read/config_replace/config_reload config tools instead"

func rejectProtectedConfigCommand(cmd string, protectedPath string) error {
	normalized := normalizeCommandPath(cmd)
	for _, marker := range protectedConfigMarkers(protectedPath) {
		if marker != "" && strings.Contains(normalized, marker) {
			return fmt.Errorf("%s: command references %q", protectedConfigCommandMessage, marker)
		}
	}
	return nil
}

func protectedConfigMarkers(protectedPath string) []string {
	if strings.TrimSpace(protectedPath) == "" {
		protectedPath = config.DefaultConfigPath()
	}
	return []string{
		normalizeCommandPath(protectedPath),
		normalizeCommandPath(config.DefaultConfigPath()),
		"~/.mcp-ai-helper/config.yaml",
		".mcp-ai-helper/config.yaml",
	}
}

func rejectProtectedLeanCommand(cmd string) error {
	normalized := normalizeCommandPath(cmd)
	for _, marker := range protectedLeanCommandMarkers(normalized) {
		return fmt.Errorf("%s: command references %q", protectedLeanCommandMessage, marker)
	}
	return nil
}

func protectedLeanCommandMarkers(normalized string) []string {
	if strings.Contains(normalized, "mcpaihelperproject/activetasks.lean") {
		return []string{"mcpaihelperproject/activetasks.lean"}
	}
	if strings.Contains(normalized, "mcpaihelperproject/taskregistry") && strings.Contains(normalized, ".lean") {
		return []string{"mcpaihelperproject/taskregistry*.lean"}
	}
	if strings.Contains(normalized, "tasks/") && strings.Contains(normalized, ".lean") {
		return []string{"tasks/*.lean"}
	}
	return nil
}

func normalizeCommandPath(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
}

// CleanupHistory removes command log records that exceed retention policy limits.
// Safe to call multiple times; subsequent calls are no-ops when policy limits are satisfied.
func (r *Runner) CleanupHistory() error {
	return r.history.Cleanup()
}

func resolveDir(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = "."
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cwd %q is not a directory", abs)
	}
	return abs, nil
}

func (r *Runner) safeCWD(cwd string) (string, error) {
	abs, err := resolveDir(cwd)
	if err != nil {
		return "", err
	}
	for _, allowed := range r.policy.AllowedCWDs {
		var allowedAbs string
		if filepath.IsAbs(allowed) {
			allowedAbs, err = filepath.Abs(allowed)
		} else {
			allowedAbs, err = filepath.Abs(filepath.Join(abs, allowed))
		}
		if err != nil {
			continue
		}
		if abs == allowedAbs || strings.HasPrefix(abs, allowedAbs+string(os.PathSeparator)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("cwd %q is outside allowed_cwds", abs)
}

func insideDir(root string, child string) bool {
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

type limitBuffer struct {
	buf       bytes.Buffer
	maxBytes  int
	truncated bool
}

func newLimitBuffer(maxBytes int) *limitBuffer {
	if maxBytes <= 0 {
		maxBytes = 200000
	}
	return &limitBuffer{maxBytes: maxBytes}
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) <= b.maxBytes {
		_, _ = b.buf.Write(p)
		return len(p), nil
	}
	b.truncated = true
	remaining := b.maxBytes - b.buf.Len()
	if remaining > 0 {
		_, _ = b.buf.Write(p[:remaining])
	}
	return len(p), nil
}

func (b *limitBuffer) String() string {
	return b.buf.String()
}

func (b *limitBuffer) Truncated() bool {
	return b.truncated
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s]+`),
	regexp.MustCompile(`(?i)(x-api-key:\s*)[^\s]+`),
	regexp.MustCompile(`(?i)(private-token:\s*)[^\s]+`),
	regexp.MustCompile(`(?i)(api[_-]?key["']?\s*[:=]\s*["']?)[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)(token["']?\s*[:=]\s*["']?)[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)(secret["']?\s*[:=]\s*["']?)[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)(password["']?\s*[:=]\s*["']?)[^\s"']+`),
	regexp.MustCompile(`(?:AKIA|ASIA)[A-Z0-9]{14,}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9._-]{10,}`),
}

func redact(text string) string {
	out := text
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, `${1}[REDACTED]`)
	}
	return out
}

var builtInRegexPacks = map[string][]string{
	"errors-only": {
		`(?i)\b(error|failed|failure|panic|fatal|exception|timeout)\b`,
	},
	"test-failures": {
		`(?i)(^--- FAIL:|^FAIL\b|panic:|assert|expected|traceback|not equal|failure|failed|error:)`,
	},
	"compile-errors": {
		`(?i)(^#\s|:\d+:\d+:|:\d+:|undefined:|undeclared|syntax error|fatal error:|compilation terminated|build failed|cannot (find|use)|no required module provides package)`,
	},
	"git-status": {
		`^(##|[ MADRCU?!]{1,2}\s+)`,
	},
	"changed-files": {
		`^(?:[ MADRCU?!]{1,2}\s+\S|[ACDMRTUXB]\d*\s+\S|(?:\./)?[\w./-]+\.[A-Za-z0-9._-]+$|rename (?:from|to) |create mode |delete mode )`,
	},
	"summary-with-context": {
		`(?i)(^ok\b|^PASS\b|^FAIL\b|^Ran \d+ tests?|\bsummary\b|\btotal\b|\bfiles changed\b|\bchanged files\b|\bdone\b|\bfinished\b)`,
	},
}

func applyFilter(lines []string, filter Filter) ([]string, bool, error) {
	var err error
	filter, err = normalizeFilter(filter)
	if err != nil {
		return nil, false, err
	}
	if filter.Include == "" && filter.Exclude == "" && len(filter.Keywords) == 0 && len(filter.Regexes) == 0 {
		return nil, false, nil
	}
	include, err := compileFilterPattern(filter.Include, filter.CaseInsensitive)
	if err != nil {
		return nil, false, err
	}
	exclude, err := compileFilterPattern(filter.Exclude, filter.CaseInsensitive)
	if err != nil {
		return nil, false, err
	}
	regexes, err := compileFilterPatterns(filter.Regexes, filter.CaseInsensitive)
	if err != nil {
		return nil, false, err
	}
	selected := map[int]struct{}{}
	for i, line := range lines {
		if exclude != nil && exclude.MatchString(line) {
			continue
		}
		if include != nil && !include.MatchString(line) {
			continue
		}
		if len(filter.Keywords) > 0 && !matchesKeyword(line, filter.Keywords, filter.CaseInsensitive) {
			continue
		}
		if len(regexes) > 0 && !matchesRegexPack(line, regexes) {
			continue
		}
		start := max(0, i-filter.ContextBefore)
		end := min(len(lines)-1, i+filter.ContextAfter)
		for j := start; j <= end; j++ {
			selected[j] = struct{}{}
		}
	}
	indexes := make([]int, 0, len(selected))
	for index := range selected {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	truncated := false
	if filter.MaxLines > 0 && len(indexes) > filter.MaxLines {
		truncated = true
		indexes = indexes[:filter.MaxLines]
	}
	out := make([]string, 0, len(indexes))
	for _, index := range indexes {
		out = append(out, lines[index])
	}
	return out, truncated, nil
}

func normalizeFilter(filter Filter) (Filter, error) {
	switch filter.Preset {
	case "errors":
		filter.Preset = "errors-only"
	}
	if !hasPositiveSelectors(filter) {
		switch filter.Preset {
		case "errors-only":
			filter.Packs = appendUniqueStrings(filter.Packs, "errors-only")
		case "tests":
			filter.Include = `(?i)(FAIL|PASS|RUN|panic|error|failed|---)`
		case "test-failures":
			filter.Packs = appendUniqueStrings(filter.Packs, "test-failures")
		case "compile-errors":
			filter.Packs = appendUniqueStrings(filter.Packs, "compile-errors")
		case "git-status":
			filter.Packs = appendUniqueStrings(filter.Packs, "git-status")
		case "changed-files":
			filter.Packs = appendUniqueStrings(filter.Packs, "changed-files")
		case "summary-with-context":
			filter.Packs = appendUniqueStrings(filter.Packs, "summary-with-context")
		}
	}
	if filter.Preset == "summary-with-context" {
		if filter.ContextBefore <= 0 {
			filter.ContextBefore = 1
		}
		if filter.ContextAfter <= 0 {
			filter.ContextAfter = 1
		}
	}
	expanded, err := expandRegexPacks(filter.Packs)
	if err != nil {
		return Filter{}, err
	}
	filter.Regexes = appendUniqueStrings(filter.Regexes, expanded...)
	if filter.MaxLines <= 0 {
		filter.MaxLines = 80
	}
	return filter, nil
}

func hasPositiveSelectors(filter Filter) bool {
	return filter.Include != "" || len(filter.Keywords) > 0 || len(filter.Regexes) > 0 || len(filter.Packs) > 0
}

func expandRegexPacks(packs []string) ([]string, error) {
	patterns := make([]string, 0)
	for _, pack := range packs {
		if strings.TrimSpace(pack) == "" {
			continue
		}
		values, ok := builtInRegexPacks[pack]
		if !ok {
			return nil, fmt.Errorf("unknown filter pack %q", pack)
		}
		patterns = append(patterns, values...)
	}
	return patterns, nil
}

func compileFilterPattern(pattern string, caseInsensitive bool) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	if caseInsensitive && !strings.HasPrefix(pattern, "(?i)") {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func compileFilterPatterns(patterns []string, caseInsensitive bool) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := compileFilterPattern(pattern, caseInsensitive)
		if err != nil {
			return nil, err
		}
		if re != nil {
			compiled = append(compiled, re)
		}
	}
	return compiled, nil
}

func matchesKeyword(line string, keywords []string, caseInsensitive bool) bool {
	candidate := line
	if caseInsensitive {
		candidate = strings.ToLower(candidate)
	}
	for _, keyword := range keywords {
		value := keyword
		if caseInsensitive {
			value = strings.ToLower(value)
		}
		if strings.Contains(candidate, value) {
			return true
		}
	}
	return false
}

func matchesRegexPack(line string, regexes []*regexp.Regexp) bool {
	for _, re := range regexes {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func appendUniqueStrings(base []string, values ...string) []string {
	seen := make(map[string]struct{}, len(base)+len(values))
	out := make([]string, 0, len(base)+len(values))
	for _, value := range base {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func tail80(values []string) []string {
	const limit = 80
	if len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}

func shellBin() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "/bin/sh"
}

func shellArgs(cmd string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/c", cmd}
	}
	return []string{"-lc", cmd}
}
