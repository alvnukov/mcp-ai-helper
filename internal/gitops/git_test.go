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

func TestCommitOwnedCommitsIgnoredOwnedFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, ".gitignore"), "*.log\n")
	run(t, repo, "add", ".gitignore")
	run(t, repo, "commit", "-m", "ignore logs")
	writeFile(t, filepath.Join(repo, "owned.log"), "owned\n")
	writeFile(t, filepath.Join(repo, "external.log"), "external\n")

	result, err := CommitOwned(t.Context(), CommitRequest{
		RepoPath: repo,
		Files:    []string{"owned.log"},
		Message:  "owned ignored change",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if got := run(t, repo, "show", "--name-status", "--format=", "HEAD"); got != "A\towned.log\n" {
		t.Fatalf("unexpected commit diff: %q", got)
	}
	if got := run(t, repo, "status", "--short", "--ignored"); got != "!! external.log\n" {
		t.Fatalf("unexpected status: %q", got)
	}
}

func TestCommitOwnedCommitsDeletedOwnedFile(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "tracked.txt"), "tracked\n")
	run(t, repo, "add", "tracked.txt")
	run(t, repo, "commit", "-m", "initial")
	if err := os.Remove(filepath.Join(repo, "tracked.txt")); err != nil {
		t.Fatal(err)
	}

	result, err := CommitOwned(t.Context(), CommitRequest{
		RepoPath: repo,
		Files:    []string{"tracked.txt"},
		Message:  "delete tracked",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if got := run(t, repo, "show", "--name-status", "--format=", "HEAD"); got != "D\ttracked.txt\n" {
		t.Fatalf("unexpected commit diff: %q", got)
	}
	if got := run(t, repo, "status", "--short"); got != "" {
		t.Fatalf("unexpected status: %q", got)
	}
}

func TestPrepareTaskWorktreeCreatesTypedBranchInTaskPath(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "initial\n")
	run(t, repo, "add", "README.md")
	run(t, repo, "commit", "-m", "initial")

	result, err := PrepareTaskWorktree(t.Context(), PrepareTaskWorktreeRequest{RepoPath: repo, TaskID: "task-123", TaskType: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Created {
		t.Fatalf("result = %#v, want created ok", result)
	}
	if result.Branch != "feature/task-123" {
		t.Fatalf("branch = %q", result.Branch)
	}
	if result.WorktreePath != ".worktrees/task-123" {
		t.Fatalf("worktree_path = %q", result.WorktreePath)
	}
	canonicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result.CodePath != filepath.Join(canonicalRepo, ".worktrees", "task-123") {
		t.Fatalf("code_path = %q", result.CodePath)
	}
	if got := run(t, result.CodePath, "rev-parse", "--abbrev-ref", "HEAD"); got != "feature/task-123\n" {
		t.Fatalf("worktree branch = %q", got)
	}
}

func TestPrepareTaskWorktreeRequiresTaskType(t *testing.T) {
	repo := initRepo(t)
	_, err := PrepareTaskWorktree(t.Context(), PrepareTaskWorktreeRequest{RepoPath: repo, TaskID: "task-123"})
	if err == nil {
		t.Fatal("expected task_type error")
	}
}

func TestPrepareTaskWorktreeIsIdempotentForSameBranch(t *testing.T) {
	repo := initRepo(t)
	writeFile(t, filepath.Join(repo, "README.md"), "initial\n")
	run(t, repo, "add", "README.md")
	run(t, repo, "commit", "-m", "initial")

	first, err := PrepareTaskWorktree(t.Context(), PrepareTaskWorktreeRequest{RepoPath: repo, TaskID: "task-123", TaskType: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareTaskWorktree(t.Context(), PrepareTaskWorktreeRequest{RepoPath: repo, TaskID: "task-123", TaskType: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || second.Status != "ok" {
		t.Fatalf("first=%#v second=%#v", first, second)
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
