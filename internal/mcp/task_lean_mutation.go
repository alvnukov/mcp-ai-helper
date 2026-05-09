package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/lake"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

const activeTasksLeanPath = "MCPAIHelperProject/ActiveTasks.lean"

type leanMutationResult struct {
	Task         tasks.Task `json:"task"`
	Source       string     `json:"source"`
	Validation   string     `json:"validation"`
	ChangedFiles []string   `json:"changed_files,omitempty"`
}

type leanBatchMutationResult struct {
	Upserted   []tasks.Task `json:"upserted"`
	Closed     []tasks.Task `json:"closed"`
	Source     string       `json:"source"`
	Validation string       `json:"validation"`
}

func setTaskStatus(ctx context.Context, req tasks.StatusRequest, commands *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	if strings.TrimSpace(req.Status) == "" {
		return leanMutationResult{}, errors.New("status is required")
	}
	var changedFiles []string
	validationSummary := "lake serve task.transition"
	task, err := mutateLeanActiveTasks(ctx, req.RepoPath, commands, func(state *leanActiveTasksState) (tasks.Task, error) {
		return state.setStatus(req.ID, req.Status)
	}, func(projected tasks.Task) error {
		files, summary, err := validateLeanTaskTransitionWithServer(ctx, req.RepoPath, req, projected)
		if err != nil {
			return err
		}
		changedFiles = files
		if strings.TrimSpace(summary) != "" {
			validationSummary = summary
		}
		return nil
	})
	if err != nil {
		return leanMutationResult{}, err
	}
	if len(changedFiles) == 0 {
		changedFiles = []string{activeTasksLeanPath}
	}
	return leanMutationResult{Task: task, Source: "lean_registry", Validation: validationSummary + " + lake build", ChangedFiles: changedFiles}, nil
}

func upsertTask(ctx context.Context, req tasks.AddRequest, commands *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	task, err := mutateLeanActiveTasks(ctx, req.RepoPath, commands, func(state *leanActiveTasksState) (tasks.Task, error) {
		return state.upsert(req)
	}, nil)
	if err != nil {
		return leanMutationResult{}, err
	}
	return leanMutationResult{Task: task, Source: "lean_registry", Validation: "lake build"}, nil
}

func batchUpsertTasks(ctx context.Context, req tasks.BatchUpsertRequest, commands *command.Runner, _ *tasks.Store) (leanBatchMutationResult, error) {
	var out leanBatchMutationResult
	_, err := mutateLeanActiveTasks(ctx, req.RepoPath, commands, func(state *leanActiveTasksState) (tasks.Task, error) {
		seen := map[string]struct{}{}
		for _, item := range req.Tasks {
			item.RepoPath = req.RepoPath
			task, err := state.upsert(item)
			if err != nil {
				return tasks.Task{}, err
			}
			seen[task.ID] = struct{}{}
			out.Upserted = append(out.Upserted, task)
		}
		if req.CloseMissing {
			missingStatus := strings.TrimSpace(req.MissingStatus)
			if missingStatus == "" {
				missingStatus = "done"
			}
			activeStatuses := req.ActiveStatuses
			if len(activeStatuses) == 0 {
				activeStatuses = []string{"todo", "in_progress", "blocked"}
			}
			active := map[string]struct{}{}
			for _, status := range activeStatuses {
				active[status] = struct{}{}
			}
			for _, task := range state.tasks() {
				if _, ok := seen[task.ID]; ok {
					continue
				}
				if _, ok := active[task.Status]; !ok {
					continue
				}
				closed, err := state.setStatus(task.ID, missingStatus)
				if err != nil {
					return tasks.Task{}, err
				}
				out.Closed = append(out.Closed, closed)
			}
		}
		return tasks.Task{}, nil
	}, nil)
	if err != nil {
		return leanBatchMutationResult{}, err
	}
	out.Source = "lean_registry"
	out.Validation = "lake build"
	return out, nil
}

func deleteTask(ctx context.Context, req tasks.DeleteRequest, commands *command.Runner, _ *tasks.Store) (leanMutationResult, error) {
	task, err := mutateLeanActiveTasks(ctx, req.RepoPath, commands, func(state *leanActiveTasksState) (tasks.Task, error) {
		return state.delete(req.ID)
	}, nil)
	if err != nil {
		return leanMutationResult{}, err
	}
	return leanMutationResult{Task: task, Source: "lean_registry", Validation: "lake build"}, nil
}

