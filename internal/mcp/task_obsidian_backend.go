package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/alvnukov/mcp-ai-helper/internal/safefs"
	"github.com/alvnukov/mcp-ai-helper/internal/tasks"
)

// obsidianScanState is shared between a backend and its per-repo views so
// list metadata stays consistent no matter which view ran the last scan.
type obsidianScanState struct {
	mu       sync.Mutex
	lastScan taskListMetadata
}

type obsidianTaskBackend struct {
	dir   string
	state *obsidianScanState
}

type obsidianTaskScan struct {
	Tasks    []tasks.Task
	Metadata taskListMetadata
}

func newObsidianTaskBackend(dir string) taskBackend {
	return &obsidianTaskBackend{dir: dir, state: &obsidianScanState{}}
}

// NewObsidianTaskBackend creates a task backend rooted at an Obsidian notes directory.
func NewObsidianTaskBackend(dir string) taskBackend {
	return newObsidianTaskBackend(dir)
}

// dirFor resolves the configured registry directory for one operation. A
// relative directory (for example the obsidian-tasks default) resolves
// against the repo the operation targets instead of the server working
// directory.
func (b *obsidianTaskBackend) dirFor(repoPath string) (string, error) {
	if strings.TrimSpace(b.dir) == "" {
		return "", errors.New("obsidian task registry path is required")
	}
	if filepath.IsAbs(b.dir) || strings.TrimSpace(repoPath) == "" {
		return b.dir, nil
	}
	repo, err := filepath.Abs(strings.TrimSpace(repoPath))
	if err != nil {
		return "", fmt.Errorf("resolve repo_path: %w", err)
	}
	resolved := filepath.Join(repo, b.dir)
	if !strings.HasPrefix(resolved, repo+string(os.PathSeparator)) {
		return "", fmt.Errorf("obsidian task registry path %q escapes repo_path", b.dir)
	}
	return resolved, nil
}

// forRepo returns a view of the backend with the registry directory resolved
// for repoPath, sharing scan state with the original backend.
func (b *obsidianTaskBackend) forRepo(repoPath string) (*obsidianTaskBackend, error) {
	dir, err := b.dirFor(repoPath)
	if err != nil {
		return nil, err
	}
	if dir == b.dir {
		return b, nil
	}
	return &obsidianTaskBackend{dir: dir, state: b.state}, nil
}

type yamlStringList []string

type taskNote struct {
	ID                 string         `yaml:"id"`
	Title              string         `yaml:"title"`
	Status             string         `yaml:"status"`
	Priority           string         `yaml:"priority,omitempty"`
	ModelLevel         string         `yaml:"model_level,omitempty"`
	TaskType           string         `yaml:"task_type,omitempty"`
	ParentID           string         `yaml:"parent_id,omitempty"`
	Tags               []string       `yaml:"tags,omitempty"`
	Branch             string         `yaml:"branch,omitempty"`
	WorktreePath       string         `yaml:"worktree_path,omitempty"`
	AcceptanceCriteria yamlStringList `yaml:"acceptance_criteria,omitempty"`
	VerificationPlan   yamlStringList `yaml:"verification_plan,omitempty"`
	CreatedAt          string         `yaml:"created_at,omitempty"`
	UpdatedAt          string         `yaml:"updated_at,omitempty"`
	Body               string         `yaml:"-"`
	BodySection        string         `yaml:"-"`
	AccCriteriaSection []string       `yaml:"-"`
	VerPlanSection     []string       `yaml:"-"`
}

func (l *yamlStringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		out := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := yamlListItemString(item)
			if err != nil {
				return err
			}
			out = append(out, value)
		}
		*l = out
		return nil
	case yaml.ScalarNode:
		if strings.TrimSpace(node.Value) == "" {
			*l = nil
			return nil
		}
		*l = yamlStringList{node.Value}
		return nil
	default:
		return fmt.Errorf("expected YAML string list, got node kind %d", node.Kind)
	}
}

func yamlListItemString(node *yaml.Node) (string, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Value, nil
	case yaml.MappingNode:
		if len(node.Content) == 2 && node.Content[0].Kind == yaml.ScalarNode && node.Content[1].Kind == yaml.ScalarNode {
			return node.Content[0].Value + ": " + node.Content[1].Value, nil
		}
		return "", fmt.Errorf("expected single key/value string item, got mapping with %d nodes", len(node.Content))
	default:
		return "", fmt.Errorf("expected YAML string item, got node kind %d", node.Kind)
	}
}

