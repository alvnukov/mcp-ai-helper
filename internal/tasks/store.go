package tasks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/project"
)

const leanTaskPrefix = "/- mcp-ai-helper-task "

var taskIDPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type Store struct {
	projects *project.Store
}

type Task struct {
	ID                 string    `json:"id"`
	ParentID           string    `json:"parent_id,omitempty"`
	Status             string    `json:"status"`
	Title              string    `json:"title"`
	Body               string    `json:"body"`
	Priority           string    `json:"priority,omitempty"`
	Tags               []string  `json:"tags,omitempty"`
	AcceptanceCriteria []string  `json:"acceptance_criteria,omitempty"`
	VerificationPlan   []string  `json:"verification_plan,omitempty"`
	ProjectionSource   string    `json:"projection_source,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AddRequest struct {
	RepoPath           string   `json:"repo_path"`
	ID                 string   `json:"id"`
	ParentID           string   `json:"parent_id,omitempty"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Priority           string   `json:"priority"`
	Tags               []string `json:"tags"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	VerificationPlan   []string `json:"verification_plan"`
}

type ListRequest struct {
	RepoPath string `json:"repo_path"`
	Status   string `json:"status"`
	Query    string `json:"query"`
}

type GetRequest struct {
	RepoPath string `json:"repo_path"`
	ID       string `json:"id"`
}

type DeleteRequest struct {
	RepoPath string `json:"repo_path"`
	ID       string `json:"id"`
}

type UpdateRequest struct {
	RepoPath           string   `json:"repo_path"`
	ID                 string   `json:"id"`
	ParentID           string   `json:"parent_id,omitempty"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Priority           string   `json:"priority"`
	Tags               []string `json:"tags"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	VerificationPlan   []string `json:"verification_plan"`
}

type StatusRequest struct {
	RepoPath string `json:"repo_path"`
	ID       string `json:"id"`
	Status   string `json:"status"`
}

type BatchUpsertRequest struct {
	RepoPath       string       `json:"repo_path"`
	Tasks          []AddRequest `json:"tasks"`
	CloseMissing   bool         `json:"close_missing"`
	MissingStatus  string       `json:"missing_status"`
	ActiveStatuses []string     `json:"active_statuses"`
}

type BatchUpsertResult struct {
	Upserted []Task `json:"upserted"`
	Closed   []Task `json:"closed"`
}

func NewStore(projects *project.Store) *Store {
	return &Store{projects: projects}
}

func (s *Store) Add(req AddRequest) (Task, error) {
	if strings.TrimSpace(req.Title) == "" {
		return Task{}, errors.New("title is required")
	}
	now := time.Now().UTC()
	id := cleanTaskID(req.ID)
	if id == "" {
		id = cleanTaskID(req.Title)
	}
	if id == "" {
		return Task{}, errors.New("task id is empty after normalization")
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "todo"
	}
	task := Task{
		ID:                 id,
		ParentID:           req.ParentID,
		Status:             status,
		Title:              req.Title,
		Body:               req.Body,
		Priority:           strings.TrimSpace(req.Priority),
		Tags:               cleanTags(req.Tags),
		AcceptanceCriteria: cleanLines(req.AcceptanceCriteria),
		VerificationPlan:   cleanLines(req.VerificationPlan),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	path, err := s.taskPath(req.RepoPath, id)
	if err != nil {
		return Task{}, err
	}
	if existing, err := s.readTask(path); err == nil {
		task.CreatedAt = existing.CreatedAt
	}
	if err := s.writeTaskPath(path, task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) Update(req UpdateRequest) (Task, error) {
	task, err := s.Get(GetRequest{RepoPath: req.RepoPath, ID: req.ID})
	if err != nil {
		return Task{}, err
	}
	if strings.TrimSpace(req.Status) != "" {
		task.Status = strings.TrimSpace(req.Status)
	}
	if req.ParentID != "" {
		task.ParentID = req.ParentID
	}
	if strings.TrimSpace(req.Title) != "" {
		task.Title = req.Title
	}
	if req.Body != "" {
		task.Body = req.Body
	}
	if strings.TrimSpace(req.Priority) != "" {
		task.Priority = strings.TrimSpace(req.Priority)
	}
	if req.Tags != nil {
		task.Tags = cleanTags(req.Tags)
	}
	if req.AcceptanceCriteria != nil {
		task.AcceptanceCriteria = cleanLines(req.AcceptanceCriteria)
	}
	if req.VerificationPlan != nil {
		task.VerificationPlan = cleanLines(req.VerificationPlan)
	}
	task.UpdatedAt = time.Now().UTC()
	if err := s.writeTask(req.RepoPath, task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) SetStatus(req StatusRequest) (Task, error) {
	if strings.TrimSpace(req.Status) == "" {
		return Task{}, errors.New("status is required")
	}
	return s.Update(UpdateRequest{RepoPath: req.RepoPath, ID: req.ID, Status: req.Status})
}

func (s *Store) BatchUpsert(req BatchUpsertRequest) (BatchUpsertResult, error) {
	seen := map[string]struct{}{}
	result := BatchUpsertResult{}
	for _, item := range req.Tasks {
		item.RepoPath = req.RepoPath
		task, err := s.Add(item)
		if err != nil {
			return BatchUpsertResult{}, err
		}
		seen[task.ID] = struct{}{}
		result.Upserted = append(result.Upserted, task)
	}
	if !req.CloseMissing {
		return result, nil
	}
	missingStatus := strings.TrimSpace(req.MissingStatus)
	if missingStatus == "" {
		missingStatus = "done"
	}
	activeStatuses := req.ActiveStatuses
	if len(activeStatuses) == 0 {
		activeStatuses = []string{"todo", "in_progress", "blocked"}
	}
	active := statusSet(activeStatuses)
	existing, err := s.List(ListRequest{RepoPath: req.RepoPath})
	if err != nil {
		return BatchUpsertResult{}, err
	}
	for _, task := range existing {
		if _, ok := seen[task.ID]; ok || !active[task.Status] {
			continue
		}
		closed, err := s.SetStatus(StatusRequest{RepoPath: req.RepoPath, ID: task.ID, Status: missingStatus})
		if err != nil {
			return BatchUpsertResult{}, err
		}
		result.Closed = append(result.Closed, closed)
	}
	return result, nil
}

func (s *Store) List(req ListRequest) ([]Task, error) {
	dir, err := s.projects.TasksDir(req.RepoPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read task directory: %w", err)
	}
	seenIDs := map[string]struct{}{}
	var out []Task
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".lean") {
			continue
		}
		id := strings.TrimSuffix(name, ".lean")
		id = cleanTaskID(id)
		if id == "" {
			continue
		}
		if _, ok := seenIDs[id]; ok {
			continue
		}
		task, err := s.readTask(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		seenIDs[id] = struct{}{}
		if req.Status != "" && task.Status != req.Status {
			continue
		}
		if req.Query != "" && !taskMatches(task, req.Query) {
			continue
		}
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) Get(req GetRequest) (Task, error) {
	path, err := s.taskPath(req.RepoPath, req.ID)
	if err != nil {
		return Task{}, err
	}
	return s.readTask(path)
}

func (s *Store) Delete(req DeleteRequest) error {
	path, err := s.taskPath(req.RepoPath, req.ID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete task file: %w", err)
	}
	return nil
}

func (s *Store) writeTask(repoPath string, task Task) error {
	path, err := s.taskPath(repoPath, task.ID)
	if err != nil {
		return err
	}
	return s.writeTaskPath(path, task)
}

func (s *Store) writeTaskPath(path string, task Task) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}
	data, err := encodeLeanTask(task)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}
	return nil
}

func (s *Store) taskPath(repoPath string, id string) (string, error) {
	id = cleanTaskID(id)
	if id == "" {
		return "", errors.New("task id is required")
	}
	dir, err := s.projects.TasksDir(repoPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".lean"), nil
}

func (s *Store) readTask(path string) (Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Task{}, fmt.Errorf("read task file: %w", err)
	}
	return readLeanContent(data)
}

func readLeanContent(data []byte) (Task, error) {
	text := string(data)
	start := strings.Index(text, leanTaskPrefix)
	if start < 0 {
		return Task{}, errors.New("task metadata is missing")
	}
	start += len(leanTaskPrefix)
	end := strings.Index(text[start:], " -/")
	if end < 0 {
		return Task{}, errors.New("task metadata is unterminated")
	}
	var task Task
	if err := json.Unmarshal([]byte(text[start:start+end]), &task); err != nil {
		return Task{}, fmt.Errorf("decode task metadata: %w", err)
	}
	return task, nil
}

func encodeLeanTask(task Task) ([]byte, error) {
	metadata, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal task metadata: %w", err)
	}
	ident := leanTaskIdent(task.ID)
	body, err := json.Marshal(task.Body)
	if err != nil {
		return nil, fmt.Errorf("marshal task body: %w", err)
	}
	title, err := json.Marshal(task.Title)
	if err != nil {
		return nil, fmt.Errorf("marshal task title: %w", err)
	}
	status, err := json.Marshal(task.Status)
	if err != nil {
		return nil, fmt.Errorf("marshal task status: %w", err)
	}
	priority, err := json.Marshal(task.Priority)
	if err != nil {
		return nil, fmt.Errorf("marshal task priority: %w", err)
	}
	content := fmt.Sprintf("/- mcp-ai-helper-task %s -/\nnamespace MCPAIHelper.Tasks\n\ndef %s_id : String := %q\ndef %s_status : String := %s\ndef %s_title : String := %s\ndef %s_body : String := %s\ndef %s_priority : String := %s\n\nend MCPAIHelper.Tasks\n", metadata, ident, task.ID, ident, status, ident, title, ident, body, ident, priority)
	return []byte(content), nil
}

func leanTaskIdent(id string) string {
	id = cleanTaskID(id)
	if id == "" {
		return "task"
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "task"
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "task_" + out
	}
	return out
}

func cleanTaskID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = taskIDPattern.ReplaceAllString(value, "-")
	return strings.Trim(value, ".-")
}

func cleanTags(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = taskIDPattern.ReplaceAllString(value, "-")
		value = strings.Trim(value, ".-")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func cleanLines(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func statusSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func taskMatches(task Task, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		task.ID,
		task.Status,
		task.Title,
		task.Body,
		task.Priority,
		strings.Join(task.Tags, "\n"),
		strings.Join(task.AcceptanceCriteria, "\n"),
		strings.Join(task.VerificationPlan, "\n"),
	}, "\n"))
	return strings.Contains(haystack, query)
}
