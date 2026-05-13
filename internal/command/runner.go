// Package command runs bounded local shell commands and extracts compact evidence.
package command

import (
	"bytes"
	"context"
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
	"strings"
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
	policy  config.CommandPolicy
	history *History
}

// Result is the compact, redacted command execution record returned to callers.
type Result struct {
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
	if policy.LogEnabled != nil && !*policy.LogEnabled {
		return &Runner{policy: policy, history: NewInMemoryHistory()}
	}
	history, err := NewHistory(HistoryPolicy{Dir: policy.LogDir, RetentionDays: policy.LogRetentionDays, MaxRecords: policy.LogMaxRecords, Compress: policy.LogCompress})
	if err != nil {
		history = NewInMemoryHistory()
	}
	return &Runner{policy: policy, history: history}
}

// Run executes cmd in cwd after validating cwd against the configured allowlist.
func (r *Runner) Run(ctx context.Context, cmd string, cwd string, timeoutSeconds int) (Result, error) {
	return r.RunFiltered(ctx, cmd, cwd, timeoutSeconds, Filter{})
}

// RunFiltered executes cmd and applies a deterministic grep-like output filter.
func (r *Runner) RunFiltered(ctx context.Context, cmd string, cwd string, timeoutSeconds int, filter Filter) (Result, error) {
	return r.runFiltered(ctx, cmd, cwd, timeoutSeconds, filter, "")
}

func (r *Runner) runFiltered(
	ctx context.Context,
	cmd string,
	cwd string,
	timeoutSeconds int,
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
	// Kill process group on context cancellation so no orphans survive.
	stop := context.AfterFunc(runCtx, func() {
		if command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
	})
	defer stop()
	command.Stdout = stdout
	command.Stderr = stderr

	started := time.Now()
	err = command.Run()
	duration := time.Since(started)

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		switch {
		case errors.As(err, &exitErr):
			exitCode = exitErr.ExitCode()
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			exitCode = 124
		default:
			return Result{}, err
		}
	}

	stdoutText := redact(stdout.String())
	stderrText := redact(stderr.String())
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
	commandID := hex.EncodeToString(sum[:8])
	outputHash := hex.EncodeToString(sum[:])
	if err := r.history.Put(Record{
		CommandID:  commandID,
		RepoPath:   repoPath,
		Command:    cmd,
		CWD:        runCWD,
		ExitCode:   exitCode,
		Truncated:  stdout.Truncated() || stderr.Truncated() || truncatedLines,
		Stdout:     stdoutLines,
		Stderr:     stderrLines,
		Combined:   combined,
		OutputHash: outputHash,
	}); err != nil {
		return Result{}, err
	}
	commandStr := cmd
	if cmdMask != nil {
		commandStr = cmdMask.Apply(commandStr)
	}
	return Result{
		CommandID:     commandID,
		Command:       commandStr,
		CWD:           runCWD,
		ExitCode:      exitCode,
		DurationMS:    duration.Milliseconds(),
		Truncated:     stdout.Truncated() || stderr.Truncated() || truncatedLines || filterTruncated,
		StdoutTail:    tail80(stdoutLines),
		StderrTail:    tail80(stderrLines),
		FilteredLines: filteredLines,
		EvidenceLines: evidence.Select(combined, 30),
		OutputHash:    outputHash,
	}, nil
}

// RunInRepo executes cmd in repoPath or a repo-relative cwd without allowing path escape.
func (r *Runner) RunInRepo(ctx context.Context, cmd string, repoPath string, cwd string, timeoutSeconds int) (Result, error) {
	return r.RunFilteredInRepo(ctx, cmd, repoPath, cwd, timeoutSeconds, Filter{})
}

// RunFilteredInRepo executes cmd in a repo and applies a deterministic output filter.
func (r *Runner) RunFilteredInRepo(ctx context.Context, cmd string, repoPath string, cwd string, timeoutSeconds int, filter Filter) (Result, error) {
	if strings.TrimSpace(repoPath) == "" {
		return Result{}, errors.New("repo_path is required")
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
	return r.runFiltered(ctx, cmd, runCWD, timeoutSeconds, filter, repo)
}

// FilterHistory applies filter to a retained command output record.
func (r *Runner) FilterHistory(commandID string, filter Filter) (Result, error) {
	return r.history.Filter(commandID, filter)
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