var errInvalidFrontmatter = errors.New("invalid frontmatter")
var errMissingRequired = errors.New("missing required field")

func obsidianNoteName(id string) string {
	return id + ".md"
}

func (b *obsidianTaskBackend) openRoot() (*safefs.Root, error) {
	if strings.TrimSpace(b.dir) == "" {
		return nil, errors.New("obsidian task registry path is required")
	}
	root, err := safefs.Open(b.dir)
	if err != nil {
		return nil, fmt.Errorf("open obsidian task registry: %w", err)
	}
	return root, nil
}

func (b *obsidianTaskBackend) ensureRoot() (*safefs.Root, error) {
	if strings.TrimSpace(b.dir) == "" {
		return nil, errors.New("obsidian task registry path is required")
	}
	root, err := safefs.Ensure(b.dir, 0o700)
	if err != nil {
		return nil, fmt.Errorf("initialize obsidian task registry: %w", err)
	}
	return root, nil
}

func (b *obsidianTaskBackend) ensureDir() error {
	root, err := b.ensureRoot()
	if err != nil {
		return err
	}
	if err := root.Close(); err != nil {
		return fmt.Errorf("close obsidian task registry: %w", err)
	}
	return nil
}

func (b *obsidianTaskBackend) ListCurrent(ctx context.Context, repoPath string) ([]tasks.Task, string, error) {
	all, _, err := b.ListAll(ctx, repoPath)
	if err != nil {
		return nil, "obsidian_registry", err
	}
	active := make([]tasks.Task, 0, len(all))
	for _, t := range all {
		switch t.Status {
		case "todo", "in_progress", "blocked":
			active = append(active, t)
		}
	}
	return active, "obsidian_registry", nil
}

func (b *obsidianTaskBackend) ListAll(_ context.Context, repoPath string) ([]tasks.Task, string, error) {
	scoped, err := b.forRepo(repoPath)
	if err != nil {
		return nil, "obsidian_registry", err
	}
	all, err := scoped.readAll()
	if err != nil {
		return nil, "obsidian_registry", err
	}
	return withObsidianWorktreeContext(repoPath, all), "obsidian_registry", nil
}

func (b *obsidianTaskBackend) Get(_ context.Context, repoPath string, id string) (tasks.Task, string, error) {
	scoped, err := b.forRepo(repoPath)
	if err != nil {
		return tasks.Task{}, "obsidian_registry", err
	}
	t, err := scoped.readOne(id)
	if err != nil {
		return tasks.Task{}, "obsidian_registry", err
	}
	return tasks.WithWorktreeContext(repoPath, t), "obsidian_registry", nil
}

