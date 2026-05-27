// Package fileops provides guarded, idempotent file edit operations.
package fileops

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Snapshot records file identity before a guarded edit.
type Snapshot struct {
	Path         string `json:"path"`
	RepoPath     string `json:"repo_path,omitempty"`
	RelativePath string `json:"relative_path,omitempty"`
	Hash         string `json:"hash"`
	Size         int    `json:"size"`
	Exists       bool   `json:"exists"`
}

// ReplaceRequest describes one unique text replacement guarded by file hash.
// When Old/New contain characters that are difficult to represent in JSON strings
// (e.g. raw Go literals with backslashes), use OldB64 and NewB64 instead —
// they carry the same content base64-encoded and avoid escaping problems.
type ReplaceRequest struct {
	RepoPath     string `json:"repo_path"`
	Path         string `json:"path"`
	ExpectedHash string `json:"expected_hash"`
	Old          string `json:"old"`
	New          string `json:"new"`
	OldB64       string `json:"old_b64,omitempty"`
	NewB64       string `json:"new_b64,omitempty"`
}

// ReplaceResult reports the result of a guarded replacement.
type ReplaceResult struct {
	Status  string `json:"status"`
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
	OldHash string `json:"old_hash"`
	NewHash string `json:"new_hash"`
	Reason  string `json:"reason,omitempty"`
}

// ReadSnapshot returns file hash and size for an absolute or process-relative path.
func ReadSnapshot(path string) (Snapshot, error) {
	clean, err := cleanPath(path)
	if err != nil {
		return Snapshot{}, err
	}
	// #nosec G304 -- cleanPath resolves the caller-specified local path before reading.
	data, err := os.ReadFile(clean)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Snapshot{Path: clean, Exists: false}, nil
		}
		return Snapshot{}, err
	}
	return Snapshot{Path: clean, Hash: Hash(data), Size: len(data), Exists: true}, nil
}

// ReadSnapshotInRepo returns a snapshot for a repo-relative path.
func ReadSnapshotInRepo(repoPath string, path string) (Snapshot, error) {
	resolved, rel, err := repoRelativePath(repoPath, path)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot, err := ReadSnapshot(resolved)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot.RepoPath = repoPath
	snapshot.RelativePath = rel
	return snapshot, nil
}

func resolveText(req ReplaceRequest) (old string, new string, err error) {
	if req.OldB64 != "" {
		decoded, decodeErr := base64.StdEncoding.DecodeString(req.OldB64)
		if decodeErr != nil {
			return "", "", fmt.Errorf("old_b64: invalid base64: %w", decodeErr)
		}
		old = string(decoded)
	} else {
		old = req.Old
	}
	if req.NewB64 != "" {
		decoded, decodeErr := base64.StdEncoding.DecodeString(req.NewB64)
		if decodeErr != nil {
			return "", "", fmt.Errorf("new_b64: invalid base64: %w", decodeErr)
		}
		new = string(decoded)
	} else {
		new = req.New
	}
	return old, new, nil
}

