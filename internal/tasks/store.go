package tasks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var taskIDPattern = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
var branchTypePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var ErrLegacyTaskStoreDisabled = errors.New("legacy task file store is disabled; use the canonical Lean/Lake task registry")

// Store is retained only as a dependency placeholder for older internal wiring.
// It intentionally performs no task persistence; task state is Lean/Lake-owned.
type Store struct{}

type Task struct {
	ID                 string    `json:"id"`
	TaskType           string    `json:"task_type,omitempty"`
	Branch             string    `json:"branch,omitempty"`
	WorktreePath       string    `json:"worktree_path,omitempty"`
	CodePath           string    `json:"code_path,omitempty"`
	WorktreeExists     bool      `json:"worktree_exists,omitempty"`
	ParentID           string    `json:"parent_id,omitempty"`
	Status             string    `json:"status"`
	Title              string    `json:"title"`
	Body               string    `json:"body"`
	Priority           string    `json:"priority,omitempty"`
	ModelLevel         string    `json:"model_level"`
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
	TaskType           string   `json:"task_type"`
	Branch             string   `json:"branch"`
	WorktreePath       string   `json:"worktree_path"`
	ParentID           string   `json:"parent_id,omitempty"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Priority           string   `json:"priority"`
	ModelLevel         string   `json:"model_level"`
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
	TaskType           string   `json:"task_type"`
	Branch             string   `json:"branch"`
	WorktreePath       string   `json:"worktree_path"`
	ParentID           string   `json:"parent_id,omitempty"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Priority           string   `json:"priority"`
	ModelLevel         string   `json:"model_level"`
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
	Upserted   []Task `json:"upserted"`
	Closed     []Task `json:"closed"`
	Source     string `json:"source,omitempty"`
	Validation string `json:"validation,omitempty"`
}

func NewStore(_ any) *Store { return &Store{} }

func (s *Store) Add(AddRequest) (Task, error)          { return Task{}, ErrLegacyTaskStoreDisabled }
func (s *Store) Update(UpdateRequest) (Task, error)    { return Task{}, ErrLegacyTaskStoreDisabled }
func (s *Store) SetStatus(StatusRequest) (Task, error) { return Task{}, ErrLegacyTaskStoreDisabled }
func (s *Store) BatchUpsert(BatchUpsertRequest) (BatchUpsertResult, error) {
	return BatchUpsertResult{}, ErrLegacyTaskStoreDisabled
}
func (s *Store) List(ListRequest) ([]Task, error) { return nil, ErrLegacyTaskStoreDisabled }
func (s *Store) Get(GetRequest) (Task, error)     { return Task{}, ErrLegacyTaskStoreDisabled }
func (s *Store) Delete(DeleteRequest) error       { return ErrLegacyTaskStoreDisabled }

func WorktreePathForID(id string) string {
	id = cleanTaskID(id)
	if id == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join(".worktrees", id))
}

func BranchForTask(taskType string, taskID string) (string, error) {
	taskType = cleanTaskType(taskType)
	taskID = cleanTaskID(taskID)
	if taskType == "" {
		return "", errors.New("task_type is required for task worktree branch")
	}
	if taskID == "" {
		return "", errors.New("task id is required for task worktree branch")
	}
	if !branchTypePattern.MatchString(taskType) {
		return "", fmt.Errorf("task_type %q is not a valid branch type", taskType)
	}
	return taskType + "/" + taskID, nil
}

func NormalizeWorktreeFields(task *Task) error {
	if task == nil {
		return errors.New("task is nil")
	}
	task.ID = cleanTaskID(task.ID)
	if task.ID == "" {
		return errors.New("task id is required")
	}
	task.TaskType = cleanTaskType(task.TaskType)
	wantWorktree := WorktreePathForID(task.ID)
	worktreePath := filepath.ToSlash(filepath.Clean(strings.TrimSpace(task.WorktreePath)))
	if worktreePath == "" || worktreePath == "." {
		worktreePath = wantWorktree
	}
	if worktreePath != wantWorktree {
		return fmt.Errorf("worktree_path must be %s for task %s, got %q", wantWorktree, task.ID, worktreePath)
	}
	task.WorktreePath = worktreePath
	branch := filepath.ToSlash(strings.TrimSpace(task.Branch))
	if task.TaskType == "" {
		if branch == "" {
			task.Branch = ""
			return nil
		}
		parts := strings.Split(branch, "/")
		if len(parts) != 2 || parts[1] != task.ID || !branchTypePattern.MatchString(parts[0]) {
			return fmt.Errorf("branch must be <task_type>/%s, got %q", task.ID, branch)
		}
		task.TaskType = parts[0]
		task.Branch = branch
		return nil
	}
	wantBranch, err := BranchForTask(task.TaskType, task.ID)
	if err != nil {
		return err
	}
	if branch == "" {
		task.Branch = wantBranch
		return nil
	}
	if branch != wantBranch {
		return fmt.Errorf("branch must be %s for task %s, got %q", wantBranch, task.ID, branch)
	}
	task.Branch = branch
	return nil
}

func WithWorktreeContext(repoPath string, task Task) Task {
	if task.ID == "" {
		return task
	}
	_ = NormalizeWorktreeFields(&task)
	if task.WorktreePath == "" {
		task.WorktreePath = WorktreePathForID(task.ID)
	}
	if strings.TrimSpace(repoPath) == "" {
		return task
	}
	repoAbs, err := filepath.Abs(repoPath)
	if err != nil {
		return task
	}
	codePath := filepath.Join(repoAbs, filepath.FromSlash(task.WorktreePath))
	task.CodePath = codePath
	if info, err := os.Stat(codePath); err == nil && info.IsDir() {
		task.WorktreeExists = true
	} else {
		task.WorktreeExists = false
	}
	return task
}

func NormalizeModelLevel(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.Join(strings.Fields(value), "_")
	switch value {
	case "":
		return "", nil
	case "low", "medium", "high", "very_high":
		return value, nil
	case "veryhigh":
		return "very_high", nil
	default:
		return "", fmt.Errorf("unsupported task model_level %q", value)
	}
}

func cleanTaskID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = taskIDPattern.ReplaceAllString(value, "-")
	return strings.Trim(value, ".-")
}

func cleanTaskType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = taskIDPattern.ReplaceAllString(value, "-")
	return strings.Trim(value, ".-")
}