func (b *obsidianTaskBackend) Upsert(_ context.Context, req tasks.AddRequest) (taskMutationResult, error) {
	scoped, err := b.forRepo(req.RepoPath)
	if err != nil {
		return taskMutationResult{}, err
	}
	if err := scoped.ensureDir(); err != nil {
		return taskMutationResult{}, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return taskMutationResult{}, errors.New("title is required")
	}
	status := normalizeFrontmatterEnum(req.Status)
	if strings.TrimSpace(status) == "" {
		status = "todo"
	}
	if !validStatus(status) {
		return taskMutationResult{}, fmt.Errorf("invalid status %q; expected one of todo, in_progress, blocked, done", req.Status)
	}
	priority := normalizeFrontmatterEnum(req.Priority)
	if priority != "" && !validPriority(priority) {
		return taskMutationResult{}, fmt.Errorf("invalid priority %q; expected one of low, medium, high, critical", req.Priority)
	}
	modelLevel := normalizeFrontmatterEnum(req.ModelLevel)
	if modelLevel != "" && !validModelLevel(modelLevel) {
		return taskMutationResult{}, fmt.Errorf("invalid model_level %q; expected one of low, medium, high, very_high", req.ModelLevel)
	}
	id := req.ID
	if id == "" {
		id = tasks.NormalizeID(req.Title)
	}
	if !safeObsidianTaskID(id) {
		return taskMutationResult{}, fmt.Errorf("task id %q is not safe for an Obsidian task filename", id)
	}
	now := time.Now().UTC()
	existing, exists := scoped.tryRead(id)
	taskType := req.TaskType
	parentID := req.ParentID
	tags := req.Tags
	branch := req.Branch
	worktreePath := req.WorktreePath
	acceptanceCriteria := req.AcceptanceCriteria
	verificationPlan := req.VerificationPlan
	body := req.Body
	if exists {
		if strings.TrimSpace(req.Status) == "" {
			status = existing.Status
		}
		if strings.TrimSpace(req.Priority) == "" {
			priority = existing.Priority
		}
		if strings.TrimSpace(req.ModelLevel) == "" {
			modelLevel = existing.ModelLevel
		}
		if strings.TrimSpace(req.TaskType) == "" {
			taskType = existing.TaskType
		}
		if strings.TrimSpace(req.ParentID) == "" {
			parentID = existing.ParentID
		}
		if req.Tags == nil {
			tags = append([]string(nil), existing.Tags...)
		}
		if strings.TrimSpace(req.Branch) == "" {
			branch = existing.Branch
		}
		if strings.TrimSpace(req.WorktreePath) == "" {
			worktreePath = existing.WorktreePath
		}
		if req.AcceptanceCriteria == nil {
			acceptanceCriteria = append([]string(nil), existing.AcceptanceCriteria...)
		}
		if req.VerificationPlan == nil {
			verificationPlan = append([]string(nil), existing.VerificationPlan...)
		}
		if req.Body == "" {
			body = existing.Body
		}
	}
	createdAt := now
	updatedAt := now
	if req.PreserveTimestamps {
		createdAt = req.CreatedAt.UTC()
		updatedAt = req.UpdatedAt.UTC()
	} else {
		if exists && !existing.CreatedAt.IsZero() {
			createdAt = existing.CreatedAt.UTC()
		}
		if !req.CreatedAt.IsZero() {
			createdAt = req.CreatedAt.UTC()
		}
		if !req.UpdatedAt.IsZero() {
			updatedAt = req.UpdatedAt.UTC()
		}
	}
	note := taskNote{
		ID: id, Title: req.Title, Status: status,
		Priority: priority, ModelLevel: modelLevel,
		TaskType: taskType, ParentID: parentID,
		Tags: nonNilTags(tags), Branch: branch,
		WorktreePath:       worktreePath,
		AcceptanceCriteria: yamlStringList(acceptanceCriteria),
		VerificationPlan:   yamlStringList(verificationPlan),
		CreatedAt:          createdAt.Format(time.RFC3339Nano), UpdatedAt: updatedAt.Format(time.RFC3339Nano),
		Body: body,
	}
	if err := scoped.writeNote(note); err != nil {
		return taskMutationResult{}, err
	}
	task := tasks.WithWorktreeContext(req.RepoPath, noteToTask(note))
	return taskMutationResult{Task: task, Source: "obsidian_registry", Validation: "frontmatter parsed + file written", ChangedFiles: []string{id + ".md"}}, nil
}

func (b *obsidianTaskBackend) SetStatus(_ context.Context, req tasks.StatusRequest) (taskMutationResult, error) {
	scoped, err := b.forRepo(req.RepoPath)
	if err != nil {
		return taskMutationResult{}, err
	}
	if err := scoped.ensureDir(); err != nil {
		return taskMutationResult{}, err
	}
	status := normalizeFrontmatterEnum(req.Status)
	if strings.TrimSpace(status) == "" {
		return taskMutationResult{}, errors.New("status is required")
	}
	if !validStatus(status) {
		return taskMutationResult{}, fmt.Errorf("invalid status %q; expected one of todo, in_progress, blocked, done", req.Status)
	}
	note, err := scoped.readNote(req.ID)
	if err != nil {
		return taskMutationResult{}, err
	}
	note.Status = status
	note.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := scoped.writeNote(note); err != nil {
		return taskMutationResult{}, err
	}
	task := tasks.WithWorktreeContext(req.RepoPath, noteToTask(note))
	return taskMutationResult{Task: task, Source: "obsidian_registry", Validation: "frontmatter parsed + file written", ChangedFiles: []string{req.ID + ".md"}}, nil
}