func findBestPartialMatch(text string, old string) string {
	best := 0
	bestPos := 0
	for i := 0; i < len(text); i++ {
		j := 0
		for i+j < len(text) && j < len(old) && text[i+j] == old[j] {
			j++
		}
		if j > best {
			best = j
			bestPos = i
		}
	}
	if best == 0 {
		return ""
	}
	start := bestPos
	if start > 20 {
		start = bestPos - 20
	}
	end := bestPos + best + 40
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

// ApplyGuardedReplace replaces one unique text span only if the file hash still matches.
// Prefer OldB64/NewB64 over Old/New when the text contains characters that are
// difficult to represent in JSON strings (e.g. Go raw string literals).
func ApplyGuardedReplace(req ReplaceRequest) (ReplaceResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return ReplaceResult{}, errors.New("path is required")
	}
	var clean string
	if strings.TrimSpace(req.RepoPath) != "" {
		var err error
		clean, _, err = repoRelativePath(req.RepoPath, req.Path)
		if err != nil {
			return ReplaceResult{}, err
		}
	} else {
		var err error
		clean, err = cleanPath(req.Path)
		if err != nil {
			return ReplaceResult{}, err
		}
	}
	if req.ExpectedHash == "" {
		return ReplaceResult{}, errors.New("expected_hash is required")
	}
	old, new, err := resolveText(req)
	if err != nil {
		return ReplaceResult{}, err
	}
	if old == "" {
		return ReplaceResult{}, errors.New("old text is required (set old or old_b64)")
	}
	// #nosec G304 -- clean is resolved from a validated local path or repo-relative path.
	data, err := os.ReadFile(clean)
	if err != nil {
		return ReplaceResult{}, err
	}
	oldHash := Hash(data)
	if oldHash != req.ExpectedHash {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "file hash changed after snapshot"}, nil
	}
	text := string(data)
	if strings.Contains(text, new) && !strings.Contains(text, old) {
		return ReplaceResult{Status: "ok", Path: clean, Changed: false, OldHash: oldHash, NewHash: oldHash, Reason: "desired text already present"}, nil
	}
	count := strings.Count(text, old)
	if count == 0 {
		detail := findBestPartialMatch(text, old)
		msg := "old text not found"
		if detail != "" {
			msg += fmt.Sprintf("; best partial match near: %q", detail)
		}
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: msg}, nil
	}
	if count > 1 {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "old text is not unique"}, nil
	}
	next := strings.Replace(text, old, new, 1)
	newHash := Hash([]byte(next))
	// #nosec G703 -- clean is resolved from a validated local path or repo-relative path.
	if err := os.WriteFile(clean, []byte(next), 0o600); err != nil {
		return ReplaceResult{}, err
	}
	return ReplaceResult{Status: "ok", Path: clean, Changed: newHash != oldHash, OldHash: oldHash, NewHash: newHash}, nil
}

// FileLine is one numbered line of file content.
type FileLine struct {
	Number int    `json:"n"`
	Text   string `json:"text"`
}

// FileContent holds structured file read results.
type FileContent struct {
	Path         string     `json:"path"`
	RepoPath     string     `json:"repo_path,omitempty"`
	RelativePath string     `json:"relative_path,omitempty"`
	Hash         string     `json:"hash"`
	Size         int        `json:"size"`
	Exists       bool       `json:"exists"`
	Lines        []FileLine `json:"lines"`
}

// ReadFileContent reads a file and returns structured content with line numbers.
func ReadFileContent(path string) (FileContent, error) {
	clean, err := cleanPath(path)
	if err != nil {
		return FileContent{}, err
	}
	dir, name := filepath.Split(clean)
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileContent{Path: clean, Exists: false}, nil
		}
		return FileContent{}, err
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileContent{Path: clean, Exists: false}, nil
		}
		return FileContent{}, err
	}
	text := string(data)
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	// Drop trailing empty line from final newline.
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	lines := make([]FileLine, 0, len(raw))
	for i, line := range raw {
		lines = append(lines, FileLine{Number: i + 1, Text: line})
	}
	return FileContent{
		Path:   clean,
		Hash:   Hash(data),
		Size:   len(data),
		Exists: true,
		Lines:  lines,
	}, nil
}

// ReadFileContentInRepo reads a repo-relative file and returns structured content.
func ReadFileContentInRepo(repoPath string, path string) (FileContent, error) {
	resolved, rel, err := repoRelativePath(repoPath, path)
	if err != nil {
		return FileContent{}, err
	}
	fc, err := ReadFileContent(resolved)
	if err != nil {
		return FileContent{}, err
	}
	fc.RepoPath = repoPath
	fc.RelativePath = rel
	return fc, nil
}

