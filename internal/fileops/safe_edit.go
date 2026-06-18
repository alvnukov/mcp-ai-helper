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
	"time"
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

// --- CreateIfAbsent ---

// CreateIfAbsentRequest describes a file creation that only happens if the file doesn't exist.
type CreateIfAbsentRequest struct {
	Path       string `json:"path"`
	RepoPath   string `json:"repo_path,omitempty"`
	Content    string `json:"content,omitempty"`
	ContentB64 string `json:"content_b64,omitempty"`
	Mode       int    `json:"mode,omitempty"`
}

// CreateIfAbsent creates a file with the given content only if it doesn't already exist.
// Returns already_present if the file exists, ok if created.
func CreateIfAbsent(req CreateIfAbsentRequest) (ReplaceResult, error) {
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
	content, err := resolveContent(req.Content, req.ContentB64)
	if err != nil {
		return ReplaceResult{}, err
	}
	if content == "" {
		return ReplaceResult{}, errors.New("content is required (set content or content_b64)")
	}
	// #nosec G304 -- clean is resolved from a validated local path or repo-relative path.
	if _, statErr := os.Stat(clean); statErr == nil {
		return ReplaceResult{Status: "already_present", Path: clean, Changed: false, Reason: "file already exists"}, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return ReplaceResult{}, statErr
	}
	mode := os.FileMode(0o644)
	if req.Mode != 0 {
		mode = os.FileMode(req.Mode)
	}
	if err := os.WriteFile(clean, []byte(content), mode); err != nil {
		return ReplaceResult{}, err
	}
	newHash := Hash([]byte(content))
	return ReplaceResult{Status: "ok", Path: clean, Changed: true, NewHash: newHash}, nil
}

// --- AppendUnique ---

// AppendUniqueRequest describes an append that only happens if the content is not already present.
type AppendUniqueRequest struct {
	Path         string `json:"path"`
	RepoPath     string `json:"repo_path,omitempty"`
	ExpectedHash string `json:"expected_hash"`
	Content      string `json:"content,omitempty"`
	ContentB64   string `json:"content_b64,omitempty"`
	Separator    string `json:"separator,omitempty"`
}

// AppendUnique appends content to the end of a file only if the exact content
// is not already present anywhere in the file. Hash-guarded.
func AppendUnique(req AppendUniqueRequest) (ReplaceResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return ReplaceResult{}, errors.New("path is required")
	}
	if req.ExpectedHash == "" {
		return ReplaceResult{}, errors.New("expected_hash is required")
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
	content, err := resolveContent(req.Content, req.ContentB64)
	if err != nil {
		return ReplaceResult{}, err
	}
	if content == "" {
		return ReplaceResult{}, errors.New("content is required (set content or content_b64)")
	}
	// #nosec G304 -- clean is resolved from a validated local path or repo-relative path.
	data, readErr := os.ReadFile(clean)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return ReplaceResult{Status: "conflict", Path: clean, Reason: "file does not exist"}, nil
		}
		return ReplaceResult{}, readErr
	}
	oldHash := Hash(data)
	if oldHash != req.ExpectedHash {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "file hash changed after snapshot"}, nil
	}
	text := string(data)
	if strings.Contains(text, content) {
		return ReplaceResult{Status: "ok", Path: clean, Changed: false, OldHash: oldHash, NewHash: oldHash, Reason: "content already present"}, nil
	}
	separator := req.Separator
	if separator == "" {
		separator = "\n"
	}
	var next string
	if len(data) == 0 {
		next = content
	} else if strings.HasSuffix(text, separator) {
		next = text + content
	} else {
		next = text + separator + content
	}
	newHash := Hash([]byte(next))
	if err := os.WriteFile(clean, []byte(next), 0o600); err != nil {
		return ReplaceResult{}, err
	}
	return ReplaceResult{Status: "ok", Path: clean, Changed: true, OldHash: oldHash, NewHash: newHash}, nil
}

// --- DeleteExactBlock ---

// DeleteExactBlockRequest describes deletion of an exact multi-line block.
type DeleteExactBlockRequest struct {
	Path         string `json:"path"`
	RepoPath     string `json:"repo_path,omitempty"`
	ExpectedHash string `json:"expected_hash"`
	Block        string `json:"block,omitempty"`
	BlockB64     string `json:"block_b64,omitempty"`
}

// DeleteExactBlock removes an exact multi-line block from a file.
// If the block is not found, returns ok with changed=false (idempotent).
// If the block appears more than once, returns conflict.
func DeleteExactBlock(req DeleteExactBlockRequest) (ReplaceResult, error) {
	if strings.TrimSpace(req.Path) == "" {
		return ReplaceResult{}, errors.New("path is required")
	}
	if req.ExpectedHash == "" {
		return ReplaceResult{}, errors.New("expected_hash is required")
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
	block, err := resolveContent(req.Block, req.BlockB64)
	if err != nil {
		return ReplaceResult{}, err
	}
	if block == "" {
		return ReplaceResult{}, errors.New("block is required (set block or block_b64)")
	}
	// #nosec G304 -- clean is resolved from a validated local path or repo-relative path.
	data, readErr := os.ReadFile(clean)
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			return ReplaceResult{Status: "conflict", Path: clean, Reason: "file does not exist"}, nil
		}
		return ReplaceResult{}, readErr
	}
	oldHash := Hash(data)
	if oldHash != req.ExpectedHash {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "file hash changed after snapshot"}, nil
	}
	text := string(data)
	count := strings.Count(text, block)
	if count == 0 {
		return ReplaceResult{Status: "ok", Path: clean, Changed: false, OldHash: oldHash, NewHash: oldHash, Reason: "block not found (already absent)"}, nil
	}
	if count > 1 {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "block is not unique"}, nil
	}
	next := strings.Replace(text, block, "", 1)
	// Collapse triple blank lines that may result from deletion.
	next = strings.ReplaceAll(next, "\n\n\n", "\n\n")
	newHash := Hash([]byte(next))
	if err := os.WriteFile(clean, []byte(next), 0o600); err != nil {
		return ReplaceResult{}, err
	}
	return ReplaceResult{Status: "ok", Path: clean, Changed: true, OldHash: oldHash, NewHash: newHash}, nil
}