func (b *obsidianTaskBackend) BatchUpsert(_ context.Context, req tasks.BatchUpsertRequest) (taskBatchMutationResult, error) {
	scoped, err := b.forRepo(req.RepoPath)
	if err != nil {
		return taskBatchMutationResult{}, err
	}
	if err := scoped.ensureDir(); err != nil {
		return taskBatchMutationResult{}, err
	}
	// close_missing writes MissingStatus straight into every note the batch did
	// not name, and ActiveStatuses decides which notes those are, so both are
	// checked here, before the first note is written. An unchecked
	// MissingStatus lands in many files at once and drops every one of them
	// from every listing, because scanNotes skips a note whose status it cannot
	// parse; an unchecked ActiveStatuses entry matches no note and closes
	// nothing while the batch still reports success.
	missingStatus := normalizeFrontmatterEnum(req.MissingStatus)
	if missingStatus == "" {
		missingStatus = "done"
	}
	activeStatuses := make([]string, 0, len(req.ActiveStatuses))
	for _, status := range req.ActiveStatuses {
		activeStatuses = append(activeStatuses, normalizeFrontmatterEnum(status))
	}
	if req.CloseMissing {
		if !validStatus(missingStatus) {
			return taskBatchMutationResult{}, fmt.Errorf("invalid missing_status %q; expected one of todo, in_progress, blocked, done", req.MissingStatus)
		}
		for i, status := range activeStatuses {
			if !validStatus(status) {
				return taskBatchMutationResult{}, fmt.Errorf("invalid active_statuses[%d] %q; expected one of todo, in_progress, blocked, done", i, req.ActiveStatuses[i])
			}
		}
	}
	upserted := make([]tasks.Task, 0, len(req.Tasks))
	changedFiles := make([]string, 0, len(req.Tasks))
	for _, item := range req.Tasks {
		if item.RepoPath == "" {
			item.RepoPath = req.RepoPath
		}
		result, err := b.Upsert(context.Background(), item)
		if err != nil {
			return taskBatchMutationResult{}, fmt.Errorf("batch upsert %s: %w", item.ID, err)
		}
		upserted = append(upserted, result.Task)
		for _, file := range result.ChangedFiles {
			changedFiles = appendChangedFile(changedFiles, file)
		}
	}
	closed := make([]tasks.Task, 0)
	if req.CloseMissing {
		batchIDs := make(map[string]bool, len(req.Tasks))
		for _, item := range req.Tasks {
			batchIDs[item.ID] = true
		}
		if len(activeStatuses) == 0 {
			activeStatuses = []string{"todo", "in_progress", "blocked"}
		}
		all, err := scoped.readAll()
		if err != nil {
			return taskBatchMutationResult{}, err
		}
		for _, t := range all {
			if batchIDs[t.ID] {
				continue
			}
			for _, s := range activeStatuses {
				if t.Status == s {
					note, err := scoped.readNote(t.ID)
					if err != nil {
						continue
					}
					note.Status = missingStatus
					note.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
					if err := scoped.writeNote(note); err != nil {
						continue
					}
					changedFiles = appendChangedFile(changedFiles, note.ID+".md")
					closed = append(closed, tasks.WithWorktreeContext(req.RepoPath, noteToTask(note)))
					break
				}
			}
		}
	}
	if _, err := scoped.readAll(); err != nil {
		return taskBatchMutationResult{}, err
	}
	meta := b.ListMetadata()
	for _, file := range meta.ChangedFiles {
		changedFiles = appendChangedFile(changedFiles, file)
	}
	return taskBatchMutationResult{Upserted: upserted, Closed: closed, Source: "obsidian_registry", Validation: "batch upsert complete; " + meta.Validation, ChangedFiles: changedFiles}, nil
}

func (b *obsidianTaskBackend) Delete(_ context.Context, req tasks.DeleteRequest) (taskMutationResult, error) {
	if strings.TrimSpace(req.ID) == "" {
		return taskMutationResult{}, errors.New("id is required")
	}
	scoped, err := b.forRepo(req.RepoPath)
	if err != nil {
		return taskMutationResult{}, err
	}
	root, err := scoped.openRoot()
	if err != nil {
		return taskMutationResult{}, err
	}
	defer func() { _ = root.Close() }()
	note, err := readObsidianNote(root, req.ID, true)
	if err != nil {
		return taskMutationResult{}, err
	}
	if err := root.Remove(obsidianNoteName(req.ID)); err != nil {
		return taskMutationResult{}, fmt.Errorf("delete task %s: %w", req.ID, err)
	}
	task := tasks.WithWorktreeContext(req.RepoPath, noteToTask(note))
	return taskMutationResult{Task: task, Source: "obsidian_registry", Validation: "file deleted", ChangedFiles: []string{req.ID + ".md"}}, nil
}

