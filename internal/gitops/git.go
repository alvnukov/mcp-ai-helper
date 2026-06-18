package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type CommitRequest struct {
	RepoPath string   `json:"repo_path"`
	Repo     string   `json:"repo"`
	Files    []string `json:"files"`
	Message  string   `json:"message"`
}

type CommitResult struct {
	Status      string   `json:"status"`
	Commit      string   `json:"commit,omitempty"`
	StagedFiles []string `json:"staged_files"`
	Reason      string   `json:"reason,omitempty"`
}

type PrepareTaskWorktreeRequest struct {
	RepoPath string `json:"repo_path"`
	TaskID   string `json:"task_id"`
	TaskType string `json:"task_type"`
}

type PrepareTaskWorktreeResult struct {
	Status       string `json:"status"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
	CodePath     string `json:"code_path"`
	Created      bool   `json:"created"`
	Reason       string `json:"reason,omitempty"`
}

func PrepareTaskWorktree(ctx context.Context, req PrepareTaskWorktreeRequest) (PrepareTaskWorktreeResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return PrepareTaskWorktreeResult{}, errors.New("repo_path is required")
	}
	branch, err := tasks.BranchForTask(req.TaskType, req.TaskID)
	if err != nil {
		return PrepareTaskWorktreeResult{}, err
	}
	worktreePath := tasks.WorktreePathForID(req.TaskID)
	if worktreePath == "" {
		return PrepareTaskWorktreeResult{}, errors.New("task_id is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return PrepareTaskWorktreeResult{}, err
	}
	top, err := runGit(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return PrepareTaskWorktreeResult{}, fmt.Errorf("not a git repo: %w", err)
	}
	repo = strings.TrimSpace(top)
	codePath := filepath.Join(repo, filepath.FromSlash(worktreePath))
	worktreesDir := filepath.Join(repo, ".worktrees")
	if rel, err := filepath.Rel(worktreesDir, codePath); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return PrepareTaskWorktreeResult{}, fmt.Errorf("worktree path escapes .worktrees: %s", worktreePath)
	}
	if info, statErr := os.Stat(codePath); statErr == nil {
		if !info.IsDir() {
			return PrepareTaskWorktreeResult{Status: "conflict", Branch: branch, WorktreePath: worktreePath, CodePath: codePath, Reason: "worktree path exists and is not a directory"}, nil
		}
		current, err := runGit(ctx, codePath, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return PrepareTaskWorktreeResult{Status: "conflict", Branch: branch, WorktreePath: worktreePath, CodePath: codePath, Reason: "worktree path is not a git checkout"}, nil
		}
		if strings.TrimSpace(current) != branch {
			return PrepareTaskWorktreeResult{Status: "conflict", Branch: branch, WorktreePath: worktreePath, CodePath: codePath, Reason: "worktree path is on branch " + strings.TrimSpace(current)}, nil
		}
		return PrepareTaskWorktreeResult{Status: "ok", Branch: branch, WorktreePath: worktreePath, CodePath: codePath, Created: false, Reason: "worktree already exists"}, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return PrepareTaskWorktreeResult{}, statErr
	}
	if err := os.MkdirAll(worktreesDir, 0o700); err != nil {
		return PrepareTaskWorktreeResult{}, err
	}
	args := []string{"worktree", "add"}
	if gitBranchExists(ctx, repo, branch) {
		args = append(args, codePath, branch)
	} else {
		args = append(args, "-b", branch, codePath, "HEAD")
	}
	if _, err := runGit(ctx, repo, args...); err != nil {
		return PrepareTaskWorktreeResult{}, err
	}
	return PrepareTaskWorktreeResult{Status: "ok", Branch: branch, WorktreePath: worktreePath, CodePath: codePath, Created: true}, nil
}

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

	trackedFiles, err := trackedOwnedFiles(ctx, repo, owned)
	if err != nil {
		return CommitResult{}, err
	}
	if len(trackedFiles) > 0 {
		updateArgs := append([]string{"add", "-u", "--"}, trackedFiles...)
		if _, err := runGit(ctx, repo, updateArgs...); err != nil {
			return CommitResult{}, err
		}
	}
	existingFiles := make([]string, 0, len(owned))
	for _, file := range owned {
		_, statErr := os.Stat(filepath.Join(repo, file))
		if statErr == nil {
			existingFiles = append(existingFiles, file)
			continue
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return CommitResult{}, statErr
		}
	}
	if len(existingFiles) > 0 {
		ignored, _ := ignoredOwnedFiles(ctx, repo, existingFiles)
		normal := make([]string, 0, len(existingFiles))
		force := make([]string, 0, len(ignored))
		for _, f := range existingFiles {
			if ignored[f] {
				force = append(force, f)
			} else {
				normal = append(normal, f)
			}
		}
		if len(normal) > 0 {
			if _, err := runGit(ctx, repo, append([]string{"add", "--"}, normal...)...); err != nil {
				return CommitResult{}, err
			}
		}
		if len(force) > 0 {
			if _, err := runGit(ctx, repo, append([]string{"add", "-f", "--"}, force...)...); err != nil {
				return CommitResult{}, err
			}
		}
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
