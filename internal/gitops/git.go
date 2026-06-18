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

// --- git log ---

// LogRequest describes a git log query.
type LogRequest struct {
	RepoPath string `json:"repo_path"`
	Limit    int    `json:"limit,omitempty"`
	Path     string `json:"path,omitempty"`
	Author   string `json:"author,omitempty"`
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
	Grep     string `json:"grep,omitempty"`
}

// LogCommit describes one commit in the log.
type LogCommit struct {
	Hash      string   `json:"hash"`
	ShortHash string   `json:"short_hash"`
	Author    string   `json:"author"`
	Date      string   `json:"date"`
	Message   string   `json:"message"`
	Files     []string `json:"files,omitempty"`
}

// LogResult holds structured git log output.
type LogResult struct {
	Commits []LogCommit `json:"commits"`
	Total   int         `json:"total"`
}

// Log returns structured git log for a repository.
func Log(ctx context.Context, req LogRequest) (LogResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return LogResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return LogResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return LogResult{}, err
	}

	args := []string{"log", "--format=%H|%h|%an|%ai|%s", "--no-color"}
	if req.Limit > 0 {
		args = append(args, fmt.Sprintf("-%d", req.Limit))
	} else {
		args = append(args, "-20")
	}
	if req.Author != "" {
		args = append(args, "--author="+req.Author)
	}
	if req.Since != "" {
		args = append(args, "--since="+req.Since)
	}
	if req.Until != "" {
		args = append(args, "--until="+req.Until)
	}
	if req.Grep != "" {
		args = append(args, "--grep="+req.Grep)
	}
	if req.Path != "" {
		args = append(args, "--", req.Path)
	}

	raw, err := runGit(ctx, repo, args...)
	if err != nil {
		return LogResult{}, err
	}

	result := LogResult{}
	for _, line := range splitLines(raw) {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		commit := LogCommit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Author:    parts[2],
			Date:      parts[3],
			Message:   parts[4],
		}
		result.Commits = append(result.Commits, commit)
	}
	result.Total = len(result.Commits)
	return result, nil
}

// LogDiffRequest describes a git show query for a single commit.
type LogDiffRequest struct {
	RepoPath string `json:"repo_path"`
	Hash     string `json:"hash"`
}

// LogDiffResult holds structured git show output for a commit.
type LogDiffResult struct {
	Hash      string     `json:"hash"`
	ShortHash string     `json:"short_hash"`
	Author    string     `json:"author"`
	Date      string     `json:"date"`
	Message   string     `json:"message"`
	Files     []DiffFile `json:"files"`
	Stats     []FileStat `json:"stats"`
}