func (b *obsidianTaskBackend) ListMetadata() taskListMetadata {
	b.state.mu.Lock()
	defer b.state.mu.Unlock()
	return taskListMetadata{
		Validation:      b.state.lastScan.Validation,
		Diagnostics:     append([]taskRegistryDiagnostic(nil), b.state.lastScan.Diagnostics...),
		ChangedFiles:    append([]string(nil), b.state.lastScan.ChangedFiles...),
		RegistryCreated: b.state.lastScan.RegistryCreated,
		RegistryPath:    b.state.lastScan.RegistryPath,
	}
}

func (b *obsidianTaskBackend) setListMetadata(meta taskListMetadata) {
	b.state.mu.Lock()
	defer b.state.mu.Unlock()
	b.state.lastScan = taskListMetadata{
		Validation:      meta.Validation,
		Diagnostics:     append([]taskRegistryDiagnostic(nil), meta.Diagnostics...),
		ChangedFiles:    append([]string(nil), meta.ChangedFiles...),
		RegistryCreated: meta.RegistryCreated,
		RegistryPath:    meta.RegistryPath,
	}
}

func withObsidianWorktreeContext(repoPath string, items []tasks.Task) []tasks.Task {
	out := make([]tasks.Task, 0, len(items))
	for _, item := range items {
		out = append(out, tasks.WithWorktreeContext(repoPath, item))
	}
	return out
}

func (b *obsidianTaskBackend) readAll() ([]tasks.Task, error) {
	scan, err := b.scanNotes(false)
	if err != nil {
		b.setListMetadata(taskListMetadata{})
		return nil, err
	}
	b.setListMetadata(scan.Metadata)
	return scan.Tasks, nil
}

func (b *obsidianTaskBackend) scanNotes(autoHeal bool) (obsidianTaskScan, error) {
	// #nosec G703 -- b.dir is the validated task_registry path from config; safefs roots bound every access under it.
	info, statErr := os.Stat(b.dir)
	if errors.Is(statErr, os.ErrNotExist) {
		scan := obsidianTaskScan{Tasks: []tasks.Task{}}
		scan.Metadata.RegistryPath = b.dir
		scan.addDiagnostic("registry_missing", "", "configured Obsidian task registry does not exist; initialize it explicitly with task_registry_init", "error")
		scan.finishValidation()
		return scan, nil
	}
	if statErr != nil {
		return obsidianTaskScan{}, fmt.Errorf("stat obsidian task registry: %w", statErr)
	}
	if !info.IsDir() {
		return obsidianTaskScan{}, fmt.Errorf("obsidian task registry path is not a directory: %s", b.dir)
	}
	root, err := b.openRoot()
	if err != nil {
		return obsidianTaskScan{}, err
	}
	defer func() { _ = root.Close() }()
	entries, err := root.ReadDir(".")
	if err != nil {
		return obsidianTaskScan{}, fmt.Errorf("read obsidian task dir: %w", err)
	}
	scan := obsidianTaskScan{Tasks: make([]tasks.Task, 0, len(entries))}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filenameID := strings.TrimSuffix(entry.Name(), ".md")
		note, err := readObsidianNote(root, filenameID, false)
		if err != nil {
			scan.addDiagnostic("invalid_projection", entry.Name(), fmt.Sprintf("skipped invalid Obsidian task note: %v", err), "warning")
			continue
		}
		if note.ID != filenameID {
			if !b.autoHealFilenameIDMismatch(root, &scan, filenameID, note, autoHeal) {
				continue
			}
		}
		scan.Tasks = append(scan.Tasks, noteToTask(note))
	}
	sort.Slice(scan.Tasks, func(i, j int) bool { return scan.Tasks[i].ID < scan.Tasks[j].ID })
	scan.finishValidation()
	return scan, nil
}

