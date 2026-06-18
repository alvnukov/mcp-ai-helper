// Package gitops provides explicit owned-file git operations.
package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/tasks"
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

func gitBranchExists(ctx context.Context, repo string, branch string) bool {
	_, err := runGit(ctx, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
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

func trackedOwnedFiles(ctx context.Context, repo string, owned []string) ([]string, error) {
	args := append([]string{"ls-files", "--"}, owned...)
	out, err := runGit(ctx, repo, args...)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
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

func ignoredOwnedFiles(ctx context.Context, repo string, files []string) (map[string]bool, error) {
	if len(files) == 0 {
		return nil, nil
	}
	args := append([]string{"check-ignore", "--"}, files...)
	out, err := runGit(ctx, repo, args...)
	if err != nil {
		return nil, nil
	}
	ignored := map[string]bool{}
	for _, line := range splitLines(out) {
		ignored[line] = true
	}
	return ignored, nil
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

// --- git status ---

// StatusRequest describes a git status query.
type StatusRequest struct {
	RepoPath string `json:"repo_path"`
}

// FileStatus describes one file's git status.
type FileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	XY     string `json:"xy"`
}

// StatusResult holds structured git status output.
type StatusResult struct {
	Branch    string       `json:"branch"`
	Upstream  string       `json:"upstream,omitempty"`
	Ahead     int          `json:"ahead,omitempty"`
	Behind    int          `json:"behind,omitempty"`
	Staged    []FileStatus `json:"staged,omitempty"`
	Modified  []FileStatus `json:"modified,omitempty"`
	Untracked []FileStatus `json:"untracked,omitempty"`
	Deleted   []FileStatus `json:"deleted,omitempty"`
	IsClean   bool         `json:"is_clean"`
	RepoPath  string       `json:"repo_path"`
}

// Status returns structured git status for a repository.
func Status(ctx context.Context, req StatusRequest) (StatusResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return StatusResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return StatusResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return StatusResult{}, err
	}

	result := StatusResult{RepoPath: repo, IsClean: true}

	branch, _ := runGit(ctx, repo, "rev-parse", "--abbrev-ref", "HEAD")
	result.Branch = strings.TrimSpace(branch)

	upstream, _ := runGit(ctx, repo, "rev-parse", "--abbrev-ref", "@{upstream}")
	result.Upstream = strings.TrimSpace(upstream)
	if result.Upstream != "" {
		ab, _ := runGit(ctx, repo, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
		parts := strings.Fields(strings.TrimSpace(ab))
		if len(parts) == 2 {
			result.Ahead, _ = strconv.Atoi(parts[0])
			result.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	porcelain, err := runGit(ctx, repo, "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
	if err != nil {
		return StatusResult{}, err
	}

	for _, line := range splitLines(porcelain) {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case '#':
			continue
		case '?':
			path := strings.TrimSpace(line[2:])
			result.Untracked = append(result.Untracked, FileStatus{Path: path, Status: "untracked", XY: "??"})
			result.IsClean = false
		case '1', '2':
			parts := strings.Fields(line)
			if len(parts) < 5 {
				continue
			}
			xy := parts[1]
			path := parts[4]
			x := xy[0]
			y := xy[1]
			fs := FileStatus{Path: path, XY: xy}
			if x != '.' && x != '?' {
				fs.Status = "staged"
				result.Staged = append(result.Staged, fs)
				result.IsClean = false
			}
			if y == 'M' {
				result.Modified = append(result.Modified, FileStatus{Path: path, XY: xy, Status: "modified"})
				result.IsClean = false
			} else if y == 'D' {
				result.Deleted = append(result.Deleted, FileStatus{Path: path, XY: xy, Status: "deleted"})
				result.IsClean = false
			}
		case 'u':
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				result.Modified = append(result.Modified, FileStatus{Path: parts[4], XY: "UU", Status: "conflict"})
				result.IsClean = false
			}
		}
	}

	return result, nil
}

// --- git diff ---

// DiffRequest describes a git diff query.
type DiffRequest struct {
	RepoPath string `json:"repo_path"`
	Cached   bool   `json:"cached,omitempty"`
	Path     string `json:"path,omitempty"`
}

// DiffHunk is one hunk in a diff.
type DiffHunk struct {
	Header string   `json:"header"`
	Lines  []string `json:"lines"`
}

// DiffFile is one file's diff.
type DiffFile struct {
	Path     string     `json:"path"`
	OldPath  string     `json:"old_path,omitempty"`
	Status   string     `json:"status"`
	Hunks    []DiffHunk `json:"hunks,omitempty"`
	IsBinary bool       `json:"is_binary,omitempty"`
}

// DiffResult holds structured diff output.
type DiffResult struct {
	Files []DiffFile `json:"files"`
	Empty bool       `json:"empty"`
}

// Diff returns structured git diff for a repository.
func Diff(ctx context.Context, req DiffRequest) (DiffResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return DiffResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return DiffResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return DiffResult{}, err
	}

	args := []string{"diff", "--no-color", "-U3"}
	if req.Cached {
		args = append(args, "--cached")
	}
	if req.Path != "" {
		args = append(args, "--", req.Path)
	}

	raw, err := runGit(ctx, repo, args...)
	if err != nil {
		return DiffResult{}, err
	}
	if strings.TrimSpace(raw) == "" {
		return DiffResult{Empty: true}, nil
	}

	result := DiffResult{}
	var currentFile *DiffFile
	var currentHunk *DiffHunk

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				result.Files = append(result.Files, *currentFile)
			}
			currentFile = &DiffFile{Status: "modified"}
			currentHunk = nil
		case strings.HasPrefix(line, "rename from"):
			if currentFile != nil {
				currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
				currentFile.Status = "renamed"
			}
		case strings.HasPrefix(line, "rename to"):
			if currentFile != nil {
				currentFile.Path = strings.TrimPrefix(line, "rename to ")
			}
		case strings.HasPrefix(line, "new file"):
			if currentFile != nil {
				currentFile.Status = "added"
			}
		case strings.HasPrefix(line, "deleted file"):
			if currentFile != nil {
				currentFile.Status = "deleted"
			}
		case strings.HasPrefix(line, "--- a/"):
			if currentFile != nil && currentFile.Path == "" {
				currentFile.Path = strings.TrimPrefix(line, "--- a/")
			}
		case strings.HasPrefix(line, "+++ b/"):
			if currentFile != nil {
				currentFile.Path = strings.TrimPrefix(line, "+++ b/")
			}
		case strings.HasPrefix(line, "@@"):
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				currentHunk = &DiffHunk{Header: line}
			}
		case strings.HasPrefix(line, "Binary"):
			if currentFile != nil {
				currentFile.IsBinary = true
			}
		default:
			if currentHunk != nil {
				currentHunk.Lines = append(currentHunk.Lines, line)
			}
		}
	}

	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		result.Files = append(result.Files, *currentFile)
	}

	return result, nil
}