func mutateLeanActiveTasks(ctx context.Context, repoPath string, commands *command.Runner, mutate func(*leanActiveTasksState) (tasks.Task, error), validate func(tasks.Task) error) (tasks.Task, error) {
	workspace, err := lake.ResolveWorkspace(repoPath)
	if err != nil {
		return tasks.Task{}, err
	}
	path := filepath.Join(workspace.Dir, activeTasksLeanPath)
	original, err := os.ReadFile(path)
	if err != nil {
		return tasks.Task{}, fmt.Errorf("read Lean task registry: %w", err)
	}
	state, err := newLeanActiveTasksState(string(original))
	if err != nil {
		return tasks.Task{}, err
	}
	beforeHash := sha256.Sum256(original)
	task, err := mutate(state)
	if err != nil {
		return tasks.Task{}, err
	}
	if validate != nil {
		if err := validate(task); err != nil {
			return tasks.Task{}, err
		}
	}
	updated := []byte(state.text)
	afterHash := sha256.Sum256(updated)
	if beforeHash == afterHash {
		return withProjectionSource(task, "lean_registry"), nil
	}
	if err := os.WriteFile(path, updated, 0o600); err != nil {
		return tasks.Task{}, fmt.Errorf("write Lean task registry: %w", err)
	}
	result, buildErr := lake.Build(ctx, repoPath, lake.CommandRunner{Commands: commands, TimeoutSeconds: 20})
	if buildErr != nil || result.ExitCode != 0 {
		_ = os.WriteFile(path, original, 0o600)
		if buildErr != nil {
			return tasks.Task{}, fmt.Errorf("validate Lean task registry: %w", buildErr)
		}
		diagnostic := strings.TrimSpace(strings.Join(result.Diagnostics, "\n"))
		if diagnostic == "" {
			diagnostic = "lake build failed"
		}
		return tasks.Task{}, fmt.Errorf("validate Lean task registry: %s", diagnostic)
	}
	return withProjectionSource(task, "lean_registry"), nil
}

type leanRegistryDiagnostic struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Field    string `json:"field,omitempty"`
	TaskID   string `json:"task_id,omitempty"`
}

type leanRegistryValidation struct {
	Checked bool   `json:"checked"`
	Summary string `json:"summary"`
}

type leanRegistryEnvelope struct {
	SchemaVersion int                      `json:"schema_version"`
	OK            bool                     `json:"ok"`
	Operation     string                   `json:"operation"`
	Data          json.RawMessage          `json:"data"`
	Diagnostics   []leanRegistryDiagnostic `json:"diagnostics"`
	ChangedFiles  []string                 `json:"changed_files"`
	Validation    leanRegistryValidation   `json:"validation"`
}

type leanTaskTransitionPayload struct {
	Task leanTaskProjection `json:"task"`
}

func validateLeanTaskTransitionWithServer(ctx context.Context, repoPath string, req tasks.StatusRequest, projected tasks.Task) ([]string, string, error) {
	result, err := lake.CallServerRPC(ctx, repoPath, lake.RPCRequest{
		SourceFile:     "MCPAIHelperProject/TaskRegistryExport.lean",
		Method:         "MCPAIHelperProject.TaskRegistryExport.taskTransition",
		Params:         map[string]string{"id": req.ID, "to": req.Status},
		TimeoutSeconds: 20,
	})
	if err != nil {
		return nil, "", err
	}
	if result.Blocker != "" {
		return nil, "", fmt.Errorf("Lean task transition server blocker: %s", result.Blocker)
	}
	if len(result.Result) == 0 {
		return nil, "", errors.New("Lean task transition server returned no result")
	}
	var envelope leanRegistryEnvelope
	if err := json.Unmarshal(result.Result, &envelope); err != nil {
		return nil, "", fmt.Errorf("decode Lean task transition envelope: %w", err)
	}
	if envelope.SchemaVersion != 1 {
		return nil, "", fmt.Errorf("unsupported Lean task transition schema_version: %d", envelope.SchemaVersion)
	}
	if envelope.Operation != "task.transition" {
		return nil, "", fmt.Errorf("unexpected Lean task transition operation: %q", envelope.Operation)
	}
	if !envelope.OK {
		return nil, "", fmt.Errorf("Lean task transition rejected: %s", leanRegistryDiagnosticsMessage(envelope.Diagnostics))
	}
	if !envelope.Validation.Checked {
		return nil, "", errors.New("Lean task transition envelope did not report checked validation")
	}
	if !changedFilesContainLeanPath(envelope.ChangedFiles, activeTasksLeanPath) {
		return nil, "", fmt.Errorf("Lean task transition did not authorize changed file %s", activeTasksLeanPath)
	}
	var payload leanTaskTransitionPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		return nil, "", fmt.Errorf("decode Lean task transition payload: %w", err)
	}
	serverTask, err := payload.Task.toTask()
	if err != nil {
		return nil, "", err
	}
	if serverTask.ID != projected.ID || serverTask.Status != projected.Status {
		return nil, "", fmt.Errorf("Lean task transition mismatch: server=%s/%s projected=%s/%s", serverTask.ID, serverTask.Status, projected.ID, projected.Status)
	}
	return append([]string(nil), envelope.ChangedFiles...), envelope.Validation.Summary, nil
}