func (b *obsidianTaskBackend) autoHealFilenameIDMismatch(root *safefs.Root, scan *obsidianTaskScan, filenameID string, note taskNote, autoHeal bool) bool {
	oldName := obsidianNoteName(filenameID)
	newName := obsidianNoteName(note.ID)
	if !safeObsidianTaskID(note.ID) {
		scan.addDiagnostic("projection_id_unsafe", oldName, fmt.Sprintf("frontmatter id %q cannot be used as a safe task filename", note.ID), "error")
		return false
	}
	if _, err := root.Stat(newName); err == nil {
		scan.addDiagnostic("projection_id_conflict", oldName, fmt.Sprintf("frontmatter id %q conflicts with existing %s; skipped without overwrite", note.ID, newName), "error")
		return false
	} else if !errors.Is(err, os.ErrNotExist) {
		scan.addDiagnostic("projection_id_conflict_check_failed", oldName, fmt.Sprintf("cannot check target %s: %v", newName, err), "error")
		return false
	}
	if !autoHeal {
		scan.addDiagnostic("projection_id_mismatch", oldName, fmt.Sprintf("frontmatter id %q differs from filename id %q", note.ID, filenameID), "warning")
		return false
	}
	if err := root.Rename(oldName, newName); err != nil {
		scan.addDiagnostic("projection_id_rename_failed", oldName, fmt.Sprintf("cannot rename to %s: %v", newName, err), "error")
		return false
	}
	scan.Metadata.ChangedFiles = appendChangedFile(scan.Metadata.ChangedFiles, oldName)
	scan.Metadata.ChangedFiles = appendChangedFile(scan.Metadata.ChangedFiles, newName)
	scan.addDiagnostic("projection_id_auto_healed", oldName, fmt.Sprintf("renamed %s to %s to match frontmatter id", oldName, newName), "info")
	return true
}

func safeObsidianTaskID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return false
	}
	return filepath.Clean(id) == id
}

func (scan *obsidianTaskScan) addDiagnostic(code string, file string, message string, severity string) {
	scan.Metadata.Diagnostics = append(scan.Metadata.Diagnostics, taskRegistryDiagnostic{Code: code, File: file, Message: message, Severity: severity})
}

func (scan *obsidianTaskScan) finishValidation() {
	scan.Metadata.Validation = fmt.Sprintf("obsidian registry validated: %d task(s), %d diagnostic(s), %d changed file(s)", len(scan.Tasks), len(scan.Metadata.Diagnostics), len(scan.Metadata.ChangedFiles))
}

func appendChangedFile(files []string, file string) []string {
	for _, existing := range files {
		if existing == file {
			return files
		}
	}
	return append(files, file)
}

func (b *obsidianTaskBackend) readOne(id string) (tasks.Task, error) {
	note, err := b.readNote(id)
	if err != nil {
		return tasks.Task{}, err
	}
	return noteToTask(note), nil
}

func (b *obsidianTaskBackend) tryRead(id string) (tasks.Task, bool) {
	t, err := b.readOne(id)
	if err != nil {
		return tasks.Task{}, false
	}
	return t, true
}

func (b *obsidianTaskBackend) readNote(id string) (taskNote, error) {
	root, err := b.openRoot()
	if err != nil {
		return taskNote{}, err
	}
	defer func() { _ = root.Close() }()
	return readObsidianNote(root, id, true)
}

func readObsidianNote(root *safefs.Root, id string, requireFilenameID bool) (taskNote, error) {
	if !safeObsidianTaskID(id) {
		return taskNote{}, fmt.Errorf("task id %q is not safe for an Obsidian task filename", id)
	}
	data, err := root.ReadFile(obsidianNoteName(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return taskNote{}, fmt.Errorf("task %s not found", id)
		}
		return taskNote{}, fmt.Errorf("read task %s: %w", id, err)
	}
	return parseNoteWithFilenamePolicy(data, id, requireFilenameID)
}

func parseNote(data []byte, expectedID string) (taskNote, error) {
	return parseNoteWithFilenamePolicy(data, expectedID, true)
}

func parseNoteWithFilenamePolicy(data []byte, expectedID string, requireFilenameID bool) (taskNote, error) {
	text := string(data)
	fm, body, err := splitFrontmatter(text, expectedID)
	if err != nil {
		return taskNote{}, err
	}
	var note taskNote
	if err := yaml.Unmarshal([]byte(fm), &note); err != nil {
		repaired := quotePlainScalarFrontmatter(fm)
		if repaired == fm {
			return taskNote{}, fmt.Errorf("frontmatter parse failed in %s.md: %w", expectedID, err)
		}
		if retryErr := yaml.Unmarshal([]byte(repaired), &note); retryErr != nil {
			return taskNote{}, fmt.Errorf("frontmatter parse failed in %s.md: %w", expectedID, err)
		}
	}
	if err := normalizeParsedNote(&note, expectedID, requireFilenameID); err != nil {
		return taskNote{}, err
	}
	note.Body, note.AccCriteriaSection, note.VerPlanSection = splitBody(body)
	return note, nil
}

