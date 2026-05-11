package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/project"
)

func TestRunnerRejectsCWDOutsidePolicy(t *testing.T) {
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{t.TempDir()}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20})
	_, err := runner.Run(t.Context(), "echo ok", "/", 1)
	if err == nil {
		t.Fatal("expected cwd policy error")
	}
}

func TestRunnerExtractsEvidence(t *testing.T) {
	dir := t.TempDir()
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{dir}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20})
	result, err := runner.Run(t.Context(), "printf 'ok\\nerror: bad\\n'", dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.EvidenceLines) == 0 {
		t.Fatal("expected evidence lines")
	}
}

func TestRunnerStoresHistoryAndRefilters(t *testing.T) {
	dir := t.TempDir()
	logDir := t.TempDir()
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{dir}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20, LogDir: logDir})
	result, err := runner.RunFiltered(t.Context(), "printf 'info\\nerror: bad\\nwarn\\n'", dir, 1, Filter{Preset: "errors"})
	if err != nil {
		t.Fatal(err)
	}
	if result.CommandID == "" {
		t.Fatal("expected command id")
	}
	if len(result.FilteredLines) != 1 || result.FilteredLines[0] != "error: bad" {
		t.Fatalf("filtered lines = %#v", result.FilteredLines)
	}
	refiltered, err := runner.FilterHistory(result.CommandID, Filter{Include: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refiltered.FilteredLines) != 1 || refiltered.FilteredLines[0] != "warn" {
		t.Fatalf("refiltered lines = %#v", refiltered.FilteredLines)
	}
}

func TestRunnerStoresRepoHistoryUnderProjectLogs(t *testing.T) {
	repoPath := t.TempDir()
	logRoot := t.TempDir()
	policy := config.CommandPolicy{
		AllowedCWDs:           []string{repoPath},
		DefaultTimeoutSeconds: 1,
		MaxOutputBytes:        1000,
		MaxLines:              20,
		LogDir:                logRoot,
	}
	runner := NewRunner(policy)
	result, err := runner.RunFilteredInRepo(t.Context(), "printf 'ok\\nwarn: keep\\n'", repoPath, "", 1, Filter{Include: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	projectName, err := project.Name(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(logRoot, "repos", projectName, "logs", "index.jsonl")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("stat repo log index: %v", err)
	}
	reloaded := NewRunner(policy)
	refiltered, err := reloaded.FilterHistory(result.CommandID, Filter{Include: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if len(refiltered.FilteredLines) != 1 || refiltered.FilteredLines[0] != "ok" {
		t.Fatalf("refiltered lines = %#v", refiltered.FilteredLines)
	}
}

func TestHistoryCleanupRemovesRecordsBeyondLimits(t *testing.T) {
	repoPath := t.TempDir()
	logRoot := t.TempDir()
	policy := config.CommandPolicy{
		AllowedCWDs:           []string{repoPath},
		DefaultTimeoutSeconds: 1,
		MaxOutputBytes:        1000,
		MaxLines:              20,
		LogDir:                logRoot,
		LogRetentionDays:      0,
		LogMaxRecords:         2,
	}

	runner := NewRunner(policy)
	for i := 0; i < 5; i++ {
		_, err := runner.RunFilteredInRepo(t.Context(), "printf 'record"+string(rune('0'+i))+"\n'", repoPath, "", 1, Filter{})
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := runner.CleanupHistory(); err != nil {
		t.Fatal(err)
	}

	projectName, err := project.Name(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(logRoot, "repos", projectName, "logs", "index.jsonl")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, b := range data {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("expected 2 index entries after cleanup, got %d", lines)
	}
}

func TestRunnerRunInRepoRejectsEscapingCWD(t *testing.T) {
	dir := t.TempDir()
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{dir}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20})
	_, err := runner.RunInRepo(t.Context(), "echo ok", dir, "../outside", 1)
	if err == nil {
		t.Fatal("expected repo escape error")
	}
}

func TestRunnerRunInRepoAllowsRestrictedSubdir(t *testing.T) {
	dir := t.TempDir()
	safe := filepath.Join(dir, "safe")
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(safe, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{safe}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20})
	if _, err := runner.RunInRepo(t.Context(), "pwd", dir, "safe", 1); err != nil {
		t.Fatalf("safe subdir should be allowed: %v", err)
	}
	if _, err := runner.RunInRepo(t.Context(), "pwd", dir, "blocked", 1); err == nil {
		t.Fatal("blocked subdir should be rejected")
	}
}

func TestApplyFilterPresetProfiles(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		filter Filter
		want   []string
	}{
		{
			name:   "errors-only",
			lines:  []string{"info", "panic: boom", "warning: heads up", "summary: 1 failed"},
			filter: Filter{Preset: "errors-only"},
			want:   []string{"panic: boom", "summary: 1 failed"},
		},
		{
			name:   "test-failures",
			lines:  []string{"=== RUN   TestOne", "--- FAIL: TestOne", "panic: boom", "PASS"},
			filter: Filter{Preset: "test-failures"},
			want:   []string{"--- FAIL: TestOne", "panic: boom"},
		},
		{
			name:   "compile-errors",
			lines:  []string{"ok", "main.go:12:3: undefined: missing", "done"},
			filter: Filter{Preset: "compile-errors"},
			want:   []string{"main.go:12:3: undefined: missing"},
		},
		{
			name:   "git-status",
			lines:  []string{"## main...origin/main", " M internal/command/runner.go", "plain text"},
			filter: Filter{Preset: "git-status"},
			want:   []string{"## main...origin/main", " M internal/command/runner.go"},
		},
		{
			name:   "changed-files",
			lines:  []string{" M internal/command/runner.go", "internal/command/runner_test.go", "note"},
			filter: Filter{Preset: "changed-files"},
			want:   []string{" M internal/command/runner.go", "internal/command/runner_test.go"},
		},
		{
			name:   "summary-with-context",
			lines:  []string{"setup", "warning: deprecated", "summary: 1 failed", "done in 0.1s"},
			filter: Filter{Preset: "summary-with-context"},
			want:   []string{"warning: deprecated", "summary: 1 failed", "done in 0.1s"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, truncated, err := applyFilter(tc.lines, tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if truncated {
				t.Fatal("did not expect truncation")
			}
			if !sameStrings(got, tc.want) {
				t.Fatalf("filtered lines = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestRunnerStoresHistoryAndRefiltersWithPacks(t *testing.T) {
	dir := t.TempDir()
	logDir := t.TempDir()
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{dir}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20, LogDir: logDir})
	result, err := runner.RunFiltered(t.Context(), "printf 'ok\nmain.go:12:3: undefined: missing\n M internal/command/runner.go\ninternal/command/runner_test.go\n'", dir, 1, Filter{Preset: "compile-errors"})
	if err != nil {
		t.Fatal(err)
	}
	if !sameStrings(result.FilteredLines, []string{"main.go:12:3: undefined: missing"}) {
		t.Fatalf("filtered lines = %#v", result.FilteredLines)
	}
	refiltered, err := runner.FilterHistory(result.CommandID, Filter{Packs: []string{"git-status", "changed-files"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{" M internal/command/runner.go", "internal/command/runner_test.go"}
	if !sameStrings(refiltered.FilteredLines, want) {
		t.Fatalf("refiltered lines = %#v, want %#v", refiltered.FilteredLines, want)
	}
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