func leanRegistryDiagnosticsMessage(diagnostics []leanRegistryDiagnostic) string {
	if len(diagnostics) == 0 {
		return "no diagnostics"
	}
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		code := strings.TrimSpace(diagnostic.Code)
		message := strings.TrimSpace(diagnostic.Message)
		switch {
		case code != "" && message != "":
			parts = append(parts, code+": "+message)
		case message != "":
			parts = append(parts, message)
		case code != "":
			parts = append(parts, code)
		}
	}
	if len(parts) == 0 {
		return "empty diagnostics"
	}
	return strings.Join(parts, "; ")
}

func changedFilesContainLeanPath(files []string, path string) bool {
	want := filepath.ToSlash(filepath.Clean(path))
	for _, file := range files {
		if filepath.ToSlash(filepath.Clean(file)) == want {
			return true
		}
	}
	return false
}

type leanActiveTasksState struct {
	text string
}

func newLeanActiveTasksState(text string) (*leanActiveTasksState, error) {
	if strings.Contains(text, "<<<<<<<") || strings.Contains(text, ">>>>>>>") || strings.Contains(text, "=======") {
		return nil, errors.New("Lean task registry has conflict markers")
	}
	return &leanActiveTasksState{text: text}, nil
}

func (s *leanActiveTasksState) setStatus(id string, status string) (tasks.Task, error) {
	decl, err := leanTaskDecl(id)
	if err != nil {
		return tasks.Task{}, err
	}
	leanStatus, err := leanLifecycleStatus(status)
	if err != nil {
		return tasks.Task{}, err
	}
	block, err := s.artifactBlock(decl, id)
	if err != nil {
		return tasks.Task{}, err
	}
	statusRe := regexp.MustCompile(`status := \.[A-Za-z]+,`)
	matches := statusRe.FindAllStringIndex(block, -1)
	if len(matches) != 1 {
		return tasks.Task{}, fmt.Errorf("task %s status field is not uniquely editable", id)
	}
	updatedBlock := block[:matches[0][0]] + "status := " + leanStatus + "," + block[matches[0][1]:]
	s.text = strings.Replace(s.text, block, updatedBlock, 1)
	return s.taskByID(id)
}