func splitFrontmatter(text string, expectedID string) (string, string, error) {
	const opening = "---\n"
	if !strings.HasPrefix(text, opening) {
		return "", "", fmt.Errorf("%w: missing opening --- in %s", errInvalidFrontmatter, expectedID)
	}
	rest := text[len(opening):]
	lineStart := 0
	for lineStart <= len(rest) {
		lineEnd := strings.IndexByte(rest[lineStart:], '\n')
		if lineEnd < 0 {
			break
		}
		lineEnd += lineStart
		line := strings.TrimSpace(rest[lineStart:lineEnd])
		if line == "---" || line == "..." {
			return rest[:lineStart], rest[lineEnd+1:], nil
		}
		lineStart = lineEnd + 1
	}
	return "", "", fmt.Errorf("%w: missing closing --- in %s", errInvalidFrontmatter, expectedID)
}

func normalizeParsedNote(note *taskNote, expectedID string, requireFilenameID bool) error {
	note.ID = strings.TrimSpace(note.ID)
	note.Title = strings.TrimSpace(note.Title)
	if note.ID == "" {
		return fmt.Errorf("%w: 'id' is required in %s.md", errMissingRequired, expectedID)
	}
	if note.Title == "" {
		return fmt.Errorf("%w: 'title' is required in %s.md", errMissingRequired, expectedID)
	}
	if requireFilenameID && note.ID != expectedID {
		return fmt.Errorf("id mismatch in %s: frontmatter id=%s, filename id=%s", expectedID, note.ID, expectedID)
	}
	note.Status = normalizeFrontmatterEnum(note.Status)
	if !validStatus(note.Status) {
		return fmt.Errorf("invalid status in %s.md: %s", expectedID, note.Status)
	}
	note.Priority = normalizeFrontmatterEnum(note.Priority)
	if note.Priority != "" && !validPriority(note.Priority) {
		return fmt.Errorf("invalid priority in %s.md: %s", expectedID, note.Priority)
	}
	note.ModelLevel = normalizeFrontmatterEnum(note.ModelLevel)
	if note.ModelLevel != "" && !validModelLevel(note.ModelLevel) {
		return fmt.Errorf("invalid model_level in %s.md: %s", expectedID, note.ModelLevel)
	}
	note.Tags = normalizeTags(note.Tags)
	return nil
}

