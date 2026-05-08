// Package fileops provides guarded, idempotent file edit operations.
package fileops

import (
	"crypto/sha256"
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
type ReplaceRequest struct {
	RepoPath     string `json:"repo_path"`
	Path         string `json:"path"`
	ExpectedHash string `json:"expected_hash"`
	Old          string `json:"old"`
	New          string `json:"new"`
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
	snapshot.RepoPath = filepath.Clean(repoPath)
	snapshot.RelativePath = rel
	return snapshot, nil
}

// ApplyGuardedReplace replaces one unique old span when the file hash still matches.
func ApplyGuardedReplace(req ReplaceRequest) (ReplaceResult, error) {
	path := req.Path
	if strings.TrimSpace(req.RepoPath) != "" {
		resolved, _, err := repoRelativePath(req.RepoPath, req.Path)
		if err != nil {
			return ReplaceResult{}, err
		}
		path = resolved
	}
	clean, err := cleanPath(path)
	if err != nil {
		return ReplaceResult{}, err
	}
	if req.ExpectedHash == "" {
		return ReplaceResult{}, errors.New("expected_hash is required")
	}
	if req.Old == "" {
		return ReplaceResult{}, errors.New("old text is required")
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
	if strings.Contains(text, req.New) && !strings.Contains(text, req.Old) {
		return ReplaceResult{Status: "ok", Path: clean, Changed: false, OldHash: oldHash, NewHash: oldHash, Reason: "desired text already present"}, nil
	}
	count := strings.Count(text, req.Old)
	if count == 0 {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "old text not found"}, nil
	}
	if count > 1 {
		return ReplaceResult{Status: "conflict", Path: clean, OldHash: oldHash, Reason: "old text is not unique"}, nil
	}
	next := strings.Replace(text, req.Old, req.New, 1)
	newHash := Hash([]byte(next))
	// #nosec G703 -- clean is resolved from a validated local path or repo-relative path.
	if err := os.WriteFile(clean, []byte(next), 0o600); err != nil {
		return ReplaceResult{}, err
	}
	return ReplaceResult{Status: "ok", Path: clean, Changed: newHash != oldHash, OldHash: oldHash, NewHash: newHash}, nil
}

// Hash returns a SHA-256 hex digest for data.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cleanPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path is required")
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
	resolved := filepath.Join(repoAbs, rel)
	return resolved, filepath.ToSlash(rel), nil
}