func (s *leanActiveTasksState) upsert(req tasks.AddRequest) (tasks.Task, error) {
	if strings.TrimSpace(req.Title) == "" {
		return tasks.Task{}, errors.New("title is required")
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return tasks.Task{}, errors.New("task id is required for Lean task upsert")
	}
	decl, err := leanTaskDecl(id)
	if err != nil {
		return tasks.Task{}, err
	}
	if req.Status == "" {
		req.Status = "todo"
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	if existing, err := s.taskByID(id); err == nil && !existing.CreatedAt.IsZero() {
		createdAt = existing.CreatedAt.Format(time.RFC3339Nano)
	}
	task := tasks.Task{ID: id, Status: req.Status, Title: req.Title, Body: req.Body, Priority: req.Priority, Tags: uniqueStrings(req.Tags), CreatedAt: parseTaskTimeOrZero(createdAt), UpdatedAt: time.Now().UTC(), ProjectionSource: "lean_registry"}
	block, err := renderLeanTaskBlock(decl, task)
	if err != nil {
		return tasks.Task{}, err
	}
	if old, err := s.fullTaskBlock(decl, id); err == nil {
		s.text = strings.Replace(s.text, old, block, 1)
		return task, nil
	}
	marker := "def activeArtifacts : List Artifact :="
	idx := strings.Index(s.text, marker)
	if idx < 0 {
		return tasks.Task{}, errors.New("activeArtifacts declaration not found")
	}
	s.text = s.text[:idx] + block + s.text[idx:]
	listRe := regexp.MustCompile(`def activeArtifacts : List Artifact :=\n  \[([^\]]*)\]`)
	match := listRe.FindStringSubmatchIndex(s.text)
	if match == nil {
		return tasks.Task{}, errors.New("activeArtifacts list is not in expected single-line layout")
	}
	artifactName := decl + "Artifact"
	items := strings.TrimSpace(s.text[match[2]:match[3]])
	if strings.Contains(items, artifactName) {
		return task, nil
	}
	if items == "" {
		items = artifactName
	} else {
		items += ", " + artifactName
	}
	s.text = s.text[:match[2]] + items + s.text[match[3]:]
	return task, nil
}

func (s *leanActiveTasksState) delete(id string) (tasks.Task, error) {
	decl, err := leanTaskDecl(id)
	if err != nil {
		return tasks.Task{}, err
	}
	task, err := s.taskByID(id)
	if err != nil {
		return tasks.Task{}, err
	}
	block, err := s.fullTaskBlock(decl, id)
	if err != nil {
		return tasks.Task{}, err
	}
	s.text = strings.Replace(s.text, block, "", 1)
	listRe := regexp.MustCompile(`def activeArtifacts : List Artifact :=
  \[([^\]]*)\]`)
	match := listRe.FindStringSubmatchIndex(s.text)
	if match == nil {
		return tasks.Task{}, errors.New("activeArtifacts list is not in expected single-line layout")
	}
	artifactName := decl + "Artifact"
	var kept []string
	for _, item := range strings.Split(s.text[match[2]:match[3]], ",") {
		item = strings.TrimSpace(item)
		if item == "" || item == artifactName {
			continue
		}
		kept = append(kept, item)
	}
	s.text = s.text[:match[2]] + strings.Join(kept, ", ") + s.text[match[3]:]
	return task, nil
}

func (s *leanActiveTasksState) fullTaskBlock(decl string, id string) (string, error) {
	if count := strings.Count(s.text, `value := "`+id+`"`); count != 1 {
		return "", fmt.Errorf("task %s id occurrence count = %d, want 1", id, count)
	}
	start := strings.Index(s.text, "def "+decl+"Id : ArtifactId :=")
	if start < 0 {
		return "", fmt.Errorf("task %s declaration not found", id)
	}
	artifactStart := strings.Index(s.text[start:], "def "+decl+"Artifact : Artifact :=")
	if artifactStart < 0 {
		return "", fmt.Errorf("task %s artifact declaration not found", id)
	}
	searchFrom := start + artifactStart + len("def "+decl+"Artifact : Artifact :=")
	nextTaskRe := regexp.MustCompile(`

def task[0-9]+Id : ArtifactId :=|

def activeArtifacts : List Artifact :=`)
	match := nextTaskRe.FindStringIndex(s.text[searchFrom:])
	if match == nil {
		return "", fmt.Errorf("task %s declaration end not found", id)
	}
	return s.text[start : searchFrom+match[0]+2], nil
}

func (s *leanActiveTasksState) artifactBlock(decl string, id string) (string, error) {
	full, err := s.fullTaskBlock(decl, id)
	if err != nil {
		return "", err
	}
	start := strings.Index(full, "def "+decl+"Artifact : Artifact :=")
	if start < 0 {
		return "", fmt.Errorf("task %s artifact declaration not found", id)
	}
	return full[start:], nil
}

func (s *leanActiveTasksState) taskByID(id string) (tasks.Task, error) {
	payload, err := leanProjectionFromText(s.text, id)
	if err != nil {
		return tasks.Task{}, err
	}
	return payload.toTask()
}

func (s *leanActiveTasksState) tasks() []tasks.Task {
	ids := regexp.MustCompile(`value := "(task-[^"]+)"`).FindAllStringSubmatch(s.text, -1)
	out := make([]tasks.Task, 0, len(ids))
	for _, match := range ids {
		task, err := s.taskByID(match[1])
		if err == nil {
			out = append(out, task)
		}
	}
	return out
}

func leanProjectionFromText(text string, id string) (leanTaskProjection, error) {
	decl, err := leanTaskDecl(id)
	if err != nil {
		return leanTaskProjection{}, err
	}
	state, err := newLeanActiveTasksState(text)
	if err != nil {
		return leanTaskProjection{}, err
	}
	full, err := state.fullTaskBlock(decl, id)
	if err != nil {
		return leanTaskProjection{}, err
	}
	return leanTaskProjection{ID: id, Status: goStatusFromLean(mustFindLeanField(full, `status := \.(\w+),`)), Title: mustFindLeanStringField(full, "title"), Body: mustFindLeanStringField(full, "body"), Priority: goPriorityFromLean(mustFindLeanField(full, `priority := \.(\w+),`)), Tags: mustFindLeanStringList(full, decl+"Tags"), CreatedAt: mustFindLeanStringField(full, "createdAt"), UpdatedAt: mustFindLeanStringField(full, "updatedAt")}, nil
}

func renderLeanTaskBlock(decl string, task tasks.Task) (string, error) {
	status, err := leanLifecycleStatus(task.Status)
	if err != nil {
		return "", err
	}
	priority, err := leanPriority(task.Priority)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("def %sId : ArtifactId :=\n  { value := %s }\n\ndef %sTags : List String :=\n  %s\n\ndef %sArtifact : Artifact :=\n  { id := %sId,\n    kind := .task,\n    status := %s,\n    priority := %s,\n    title := %s,\n    body := %s,\n    tags := %sTags,\n    createdAt := %s,\n    updatedAt := %s }\n\n", decl, leanQuote(task.ID), decl, leanStringList(task.Tags), decl, decl, status, priority, leanQuote(task.Title), leanQuote(task.Body), decl, leanQuote(task.CreatedAt.Format(time.RFC3339Nano)), leanQuote(task.UpdatedAt.Format(time.RFC3339Nano))), nil
}

func leanTaskDecl(id string) (string, error) {
	if !regexp.MustCompile(`^task-[0-9]+$`).MatchString(id) {
		return "", fmt.Errorf("Lean task mutation supports canonical task-NNN ids only: %s", id)
	}
	return "task" + strings.TrimPrefix(id, "task-"), nil
}

func leanLifecycleStatus(status string) (string, error) {
	switch strings.TrimSpace(status) {
	case "todo":
		return ".proposed", nil
	case "in_progress":
		return ".active", nil
	case "blocked":
		return ".blocked", nil
	case "done":
		return ".verified", nil
	default:
		return "", fmt.Errorf("unsupported task status %q", status)
	}
}

func leanPriority(priority string) (string, error) {
	switch strings.TrimSpace(priority) {
	case "", "medium", "normal":
		return ".normal", nil
	case "low":
		return ".low", nil
	case "high":
		return ".high", nil
	case "critical":
		return ".critical", nil
	default:
		return "", fmt.Errorf("unsupported task priority %q", priority)
	}
}

func goStatusFromLean(status string) string {
	switch status {
	case "proposed":
		return "todo"
	case "active":
		return "in_progress"
	case "blocked":
		return "blocked"
	default:
		return "done"
	}
}

func goPriorityFromLean(priority string) string {
	if priority == "normal" {
		return "medium"
	}
	return priority
}

func leanQuote(value string) string {
	quoted := fmt.Sprintf("%q", value)
	return quoted
}

func leanStringList(values []string) string {
	cleaned := uniqueStrings(values)
	if len(cleaned) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(cleaned))
	for _, value := range cleaned {
		parts = append(parts, leanQuote(value))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func mustFindLeanField(text string, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func mustFindLeanStringField(text string, field string) string {
	match := regexp.MustCompile(field + ` := "((?:\\.|[^"])*)"`).FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return unescapeLeanString(match[1])
}

func mustFindLeanStringList(text string, decl string) []string {
	match := regexp.MustCompile(`def ` + regexp.QuoteMeta(decl) + ` : List String :=\n  \[([^\]]*)\]`).FindStringSubmatch(text)
	if len(match) != 2 || strings.TrimSpace(match[1]) == "" {
		return nil
	}
	itemRe := regexp.MustCompile(`"((?:\\.|[^"])*)"`)
	matches := itemRe.FindAllStringSubmatch(match[1], -1)
	out := make([]string, 0, len(matches))
	for _, item := range matches {
		out = append(out, unescapeLeanString(item[1]))
	}
	return out
}

func unescapeLeanString(value string) string {
	unquoted := "\"" + value + "\""
	var out string
	if _, err := fmt.Sscanf(unquoted, "%q", &out); err != nil {
		return value
	}
	return out
}

func parseTaskTimeOrZero(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func withProjectionSource(task tasks.Task, source string) tasks.Task {
	task.ProjectionSource = source
	return task
}