// FileStat describes file change statistics.
type FileStat struct {
	Path     string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// LogDiff returns structured git show output for a single commit.
func LogDiff(ctx context.Context, req LogDiffRequest) (LogDiffResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return LogDiffResult{}, errors.New("repo_path is required")
	}
	if strings.TrimSpace(req.Hash) == "" {
		return LogDiffResult{}, errors.New("hash is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return LogDiffResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return LogDiffResult{}, err
	}

	raw, err := runGit(ctx, repo, "show", "--format=%H|%h|%an|%ai|%s", "--no-color", "-U3", req.Hash)
	if err != nil {
		return LogDiffResult{}, err
	}

	result := LogDiffResult{}
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 {
		header := lines[0]
		parts := strings.SplitN(header, "|", 5)
		if len(parts) >= 5 {
			result.Hash = parts[0]
			result.ShortHash = parts[1]
			result.Author = parts[2]
			result.Date = parts[3]
			result.Message = parts[4]
		}
	}

	var currentFile *DiffFile
	var currentHunk *DiffHunk

	for _, line := range lines[1:] {
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
		case strings.HasPrefix(line, "Binary files"):
			if currentFile != nil {
				currentFile.IsBinary = true
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

	statRaw, err := runGit(ctx, repo, "show", "--stat", "--format=", "--no-color", req.Hash)
	if err == nil {
		for _, line := range splitLines(statRaw) {
			if strings.Contains(line, "|") {
				parts := strings.SplitN(line, "|", 2)
				if len(parts) == 2 {
					path := strings.TrimSpace(parts[0])
					changeStr := strings.TrimSpace(parts[1])
					additions := strings.Count(changeStr, "+")
					deletions := strings.Count(changeStr, "-")
					result.Stats = append(result.Stats, FileStat{Path: path, Additions: additions, Deletions: deletions})
				}
			}
		}
	}

	return result, nil
}

// --- git stash ---

// StashRequest describes a git stash list query.
type StashRequest struct {
	RepoPath string `json:"repo_path"`
}

// StashEntry describes one stash entry.
type StashEntry struct {
	Index   int    `json:"index"`
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

// StashResult holds structured git stash list output.
type StashResult struct {
	Entries []StashEntry `json:"entries"`
	Total   int          `json:"total"`
}

// StashList returns structured git stash list for a repository.
func StashList(ctx context.Context, req StashRequest) (StashResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return StashResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return StashResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return StashResult{}, err
	}

	raw, err := runGit(ctx, repo, "stash", "list", "--format=%H|%gd|%s")
	if err != nil {
		return StashResult{}, err
	}

	result := StashResult{}
	for _, line := range splitLines(raw) {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		entry := StashEntry{
			Hash:    parts[0],
			Message: parts[2],
		}
		fmt.Sscanf(parts[1], "stash@{%d}", &entry.Index)
		result.Entries = append(result.Entries, entry)
	}
	result.Total = len(result.Entries)
	return result, nil
}

// --- git branch ---

// BranchRequest describes a git branch query.
type BranchRequest struct {
	RepoPath string `json:"repo_path"`
	All      bool   `json:"all,omitempty"`
}

// Branch describes one branch.
type Branch struct {
	Name      string `json:"name"`
	IsCurrent bool   `json:"is_current"`
	IsRemote  bool   `json:"is_remote,omitempty"`
	Hash      string `json:"hash"`
}

// BranchResult holds structured git branch output.
type BranchResult struct {
	Branches []Branch `json:"branches"`
	Current  string   `json:"current"`
}

// BranchList returns structured git branch list for a repository.
func BranchList(ctx context.Context, req BranchRequest) (BranchResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return BranchResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return BranchResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return BranchResult{}, err
	}

	args := []string{"branch", "--format=%(refname:short)|%(objectname:short)|%(HEAD)"}
	if req.All {
		args = append(args, "-a")
	}

	raw, err := runGit(ctx, repo, args...)
	if err != nil {
		return BranchResult{}, err
	}

	result := BranchResult{}
	for _, line := range splitLines(raw) {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		branch := Branch{
			Name:      parts[0],
			Hash:      parts[1],
			IsCurrent: parts[2] == "*",
			IsRemote:  strings.Contains(parts[0], "/"),
		}
		if branch.IsCurrent {
			result.Current = branch.Name
		}
		result.Branches = append(result.Branches, branch)
	}
	return result, nil
}

// --- git remote ---

// RemoteRequest describes a git remote query.
type RemoteRequest struct {
	RepoPath string `json:"repo_path"`
}

// Remote describes one remote.
type Remote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Fetch string `json:"fetch,omitempty"`
}

// RemoteResult holds structured git remote output.
type RemoteResult struct {
	Remotes []Remote `json:"remotes"`
}

// RemoteList returns structured git remote list for a repository.
func RemoteList(ctx context.Context, req RemoteRequest) (RemoteResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return RemoteResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return RemoteResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return RemoteResult{}, err
	}

	raw, err := runGit(ctx, repo, "remote", "-v")
	if err != nil {
		return RemoteResult{}, err
	}

	remotes := map[string]*Remote{}
	for _, line := range splitLines(raw) {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		url := parts[1]
		direction := parts[2]
		if _, ok := remotes[name]; !ok {
			remotes[name] = &Remote{Name: name}
		}
		if direction == "(fetch)" {
			remotes[name].URL = url
			remotes[name].Fetch = url
		} else if direction == "(push)" {
			remotes[name].URL = url
		}
	}

	result := RemoteResult{}
	for _, r := range remotes {
		result.Remotes = append(result.Remotes, *r)
	}
	return result, nil
}

// --- git tag ---