// SearchMatch is one search result.
type SearchMatch struct {
	File       string `json:"file"`
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

// SearchResult holds structured search results.
type SearchResult struct {
	Pattern string        `json:"pattern"`
	Path    string        `json:"path"`
	Matches []SearchMatch `json:"matches"`
	Total   int           `json:"total"`
}

// SearchFiles runs a simple text search in a directory and returns structured results.
// It reads each non-binary file under root, splits into lines, and matches pattern.
func SearchFiles(root string, pattern string, maxMatches int) (SearchResult, error) {
	if maxMatches <= 0 {
		maxMatches = 100
	}
	result := SearchResult{Pattern: pattern, Path: root}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return result, err
	}
	defer func() { _ = rootHandle.Close() }()
	seenFiles := 0
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip inaccessible entries
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") && path != root {
				return filepath.SkipDir
			}
			if base == "node_modules" || base == "__pycache__" || base == "vendor" || isTaskRegistryDir(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip binary and large files.
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".exe", ".dll", ".so", ".dylib", ".bin", ".jpg", ".png", ".gif", ".ico",
			".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".pdf", ".class", ".pyc", ".pyo":
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == "." {
			return nil
		}
		if isProtectedLeanPath(rel) {
			return nil
		}
		data, readErr := rootHandle.ReadFile(rel)
		if readErr != nil {
			return nil
		}
		if len(data) > 1<<20 { // skip files > 1MB
			return nil
		}
		text := string(data)
		lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
		fileMatchCount := 0
		for i, line := range lines {
			if strings.Contains(line, pattern) {
				if rel == "" {
					rel = path
				}
				result.Matches = append(result.Matches, SearchMatch{
					File:       filepath.ToSlash(rel),
					LineNumber: i + 1,
					Text:       line,
				})
				fileMatchCount++
				result.Total++
				if result.Total >= maxMatches {
					return filepath.SkipAll
				}
			}
		}
		if fileMatchCount > 0 {
			seenFiles++
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

// ReadFilesFileResult is a per-file result in batch reads.
type ReadFilesFileResult struct {
	Path          string     `json:"path,omitempty"`
	RelativePath  string     `json:"relative_path,omitempty"`
	Hash          string     `json:"hash,omitempty"`
	Size          int        `json:"size"`
	Exists        bool       `json:"exists"`
	Lines         []FileLine `json:"lines,omitempty"`
	Error         string     `json:"error,omitempty"`
	Truncated     bool       `json:"truncated,omitempty"`
	OmittedReason string     `json:"omitted_reason,omitempty"`
}

// ReadFilesResult holds the batch read result.
type ReadFilesResult struct {
	Files         []ReadFilesFileResult `json:"files"`
	TotalFiles    int                   `json:"total_files"`
	ReturnedFiles int                   `json:"returned_files"`
	ReturnedBytes int                   `json:"returned_bytes"`
	Truncated     bool                  `json:"truncated,omitempty"`
}

const (
	maxReadFilesPaths      = 8
	maxReadFileBytes       = 64 * 1024
	maxReadFilesTotalBytes = 128 * 1024
)

// ReadFilesInRepo reads multiple repo-relative files with hard bounds.
func ReadFilesInRepo(repoPath string, paths []string) (ReadFilesResult, error) {
	if len(paths) == 0 {
		return ReadFilesResult{}, errors.New("paths must not be empty")
	}
	if len(paths) > maxReadFilesPaths {
		return ReadFilesResult{}, fmt.Errorf("too many paths: %d, max %d", len(paths), maxReadFilesPaths)
	}

	result := ReadFilesResult{
		Files:      make([]ReadFilesFileResult, 0, len(paths)),
		TotalFiles: len(paths),
	}

	var totalBytes int
	truncated := false

	for _, path := range paths {
		fc, err := ReadFileContentInRepo(repoPath, path)
		if err != nil {
			result.Files = append(result.Files, ReadFilesFileResult{
				RelativePath: filepath.ToSlash(filepath.Clean(path)),
				Error:        err.Error(),
			})
			continue
		}

		fr := ReadFilesFileResult{
			Path:         fc.Path,
			RelativePath: fc.RelativePath,
			Hash:         fc.Hash,
			Size:         fc.Size,
			Exists:       fc.Exists,
		}

		if !fc.Exists {
			result.Files = append(result.Files, fr)
			continue
		}

		if fc.Size > maxReadFileBytes {
			fr.Truncated = true
			fr.OmittedReason = fmt.Sprintf("file size %d exceeds per-file limit %d", fc.Size, maxReadFileBytes)
			result.Files = append(result.Files, fr)
			truncated = true
			continue
		}

		if totalBytes+fc.Size > maxReadFilesTotalBytes {
			fr.Truncated = true
			fr.OmittedReason = fmt.Sprintf("adding %d bytes would exceed total limit %d", fc.Size, maxReadFilesTotalBytes)
			result.Files = append(result.Files, fr)
			truncated = true
			continue
		}

		fr.Lines = fc.Lines
		totalBytes += fc.Size
		result.ReturnedFiles++
		result.ReturnedBytes = totalBytes

		result.Files = append(result.Files, fr)
	}

	if truncated {
		result.Truncated = true
	}

	return result, nil
}

// SearchFilesInRepo runs a text search under a repo-relative directory.
func SearchFilesInRepo(repoPath string, path string, pattern string, maxMatches int) (SearchResult, error) {
	if strings.TrimSpace(path) == "" {
		return SearchFiles(repoPath, pattern, maxMatches)
	}
	resolved, _, err := repoRelativePath(repoPath, path)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchFiles(resolved, pattern, maxMatches)
}

// Hash returns a SHA-256 hex digest for data.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

const protectedLeanGenericToolMessage = "policy_denied: generic file access to protected task registry source is disabled for this path only; continue with task_current/task_get/task_graph/task_context or use a focused search that skips protected registry files"

func rejectProtectedLeanPath(path string) error {
	if isProtectedLeanPath(path) {
		return fmt.Errorf("%s: %s", protectedLeanGenericToolMessage, filepath.ToSlash(filepath.Clean(path)))
	}
	return nil
}

func isProtectedLeanPath(path string) bool {
	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(strings.TrimSpace(path))))
	if clean == "mcpaihelperproject/activetasks.lean" {
		return true
	}
	if strings.HasPrefix(clean, "mcpaihelperproject/taskregistry") && strings.HasSuffix(clean, ".lean") {
		return true
	}
	return strings.HasPrefix(clean, "tasks/") && strings.HasSuffix(clean, ".lean")
}

