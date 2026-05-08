package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommitOwnedRejectsPreStagedOutsideOwnedSet(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "owned.txt"), "owned\n")
	writeFile(t, filepath.Join(repo, "external.txt"), "external\n")
	run(t, repo, "add", "external.txt")

	result, err := CommitOwned(t.Context(), CommitRequest{
		RepoPath: repo,
		Files:    []string{"owned.txt"},
		Message:  "owned change",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" {
		t.Fatalf("status = %q, want conflict", result.Status)
	}
	staged := run(t, repo, "diff", "--cached", "--name-only")
	if staged != "external.txt\n" {
		t.Fatalf("unexpected staged files after conflict: %q", staged)
	}
}

func TestCommitOwnedCommitsOnlyOwnedFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "owned.txt"), "owned\n")
	writeFile(t, filepath.Join(repo, "external.txt"), "external\n")

	result, err := CommitOwned(t.Context(), CommitRequest{
		RepoPath: repo,
		Files:    []string{"owned.txt"},
		Message:  "owned change",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if got := run(t, repo, "status", "--short"); got != "?? external.txt\n" {
		t.Fatalf("unexpected status: %q", got)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	run(t, repo, "init")
	run(t, repo, "config", "user.email", "test@example.invalid")
	run(t, repo, "config", "user.name", "Test User")
	return repo
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	// #nosec G204 -- tests execute fixed git commands only.
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, string(out))
	}
	return string(out)
}