// TagRequest describes a git tag query.
type TagRequest struct {
	RepoPath string `json:"repo_path"`
	Pattern  string `json:"pattern,omitempty"`
}

// Tag describes one tag.
type Tag struct {
	Name   string `json:"name"`
	Hash   string `json:"hash"`
	IsAnnotated bool   `json:"is_annotated,omitempty"`
	Message string `json:"message,omitempty"`
}

// TagResult holds structured git tag output.
type TagResult struct {
	Tags  []Tag `json:"tags"`
	Total int   `json:"total"`
}

// TagList returns structured git tag list for a repository.
func TagList(ctx context.Context, req TagRequest) (TagResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return TagResult{}, errors.New("repo_path is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return TagResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return TagResult{}, err
	}

	args := []string{"tag", "--format=%(refname:short)|%(objectname:short)|%(objecttype)|%(contents)"}
	if req.Pattern != "" {
		args = append(args, "-l", req.Pattern)
	}

	raw, err := runGit(ctx, repo, args...)
	if err != nil {
		return TagResult{}, err
	}

	result := TagResult{}
	for _, line := range splitLines(raw) {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 3 {
			continue
		}
		tag := Tag{
			Name: parts[0],
			Hash: parts[1],
		}
		if parts[2] == "tag" {
			tag.IsAnnotated = true
		}
		if len(parts) > 3 {
			tag.Message = parts[3]
		}
		result.Tags = append(result.Tags, tag)
	}
	result.Total = len(result.Tags)
	return result, nil
}

// --- git blame ---

// BlameRequest describes a git blame query.
type BlameRequest struct {
	RepoPath string `json:"repo_path"`
	File     string `json:"file"`
}

// BlameLine describes one line's blame info.
type BlameLine struct {
	Hash      string `json:"hash"`
	Author    string `json:"author"`
	Date      string `json:"date"`
	Line      int    `json:"line"`
	Content   string `json:"content"`
}

// BlameResult holds structured git blame output.
type BlameResult struct {
	Lines []BlameLine `json:"lines"`
	Total int         `json:"total"`
}

// Blame returns structured git blame for a file.
func Blame(ctx context.Context, req BlameRequest) (BlameResult, error) {
	if strings.TrimSpace(req.RepoPath) == "" {
		return BlameResult{}, errors.New("repo_path is required")
	}
	if strings.TrimSpace(req.File) == "" {
		return BlameResult{}, errors.New("file is required")
	}
	repo, err := filepath.Abs(req.RepoPath)
	if err != nil {
		return BlameResult{}, err
	}
	if _, err := runGit(ctx, repo, "rev-parse", "--show-toplevel"); err != nil {
		return BlameResult{}, err
	}

	raw, err := runGit(ctx, repo, "blame", "--porcelain", "--", req.File)
	if err != nil {
		return BlameResult{}, err
	}

	result := BlameResult{}
	lines := strings.Split(raw, "\n")
	var current *BlameLine

	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if current != nil {
				current.Content = strings.TrimPrefix(line, "\t")
				result.Lines = append(result.Lines, *current)
			}
			current = nil
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		value := parts[1]
		switch key {
		case "author":
			if current == nil {
				current = &BlameLine{}
			}
			current.Author = value
		case "author-time":
			if current == nil {
				current = &BlameLine{}
			}
			current.Date = value
		case "summary":
			// skip
		default:
			if len(key) == 40 {
				if current == nil {
					current = &BlameLine{}
				}
				current.Hash = key[:8]
				if n, err := strconv.Atoi(value); err == nil {
					current.Line = n
				}
			}
		}
	}
	result.Total = len(result.Lines)
	return result, nil
}

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
			// Porcelain v2: 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			// or for renames: 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <path><tab><origPath>
			if len(parts) < 9 {
				continue
			}
			xy := parts[1]
			path := parts[8]
			// For renames, path may contain origPath after tab.
			if tabIdx := strings.IndexByte(line, '\t'); tabIdx >= 0 {
				path = line[tabIdx+1:]
			}
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
			// Unmerged: u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
			parts := strings.Fields(line)
			if len(parts) >= 11 {
				result.Modified = append(result.Modified, FileStatus{Path: parts[10], XY: parts[1], Status: "conflict"})
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