func isTaskRegistryDir(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return false
	}
	clean := strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	return clean == "obsidian-tasks" || clean == "tasks" || clean == "mcpaihelperproject"
}

func cleanPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path is required")
	}
	if err := rejectProtectedLeanPath(path); err != nil {
		return "", err
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if strings.Contains(clean, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path %q", path)
	}
	return clean, nil
}

func repoRelativePath(repoPath string, path string) (string, string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return "", "", errors.New("repo_path is required")
	}
	if strings.TrimSpace(path) == "" {
		return "", "", errors.New("path is required")
	}
	if filepath.IsAbs(path) {
		return "", "", fmt.Errorf("path must be repo-relative when repo_path is set: %q", path)
	}
	repoAbs, err := filepath.Abs(repoPath)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(repoAbs)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("repo_path %q is not a directory", repoAbs)
	}
	rel := filepath.Clean(path)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path escapes repo_path: %q", path)
	}
	if err := rejectProtectedLeanPath(rel); err != nil {
		return "", "", err
	}
	repoReal, err := filepath.EvalSymlinks(repoAbs)
	if err != nil {
		return "", "", err
	}
	resolved := filepath.Join(repoAbs, rel)
	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", "", err
	}
	if realPath != repoReal && !strings.HasPrefix(realPath, repoReal+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path escapes repo_path via symlink: %q", path)
	}
	return realPath, filepath.ToSlash(rel), nil
}