func normalizeFrontmatterEnum(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func validStatus(value string) bool {
	switch value {
	case "todo", "in_progress", "blocked", "done":
		return true
	default:
		return false
	}
}

func validPriority(value string) bool {
	switch value {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

func validModelLevel(value string) bool {
	switch value {
	case "low", "medium", "high", "very_high":
		return true
	default:
		return false
	}
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

func quotePlainScalarFrontmatter(fm string) string {
	lines := strings.Split(fm, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			value := strings.TrimSpace(trimmed[2:])
			if value != "" && !isQuotedYAMLScalar(value) {
				prefix := line[:strings.Index(line, "-")+2]
				lines[i] = prefix + " " + strconv.Quote(value)
				changed = true
			}
			continue
		}
		idx := strings.Index(line, ": ")
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		if !plainScalarFrontmatterKey(key) {
			continue
		}
		value := line[idx+2:]
		if !strings.Contains(value, ": ") || isQuotedYAMLScalar(value) {
			continue
		}
		lines[i] = key + ": " + strconv.Quote(value)
		changed = true
	}
	if !changed {
		return fm
	}
	return strings.Join(lines, "\n")
}

func isQuotedYAMLScalar(value string) bool {
	return strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'")
}

func plainScalarFrontmatterKey(key string) bool {
	switch key {
	case "id", "title", "status", "priority", "model_level", "task_type", "parent_id", "branch", "worktree_path":
		return true
	default:
		return false
	}
}

func splitBody(body string) (string, []string, []string) {
	body = strings.TrimLeft(body, "\n")
	if body == "" {
		return "", nil, nil
	}
	if !strings.HasPrefix(body, "## ") {
		idx := strings.Index(body, "\n## ")
		if idx < 0 {
			return strings.TrimSpace(body), nil, nil
		}
		pre := strings.TrimSpace(body[:idx])
		body = body[idx:]
		if pre != "" {
			return pre, nil, nil
		}
	}
	var mainBody, accSection, verSection string
	for body != "" {
		if !strings.HasPrefix(body, "## ") {
			break
		}
		nl := strings.IndexByte(body, '\n')
		var heading string
		if nl < 0 {
			heading = strings.TrimSpace(body[3:])
			body = ""
		} else {
			heading = strings.TrimSpace(body[3:nl])
			body = body[nl+1:]
		}
		nextIdx := strings.Index(body, "\n## ")
		var content string
		if nextIdx < 0 {
			content = body
			body = ""
		} else {
			content = body[:nextIdx]
			body = body[nextIdx+1:]
		}
		content = strings.TrimSpace(content)
		switch heading {
		case "Body":
			mainBody = content
		case "Acceptance Criteria":
			accSection = content
		case "Verification Plan":
			verSection = content
		}
	}
	return mainBody, parseBulletList(accSection), parseBulletList(verSection)
}

func parseBulletList(text string) []string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = line[2:]
		}
		if len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
			dotIdx := strings.Index(line, ". ")
			if dotIdx > 0 {
				line = line[dotIdx+2:]
			}
		}
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func (b *obsidianTaskBackend) writeNote(note taskNote) error {
	root, err := b.ensureRoot()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if !safeObsidianTaskID(note.ID) {
		return fmt.Errorf("task id %q is not safe for an Obsidian task filename", note.ID)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	if err := b.encodeYAML(&buf, note); err != nil {
		return err
	}
	buf.WriteString("---\n")
	if note.Body != "" {
		buf.WriteString("\n## Body\n\n")
		buf.WriteString(note.Body)
		buf.WriteString("\n")
	}
	if len(note.AccCriteriaSection) > 0 || len(note.AcceptanceCriteria) > 0 {
		criteria := note.AccCriteriaSection
		if len(criteria) == 0 {
			criteria = []string(note.AcceptanceCriteria)
		}
		buf.WriteString("\n## Acceptance Criteria\n")
		for _, c := range criteria {
			buf.WriteString("\n- " + c)
		}
		buf.WriteString("\n")
	} else if len(note.AcceptanceCriteria) > 0 {
		buf.WriteString("\n## Acceptance Criteria\n")
		for _, c := range note.AcceptanceCriteria {
			buf.WriteString("\n- " + c)
		}
		buf.WriteString("\n")
	}
	if len(note.VerPlanSection) > 0 || len(note.VerificationPlan) > 0 {
		plan := note.VerPlanSection
		if len(plan) == 0 {
			plan = []string(note.VerificationPlan)
		}
		buf.WriteString("\n## Verification Plan\n")
		for i, v := range plan {
			_, _ = fmt.Fprintf(&buf, "\n%d. %s", i+1, v)
		}
		buf.WriteString("\n")
	}
	tmpName := obsidianNoteName(note.ID) + ".tmp"
	// #nosec G306 -- obsidian markdown notes are intentionally world-readable
	if err := root.WriteFile(tmpName, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write task %s: %w", note.ID, err)
	}
	if err := root.Rename(tmpName, obsidianNoteName(note.ID)); err != nil {
		_ = root.Remove(tmpName)
		return fmt.Errorf("commit task %s: %w", note.ID, err)
	}
	return nil
}

func (b *obsidianTaskBackend) encodeYAML(buf *bytes.Buffer, note taskNote) error {
	data, err := yaml.Marshal(note)
	if err != nil {
		return fmt.Errorf("encode task %s frontmatter: %w", note.ID, err)
	}
	_, err = buf.Write(data)
	return err
}

func noteToTask(note taskNote) tasks.Task {
	createdAt, _ := time.Parse(time.RFC3339Nano, note.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, note.UpdatedAt)
	body := note.Body
	if body == "" && len(note.BodySection) > 0 {
		body = note.BodySection
	}
	ac := []string(note.AcceptanceCriteria)
	if len(ac) == 0 {
		ac = note.AccCriteriaSection
	}
	vp := []string(note.VerificationPlan)
	if len(vp) == 0 {
		vp = note.VerPlanSection
	}
	return tasks.Task{
		ID: note.ID, Title: note.Title, Status: note.Status,
		Priority: note.Priority, ModelLevel: note.ModelLevel,
		TaskType: note.TaskType, ParentID: note.ParentID,
		Tags: note.Tags, Branch: note.Branch,
		WorktreePath:       note.WorktreePath,
		AcceptanceCriteria: ac, VerificationPlan: vp,
		ProjectionSource: "obsidian_registry",
		CreatedAt:        createdAt, UpdatedAt: updatedAt,
		Body: body,
	}
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
