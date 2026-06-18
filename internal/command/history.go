package command

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zol/mcp-ai-helper/internal/evidence"
	"github.com/zol/mcp-ai-helper/internal/project"
)

const (
	defaultRetentionDays = 30
	defaultMaxRecords    = 2000
)

// HistoryPolicy controls command log persistence and cleanup.
type HistoryPolicy struct {
	Dir           string `yaml:"dir" json:"dir"`
	RetentionDays int    `yaml:"retention_days" json:"retention_days"`
	MaxRecords    int    `yaml:"max_records" json:"max_records"`
	Compress      bool   `yaml:"compress" json:"compress"`
}

// History stores bounded command output so callers can re-filter it later.
type History struct {
	mu       sync.RWMutex
	root     string
	policy   HistoryPolicy
	projects *project.Store
	records  map[string]Record
}

// Record is one retained command execution artifact.
type Record struct {
	CommandID   string    `json:"command_id"`
	Status      string    `json:"status"`
	RepoPath    string    `json:"repo_path"`
	Command     string    `json:"command"`
	CWD         string    `json:"cwd"`
	ExitCode    int       `json:"exit_code"`
	DurationMS  int64     `json:"duration_ms,omitempty"`
	Truncated   bool      `json:"truncated"`
	Stdout      []string  `json:"stdout"`
	Stderr      []string  `json:"stderr"`
	Combined    []string  `json:"combined"`
	OutputHash  string    `json:"output_hash"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type indexEntry struct {
	CommandID  string    `json:"command_id"`
	Status     string    `json:"status"`
	RepoPath   string    `json:"repo_path"`
	Project    string    `json:"project"`
	Command    string    `json:"command"`
	CWD        string    `json:"cwd"`
	ExitCode   int       `json:"exit_code"`
	Truncated  bool      `json:"truncated"`
	OutputHash string    `json:"output_hash"`
	CreatedAt  time.Time `json:"created_at"`
	File       string    `json:"file"`
	IndexPath  string    `json:"-"`
}

// NewHistory creates a persistent command history under policy.Dir.
func NewHistory(policy HistoryPolicy) (*History, error) {
	policy = normalizeHistoryPolicy(policy)
	projects, err := project.NewStore(policy.Dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(projects.Root(), "repos"), 0o700); err != nil {
		return nil, fmt.Errorf("create helper repo directory: %w", err)
	}
	history := &History{
		root:     projects.Root(),
		policy:   policy,
		projects: projects,
		records:  map[string]Record{},
	}
	if err := history.loadIndex(); err != nil {
		return nil, err
	}
	if err := history.Cleanup(); err != nil {
		return nil, err
	}
	return history, nil
}

// NewInMemoryHistory creates a volatile fallback history.
func NewInMemoryHistory() *History {
	return &History{policy: HistoryPolicy{}, records: map[string]Record{}}
}

// Put stores record in memory and on disk.
func (h *History) Put(record Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.TrimSpace(record.CommandID) == "" {
		return errors.New("command_id is required")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	if h.root == "" {
		h.records[record.CommandID] = record
		return nil
	}
	logsDir, projectName, err := h.logsDir(record.RepoPath)
	if err != nil {
		return err
	}
	recordsDir := filepath.Join(logsDir, "records")
	if err := os.MkdirAll(recordsDir, 0o700); err != nil {
		return fmt.Errorf("create command log record directory: %w", err)
	}
	fileName := record.CommandID + ".json"
	if h.policy.Compress {
		fileName += ".gz"
	}
	recordPath := filepath.Join(recordsDir, fileName)
	if err := writeJSONRecord(recordPath, record, h.policy.Compress); err != nil {
		return err
	}
	entry := indexEntry{
		CommandID:  record.CommandID,
		Status:     record.Status,
		RepoPath:   record.RepoPath,
		Project:    projectName,
		Command:    record.Command,
		CWD:        record.CWD,
		ExitCode:   record.ExitCode,
		Truncated:  record.Truncated,
		OutputHash: record.OutputHash,
		CreatedAt:  record.CreatedAt,
		File:       recordPath,
	}
	if err := appendIndexEntry(filepath.Join(logsDir, "index.jsonl"), entry); err != nil {
		return err
	}
	h.records[record.CommandID] = record
	return nil
}

// Filter returns a previously executed command record with a new output filter applied.
func (h *History) Filter(commandID string, filter Filter) (Result, error) {
	record, ok, err := h.getRecord(commandID)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, fmt.Errorf("command history entry %q not found", commandID)
	}
	filteredLines, filterTruncated, err := applyFilter(record.Combined, filter)
	if err != nil {
		return Result{}, err
	}
	durationMS := record.DurationMS
	if record.Status == "running" && !record.StartedAt.IsZero() {
		durationMS = time.Since(record.StartedAt).Milliseconds()
	}
	return Result{
		Status:        record.Status,
		CommandID:     commandID,
		Command:       record.Command,
		CWD:           record.CWD,
		ExitCode:      record.ExitCode,
		DurationMS:    durationMS,
		Truncated:     record.Truncated || filterTruncated,
		StdoutTail:    tail80(record.Stdout),
		StderrTail:    tail80(record.Stderr),
		FilteredLines: filteredLines,
		EvidenceLines: evidenceFromLines(record.Combined),
		OutputHash:    record.OutputHash,
		NextCall:      nextCallForStatus(record.Status, commandID),
	}, nil
}

func (h *History) getRecord(commandID string) (Record, bool, error) {
	h.mu.RLock()
	record, ok := h.records[commandID]
	h.mu.RUnlock()
	if ok || h.root == "" {
		return record, ok, nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if record, ok := h.records[commandID]; ok {
		return record, true, nil
	}
	if err := h.loadIndex(); err != nil {
		return Record{}, false, err
	}
	record, ok = h.records[commandID]
	return record, ok, nil
}

// Cleanup removes records outside retention policy and rewrites the search indexes.
func (h *History) Cleanup() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.root == "" {
		return nil
	}
	entries, err := h.readEntries()
	if err != nil {
		return err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -h.policy.RetentionDays)
	kept := make([]indexEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.CreatedAt.Before(cutoff) {
			h.removeEntry(entry)
			continue
		}
		kept = append(kept, entry)
	}
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].CreatedAt.Before(kept[j].CreatedAt)
	})
	if h.policy.MaxRecords > 0 && len(kept) > h.policy.MaxRecords {
		drop := len(kept) - h.policy.MaxRecords
		for _, entry := range kept[:drop] {
			h.removeEntry(entry)
		}
		kept = kept[drop:]
	}
	return rewriteIndexes(entries, kept)
}

// ListRequest controls which command records to return.
type ListRequest struct {
	Status  string `json:"status,omitempty"`
	RepoPath string `json:"repo_path,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// ListEntry is a compact summary of one command record for listing.
type ListEntry struct {
	CommandID  string    `json:"command_id"`
	Status     string    `json:"status"`
	RepoPath   string    `json:"repo_path,omitempty"`
	Command    string    `json:"command"`
	CWD        string    `json:"cwd"`
	ExitCode   int       `json:"exit_code"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListResult holds the bounded result of a command list operation.
type ListResult struct {
	Entries []ListEntry `json:"entries"`
	Total   int         `json:"total"`
}

// List returns a bounded, sorted (newest-first) list of command records.
func (h *History) List(req ListRequest) (ListResult, error) {
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 200 {
		req.Limit = 200
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.root == "" {
		// In-memory fallback: iterate records map.
		entries := make([]ListEntry, 0, len(h.records))
		for _, r := range h.records {
			if !matchesFilter(r, req) {
				continue
			}
			entries = append(entries, recordToEntry(r))
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].CreatedAt.After(entries[j].CreatedAt)
		})
		if len(entries) > req.Limit {
			entries = entries[:req.Limit]
		}
		return ListResult{Entries: entries, Total: len(entries)}, nil
	}
	// Persistent history: read index entries.
	idxEntries, err := h.readEntries()
	if err != nil {
		return ListResult{}, err
	}
	entries := make([]ListEntry, 0, len(idxEntries))
	for _, e := range idxEntries {
		if req.Status != "" && e.Status != req.Status {
			continue
		}
		if req.RepoPath != "" && e.RepoPath != req.RepoPath {
			continue
		}
		entries = append(entries, ListEntry{
			CommandID: e.CommandID,
			Status:    e.Status,
			RepoPath:  e.RepoPath,
			Command:   e.Command,
			CWD:       e.CWD,
			ExitCode:  e.ExitCode,
			CreatedAt: e.CreatedAt,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	total := len(entries)
	if len(entries) > req.Limit {
		entries = entries[:req.Limit]
	}
	return ListResult{Entries: entries, Total: total}, nil
}

func matchesFilter(r Record, req ListRequest) bool {
	if req.Status != "" && r.Status != req.Status {
		return false
	}
	if req.RepoPath != "" && r.RepoPath != req.RepoPath {
		return false
	}
	return true
}

func recordToEntry(r Record) ListEntry {
	e := ListEntry{
		CommandID: r.CommandID,
		Status:    r.Status,
		RepoPath:  r.RepoPath,
		Command:   r.Command,
		CWD:       r.CWD,
		ExitCode:  r.ExitCode,
		DurationMS: r.DurationMS,
		CreatedAt: r.CreatedAt,
	}
	if r.Status == "running" && !r.StartedAt.IsZero() {
		e.DurationMS = time.Since(r.StartedAt).Milliseconds()
	}
	return e
}

func (h *History) logsDir(repoPath string) (string, string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return filepath.Join(h.root, "logs"), "", nil
	}
	name, err := project.Name(repoPath)
	if err != nil {
		return "", "", err
	}
	logsDir, err := h.projects.LogsDir(repoPath)
	if err != nil {
		return "", "", err
	}
	return logsDir, name, nil
}

func (h *History) removeEntry(entry indexEntry) {
	_ = os.Remove(entry.File)
	delete(h.records, entry.CommandID)
}

func (h *History) loadIndex() error {
	entries, err := h.readEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		record, err := readJSONRecord(entry.File)
		if err != nil {
			continue
		}
		h.records[entry.CommandID] = record
	}
	return nil
}

func (h *History) readEntries() ([]indexEntry, error) {
	indexPaths, err := h.indexPaths()
	if err != nil {
		return nil, err
	}
	var entries []indexEntry
	for _, indexPath := range indexPaths {
		fileEntries, err := readEntriesFile(indexPath)
		if err != nil {
			return nil, err
		}
		entries = append(entries, fileEntries...)
	}
	return entries, nil
}

func (h *History) indexPaths() ([]string, error) {
	pattern := filepath.Join(h.root, "repos", "*", "logs", "index.jsonl")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("scan command log indexes: %w", err)
	}
	legacy := filepath.Join(h.root, "logs", "index.jsonl")
	if _, err := os.Stat(legacy); err == nil {
		paths = append(paths, legacy)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("stat legacy command log index: %w", err)
	}
	return paths, nil
}

func readEntriesFile(indexPath string) ([]indexEntry, error) {
	// #nosec G304 -- indexPath is discovered under the configured helper log root.
	file, err := os.Open(indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open command log index: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	var entries []indexEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry indexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("decode command log index: %w", err)
		}
		entry.IndexPath = indexPath
		entry.File = normalizeRecordPath(indexPath, entry.File)
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan command log index: %w", err)
	}
	return entries, nil
}

func normalizeRecordPath(indexPath string, recordPath string) string {
	if filepath.IsAbs(recordPath) {
		return recordPath
	}
	return filepath.Join(filepath.Dir(indexPath), filepath.FromSlash(recordPath))
}

func appendIndexEntry(path string, entry indexEntry) error {
	// #nosec G304 -- path is derived from the configured command log directory and normalized record id.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open command log index: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	entry.IndexPath = ""
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write command log index: %w", err)
	}
	return nil
}

func rewriteIndexes(oldEntries []indexEntry, keptEntries []indexEntry) error {
	grouped := map[string][]indexEntry{}
	for _, entry := range oldEntries {
		grouped[entry.IndexPath] = nil
	}
	for _, entry := range keptEntries {
		grouped[entry.IndexPath] = append(grouped[entry.IndexPath], entry)
	}
	for indexPath, entries := range grouped {
		if err := rewriteIndex(indexPath, entries); err != nil {
			return err
		}
	}
	return nil
}

func rewriteIndex(path string, entries []indexEntry) error {
	tmpPath := path + ".tmp"
	// #nosec G304 -- tmpPath is derived from the configured command log index path.
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open temporary command log index: %w", err)
	}
	for _, entry := range entries {
		entry.IndexPath = ""
		data, err := json.Marshal(entry)
		if err != nil {
			_ = file.Close()
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			_ = file.Close()
			return fmt.Errorf("write temporary command log index: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary command log index: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace command log index: %w", err)
	}
	return nil
}

func writeJSONRecord(path string, record Record, compress bool) error {
	// #nosec G304 -- path is derived from the configured command log directory and normalized record id.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open command log record: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	if compress {
		writer := gzip.NewWriter(file)
		if err := json.NewEncoder(writer).Encode(record); err != nil {
			_ = writer.Close()
			return fmt.Errorf("encode compressed command log record: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("close compressed command log record: %w", err)
		}
		return nil
	}
	if err := json.NewEncoder(file).Encode(record); err != nil {
		return fmt.Errorf("encode command log record: %w", err)
	}
	return nil
}

func readJSONRecord(path string) (Record, error) {
	// #nosec G304 -- path is read from our own command log index under the configured log directory.
	file, err := os.Open(path)
	if err != nil {
		return Record{}, fmt.Errorf("open command log record: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()
	var reader interface {
		Read([]byte) (int, error)
	} = file
	if strings.HasSuffix(path, ".gz") {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return Record{}, fmt.Errorf("open compressed command log record: %w", err)
		}
		defer func() {
			_ = gzipReader.Close()
		}()
		reader = gzipReader
	}
	var record Record
	if err := json.NewDecoder(reader).Decode(&record); err != nil {
		return Record{}, fmt.Errorf("decode command log record: %w", err)
	}
	return record, nil
}

func evidenceFromLines(lines []string) []evidence.Line {
	return evidence.Select(lines, 30)
}

func normalizeHistoryPolicy(policy HistoryPolicy) HistoryPolicy {
	home, homeErr := os.UserHomeDir()
	if policy.Dir == "" {
		if homeErr == nil {
			policy.Dir = filepath.Join(home, ".mcp-ai-helper")
		} else {
			policy.Dir = filepath.Join(".", ".mcp-ai-helper")
		}
	} else if strings.HasPrefix(policy.Dir, "~/") && homeErr == nil {
		policy.Dir = filepath.Join(home, strings.TrimPrefix(policy.Dir, "~/"))
	}
	if policy.RetentionDays <= 0 {
		policy.RetentionDays = defaultRetentionDays
	}
	if policy.MaxRecords <= 0 {
		policy.MaxRecords = defaultMaxRecords
	}
	return policy
}