// resolveContent returns the content from plain text or base64-encoded text.
func resolveContent(plain string, b64 string) (string, error) {
	if b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("content_b64: invalid base64: %w", err)
		}
		return string(decoded), nil
	}
	return plain, nil
}

// --- WriteFile ---

// WriteFileRequest describes a file write with optional overwrite guard.
type WriteFileRequest struct {
	RepoPath     string `json:"repo_path"`
	Path         string `json:"path"`
	Content      string `json:"content,omitempty"`
	ContentB64   string `json:"content_b64,omitempty"`
	ExpectedHash string `json:"expected_hash,omitempty"`
	Mode         int    `json:"mode,omitempty"`
}

// WriteFile writes content to a file, creating parent directories if needed.
// If ExpectedHash is set and the file exists with a different hash, returns conflict.
// If the file already has the desired content, returns ok with changed=false (idempotent).
func WriteFile(req WriteFileRequest) (ReplaceResult, error) {
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
	content, err := resolveContent(req.Content, req.ContentB64)
	if err != nil {
		return ReplaceResult{}, err
	}
	if content == "" {
		return ReplaceResult{}, errors.New("content is required (set content or content_b64)")
	}
	// Check if file exists.
	// #nosec G304 -- clean is resolved from a validated local path or repo-relative path.
	existing, readErr := os.ReadFile(clean)
	if readErr == nil {
		// File exists.
		oldHash := Hash(existing)
		if req.ExpectedHash != "" && oldHash != req.ExpectedHash {
			return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "file hash changed after snapshot"}, nil
		}
		newHash := Hash([]byte(content))
		if oldHash == newHash {
			return ReplaceResult{Status: "ok", Path: clean, Changed: false, OldHash: oldHash, NewHash: oldHash, Reason: "content already matches"}, nil
		}
		if err := os.WriteFile(clean, []byte(content), 0o600); err != nil {
			return ReplaceResult{}, err
		}
		return ReplaceResult{Status: "ok", Path: clean, Changed: true, OldHash: oldHash, NewHash: newHash}, nil
	}
	if !errors.Is(readErr, os.ErrNotExist) {
		return ReplaceResult{}, readErr
	}
	// File doesn't exist. Create parent dirs.
	dir := filepath.Dir(clean)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ReplaceResult{}, fmt.Errorf("create parent dirs: %w", err)
	}
	mode := os.FileMode(0o644)
	if req.Mode != 0 {
		mode = os.FileMode(req.Mode)
	}
	if err := os.WriteFile(clean, []byte(content), mode); err != nil {
		return ReplaceResult{}, err
	}
	newHash := Hash([]byte(content))
	return ReplaceResult{Status: "ok", Path: clean, Changed: true, NewHash: newHash}, nil
}

// --- ListDir ---

// ListDirRequest describes a directory listing query.
type ListDirRequest struct {
	RepoPath string `json:"repo_path"`
	Path     string `json:"path,omitempty"` // repo-relative, defaults to "."
}

// DirEntry is one item in a directory listing.
type DirEntry struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	IsDir        bool      `json:"is_dir"`
	Size         int64     `json:"size,omitempty"`
	ModifiedAt   string    `json:"modified_at,omitempty"`
	IsSymlink    bool      `json:"is_symlink,omitempty"`
}

// ListDirResult holds structured directory listing.
type ListDirResult struct {
	Path    string     `json:"path"`
	Entries []DirEntry `json:"entries"`
	Total   int        `json:"total"`
}

// ListDir returns a structured directory listing.
func ListDir(req ListDirRequest) (ListDirResult, error) {
	var dir string
	if strings.TrimSpace(req.RepoPath) != "" {
		resolved, _, err := repoRelativePath(req.RepoPath, req.Path)
		if err != nil {
			return ListDirResult{}, err
		}
		dir = resolved
	} else {
		if req.Path == "" {
			dir = "."
		} else {
			var err error
			dir, err = cleanPath(req.Path)
			if err != nil {
				return ListDirResult{}, err
			}
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ListDirResult{}, err
	}

	result := ListDirResult{
		Path:    dir,
		Entries: make([]DirEntry, 0, len(entries)),
	}

	for _, e := range entries {
		entry := DirEntry{
			Name:  e.Name(),
			Path:  filepath.ToSlash(filepath.Join(dir, e.Name())),
			IsDir: e.IsDir(),
		}
		if e.Type()&os.ModeSymlink != 0 {
			entry.IsSymlink = true
		}
		info, err := e.Info()
		if err == nil {
			entry.Size = info.Size()
			entry.ModifiedAt = info.ModTime().UTC().Format(time.RFC3339)
		}
		result.Entries = append(result.Entries, entry)
	}
	result.Total = len(result.Entries)
	return result, nil
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
