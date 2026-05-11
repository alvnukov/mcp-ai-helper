package features

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolutionPrecedenceAndReset(t *testing.T) {
	t.Parallel()
	globalPath := filepath.Join(t.TempDir(), "features.yaml")
	repo := t.TempDir()
	mgr := NewManager(globalPath)
	mgr.Now = func() time.Time { return time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC) }

	initial, err := mgr.Get("bounded_external_worker", repo)
	if err != nil {
		t.Fatal(err)
	}
	if initial.EffectiveEnabled || initial.Source != SourceCodeDefault {
		t.Fatalf("initial = %+v", initial)
	}

	enabled := true
	global, err := mgr.Set(ScopeGlobal, "", "bounded_external_worker", &enabled, "enable globally")
	if err != nil {
		t.Fatal(err)
	}
	if !global.EffectiveEnabled || global.Source != SourceGlobal {
		t.Fatalf("global = %+v", global)
	}

	disabled := false
	repoResolved, err := mgr.Set(ScopeRepo, repo, "bounded_external_worker", &disabled, "repo exception")
	if err != nil {
		t.Fatal(err)
	}
	if repoResolved.EffectiveEnabled || repoResolved.Source != SourceRepo {
		t.Fatalf("repo override = %+v", repoResolved)
	}
	if repoResolved.GlobalOverride == nil || !*repoResolved.GlobalOverride {
		t.Fatalf("global override missing from repo resolution: %+v", repoResolved)
	}

	repoReset, err := mgr.Set(ScopeRepo, repo, "bounded_external_worker", nil, "drop repo exception")
	if err != nil {
		t.Fatal(err)
	}
	if !repoReset.EffectiveEnabled || repoReset.Source != SourceGlobal {
		t.Fatalf("repo reset = %+v", repoReset)
	}

	globalReset, err := mgr.Set(ScopeGlobal, "", "bounded_external_worker", nil, "drop global default")
	if err != nil {
		t.Fatal(err)
	}
	if globalReset.EffectiveEnabled || globalReset.Source != SourceCodeDefault {
		t.Fatalf("global reset = %+v", globalReset)
	}
}

func TestRepoFirstWriteCreatesConfigAndGitignoreIdempotently(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	mgr := NewManager(filepath.Join(t.TempDir(), "features.yaml"))
	enabled := true
	if _, err := mgr.Set(ScopeRepo, repo, "bounded_external_worker", &enabled, "local trial"); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(repo, ".mcp-ai-helper.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("repo config was not created: %v", err)
	}
	firstIgnore, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(firstIgnore), repoGitignoreEntry); count != 1 {
		t.Fatalf("gitignore entry count = %d, content: %q", count, firstIgnore)
	}

	if _, err := mgr.Set(ScopeRepo, repo, "bounded_external_worker", &enabled, "same value"); err != nil {
		t.Fatal(err)
	}
	secondIgnore, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(secondIgnore), repoGitignoreEntry); count != 1 {
		t.Fatalf("gitignore entry duplicated: %q", secondIgnore)
	}
}

func TestUnknownFeatureFailsClosed(t *testing.T) {
	t.Parallel()
	mgr := NewManager(filepath.Join(t.TempDir(), "features.yaml"))
	enabled := true
	_, err := mgr.Set(ScopeGlobal, "", "does_not_exist", &enabled, "nope")
	if err == nil || !strings.Contains(err.Error(), "unknown feature id") {
		t.Fatalf("expected unknown feature error, got %v", err)
	}
	if _, statErr := os.Stat(mgr.GlobalStatePath); !os.IsNotExist(statErr) {
		t.Fatalf("unknown feature should not create state, stat err = %v", statErr)
	}
}
