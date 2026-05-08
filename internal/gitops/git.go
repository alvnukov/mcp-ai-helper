// Package gitops provides explicit owned-file git operations.
package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CommitRequest describes a safe commit over explicitly owned files.
type CommitRequest struct {
	RepoPath string   `json:"repo_path"`
	Repo     string   `json:"repo"`
	Files    []string `json:"files"`
	Message  string   `json:"message"`
}

// CommitResult reports the outcome of an owned-file commit attempt.
type CommitResult struct {
	Status      string   `json:"status"`
	Commit      string   `json:"commit,omitempty"`
	StagedFiles []string `json:"staged_files"`
	Reason      string   `json:"reason,omitempty"`
}

// CommitOwned stages and commits only the repo-relative files listed in req.
func CommitOwned(ctx context.Context, req CommitRequest) (CommitResult, error) {
	repoInput := req.RepoPath
	if repoInput == "" {
		repoInput = req.Repo
	}
	if strings.TrimSpace(repoInput) == "" {
		return CommitResult{}, errors.New("repo_path is required")
	}
	if len(req.Files) == 0 {
		return CommitResult{Status: "skipped", Reason: "no files to commit"}, nil
	}
	if strings.TrimSpace(req.Message) == "" {
		return CommitResult{}, errors.New("message is required")
	}
	repo, err := filepath.Abs(repoInput)
	if err != nil {
		return CommitResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return CommitResult{}, fmt.Errorf("not a git repo: %w", err)
	}

	owned, err := normalizeOwnedFiles(req.Files)
	if err != nil {
		return CommitResult{}, err
	}
	if len(owned) == 0 {
		return CommitResult{Status: "skipped", Reason: "no non-empty files to commit"}, nil
	}
	allowed := map[string]struct{}{}
	for _, file := range owned {
		allowed[file] = struct{}{}
	}

	preStaged, err := stagedFiles(ctx, repo)
	if err != nil {
		return CommitResult{}, err
	}
	for _, file := range preStaged {
		if _, ok := allowed[file]; !ok {
			return CommitResult{Status: "conflict", StagedFiles: preStaged, Reason: "index already contains file outside owned set: " + file}, nil
		}
	}

	args := append([]string{"add", "--"}, owned...)
	if _, err := runGit(ctx, repo, args...); err != nil {
		return CommitResult{}, err
	}
	staged, err := stagedFiles(ctx, repo)
	if err != nil {
		return CommitResult{}, err
	}
	if len(staged) == 0 {
		return CommitResult{Status: "skipped", Reason: "no staged diff"}, nil
	}
	for _, file := range staged {
		if _, ok := allowed[file]; !ok {
			return CommitResult{Status: "conflict", StagedFiles: staged, Reason: "staged diff contains file outside owned set: " + file}, nil
		}
	}
	if _, err := runGit(ctx, repo, "commit", "-m", req.Message); err != nil {
		return CommitResult{}, err
	}
	commit, err := runGit(ctx, repo, "rev-parse", "--short", "HEAD")
	if err != nil {
		return CommitResult{}, err
	}
	return CommitResult{Status: "ok", Commit: strings.TrimSpace(commit), StagedFiles: staged}, nil
}

func normalizeOwnedFiles(files []string) ([]string, error) {
	owned := make([]string, 0, len(files))
	seen := map[string]struct{}{}
	for _, file := range files {
		clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(file)))
		if clean == "" || clean == "." {
			continue
		}
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "../") || clean == ".." {
			return nil, fmt.Errorf("owned file must be repo-relative: %q", file)
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		owned = append(owned, clean)
	}
	return owned, nil
}

func stagedFiles(ctx context.Context, repo string) ([]string, error) {
	diff, err := runGit(ctx, repo, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	return splitLines(diff), nil
}

func runGit(ctx context.Context, repo string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// #nosec G204 -- git subcommands are constructed internally and file operands are normalized repo-relative paths.
	cmd := exec.CommandContext(runCtx, "git", args...)
	cmd.Dir = repo
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func splitLines(text string) []string {
	raw := strings.Split(strings.TrimSpace(text), "\n")
	if len(raw) == 1 && raw[0] == "" {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}
