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

func TestRunnerRunInRepoRejectsEscapingCWD(t *testing.T) {
	dir := t.TempDir()
	runner := NewRunner(config.CommandPolicy{AllowedCWDs: []string{dir}, DefaultTimeoutSeconds: 1, MaxOutputBytes: 1000, MaxLines: 20})
	_, err := runner.RunInRepo(t.Context(), "echo ok", dir, "../outside", 1)
	if err == nil {
		t.Fatal("expected repo escape error")
	}
}
