package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type obsidianTaskBackend struct {
	dir string
}

func newObsidianTaskBackend(dir string) taskBackend {
	return &obsidianTaskBackend{dir: dir}
}

type taskNote struct {
	ID                 string   `yaml:"id"`
	Title              string   `yaml:"title"`
	Status             string   `yaml:"status"`
	Priority           string   `yaml:"priority,omitempty"`
	ModelLevel         string   `yaml:"model_level,omitempty"`
	TaskType           string   `yaml:"task_type,omitempty"`
	ParentID           string   `yaml:"parent_id,omitempty"`
	Tags               []string `yaml:"tags,omitempty"`
	Branch             string   `yaml:"branch,omitempty"`
	WorktreePath       string   `yaml:"worktree_path,omitempty"`
	AcceptanceCriteria []string `yaml:"acceptance_criteria,omitempty"`
	VerificationPlan   []string `yaml:"verification_plan,omitempty"`
	CreatedAt          string   `yaml:"created_at,omitempty"`
	UpdatedAt          string   `yaml:"updated_at,omitempty"`
	Body               string   `yaml:"-"`
	BodySection        string   `yaml:"-"`
	AccCriteriaSection []string `yaml:"-"`
	VerPlanSection     []string `yaml:"-"`
}

var errInvalidFrontmatter = errors.New("invalid frontmatter")
var errMissingRequired = errors.New("missing required field")

func (b *obsidianTaskBackend) notePath(id string) string {
	return filepath.Join(b.dir, id+".md")
}

func (b *obsidianTaskBackend) ListCurrent(_ context.Context, _ string) ([]tasks.Task, string, error) {
	all, err := b.readAll()
	if err != nil {
		return nil, "obsidian_registry", err
	}
	active := make([]tasks.Task, 0, len(all))
	for _, t := range all {
		if t.Status == "todo" || t.Status == "in_progress" {
			active = append(active, t)
		}
	}
	return active, "obsidian_registry", nil
}

func (b *obsidianTaskBackend) ListAll(_ context.Context, _ string) ([]tasks.Task, string, error) {
	all, err := b.readAll()
	if err != nil {
		return nil, "obsidian_registry", err
	}
	return all, "obsidian_registry", nil
}

func (b *obsidianTaskBackend) Get(_ context.Context, _ string, id string) (tasks.Task, string, error) {
	t, err := b.readOne(id)
	if err != nil {
		return tasks.Task{}, "obsidian_registry", err
	}
	return t, "obsidian_registry", nil
}

func (b *obsidianTaskBackend) Upsert(_ context.Context, req tasks.AddRequest) (taskMutationResult, error) {
	if strings.TrimSpace(req.Title) == "" {
		return taskMutationResult{}, errors.New("title is required")
	}
	id := req.ID
	if id == "" {
		id = tasks.WorktreePathForID(req.Title)
	}
	now := time.Now().UTC()
	existing, exists := b.tryRead(id)
	createdAt := now.Format(time.RFC3339Nano)
	if exists && !existing.CreatedAt.IsZero() {
		createdAt = existing.CreatedAt.Format(time.RFC3339Nano)
	}
	note := taskNote{
		ID: id, Title: req.Title, Status: req.Status,
		Priority: req.Priority, ModelLevel: req.ModelLevel,
		TaskType: req.TaskType, ParentID: req.ParentID,
		Tags: nonNilTags(req.Tags), Branch: req.Branch,
		WorktreePath: req.WorktreePath,
		AcceptanceCriteria: req.AcceptanceCriteria,
		VerificationPlan:   req.VerificationPlan,
		CreatedAt: createdAt, UpdatedAt: now.Format(time.RFC3339Nano),
		Body: req.Body,
	}
	if err := b.writeNote(note); err != nil {
		return taskMutationResult{}, err
	}
	task := noteToTask(note)
	return taskMutationResult{Task: task, Source: "obsidian_registry", Validation: "frontmatter parsed + file written", ChangedFiles: []string{id + ".md"}}, nil
}

func (b *obsidianTaskBackend) SetStatus(_ context.Context, req tasks.StatusRequest) (taskMutationResult, error) {
	if strings.TrimSpace(req.Status) == "" {
		return taskMutationResult{}, errors.New("status is required")
	}
	note, err := b.readNote(req.ID)
	if err != nil {
		return taskMutationResult{}, err
	}
	note.Status = req.Status
	note.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := b.writeNote(note); err != nil {
		return taskMutationResult{}, err
	}
	task := noteToTask(note)
	return taskMutationResult{Task: task, Source: "obsidian_registry", Validation: "frontmatter parsed + file written", ChangedFiles: []string{req.ID + ".md"}}, nil
}

func (b *obsidianTaskBackend) BatchUpsert(_ context.Context, req tasks.BatchUpsertRequest) (taskBatchMutationResult, error) {
	upserted := make([]tasks.Task, 0, len(req.Tasks))
	for _, item := range req.Tasks {
		result, err := b.Upsert(context.Background(), item)
		if err != nil {
			return taskBatchMutationResult{}, fmt.Errorf("batch upsert %s: %w", item.ID, err)
		}
		upserted = append(upserted, result.Task)
	}
	closed := make([]tasks.Task, 0)
	if req.CloseMissing {
		missingStatus := req.MissingStatus
		if missingStatus == "" {
			missingStatus = "done"
		}
		batchIDs := make(map[string]bool, len(req.Tasks))
		for _, item := range req.Tasks {
			batchIDs[item.ID] = true
		}
		activeStatuses := req.ActiveStatuses
		if len(activeStatuses) == 0 {
			activeStatuses = []string{"todo", "in_progress"}
		}
		all, _ := b.readAll()
		for _, t := range all {
			if batchIDs[t.ID] {
				continue
			}
			for _, s := range activeStatuses {
				if t.Status == s {
					note, err := b.readNote(t.ID)
					if err != nil {
						continue
					}
					note.Status = missingStatus
					note.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
					if err := b.writeNote(note); err != nil {
						continue
					}
					closed = append(closed, noteToTask(note))
					break
				}
			}
		}
	}
	return taskBatchMutationResult{Upserted: upserted, Closed: closed, Source: "obsidian_registry", Validation: "batch upsert complete"}, nil
}

func (b *obsidianTaskBackend) Delete(_ context.Context, req tasks.DeleteRequest) (taskMutationResult, error) {
	if strings.TrimSpace(req.ID) == "" {
		return taskMutationResult{}, errors.New("id is required")
	}
	note, err := b.readNote(req.ID)
	if err != nil {
		return taskMutationResult{}, err
	}
	path := b.notePath(req.ID)
	if err := os.Remove(path); err != nil {
		return taskMutationResult{}, fmt.Errorf("delete task %s: %w", req.ID, err)
	}
	task := noteToTask(note)
	return taskMutationResult{Task: task, Source: "obsidian_registry", Validation: "file deleted", ChangedFiles: []string{req.ID + ".md"}}, nil
}

func (b *obsidianTaskBackend) readAll() ([]tasks.Task, error) {
	entries, err := os.ReadDir(b.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read obsidian task dir: %w", err)
	}
	out := make([]tasks.Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		note, err := b.readNote(id)
		if err != nil {
			continue
		}
		out = append(out, noteToTask(note))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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
	path := b.notePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return taskNote{}, fmt.Errorf("task %s not found", id)
		}
		return taskNote{}, fmt.Errorf("read task %s: %w", id, err)
	}
	return parseNote(data, id)
}

func parseNote(data []byte, expectedID string) (taskNote, error) {
	text := string(data)
	const delim = "---\n"
	if !strings.HasPrefix(text, delim) {
		return taskNote{}, fmt.Errorf("%w: missing opening --- in %s", errInvalidFrontmatter, expectedID)
	}
	rest := text[len(delim):]
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return taskNote{}, fmt.Errorf("%w: missing closing --- in %s", errInvalidFrontmatter, expectedID)
	}
	fm := rest[:endIdx]
	body := rest[endIdx+4:]
	var note taskNote
	if err := yaml.Unmarshal([]byte(fm), &note); err != nil {
		return taskNote{}, fmt.Errorf("frontmatter parse failed in %s.md: %w", expectedID, err)
	}
	if strings.TrimSpace(note.ID) == "" {
		return taskNote{}, fmt.Errorf("%w: 'id' is required in %s.md", errMissingRequired, expectedID)
	}
	if strings.TrimSpace(note.Title) == "" {
		return taskNote{}, fmt.Errorf("%w: 'title' is required in %s.md", errMissingRequired, expectedID)
	}
	if strings.TrimSpace(note.Status) == "" {
		return taskNote{}, fmt.Errorf("%w: 'status' is required in %s.md", errMissingRequired, expectedID)
	}
	if note.ID != expectedID {
		return taskNote{}, fmt.Errorf("id mismatch in %s: frontmatter id=%s, filename id=%s", expectedID, note.ID, expectedID)
	}
	note.Body, note.AccCriteriaSection, note.VerPlanSection = splitBody(body)
	return note, nil
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
		heading := strings.TrimSpace(body[3:])
		if nl < 0 {
			body = ""
		} else {
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
	var buf bytes.Buffer
	buf.WriteString("---\n")
	b.encodeYAML(&buf, note)
	buf.WriteString("---\n")
	if note.Body != "" {
		buf.WriteString("\n## Body\n\n")
		buf.WriteString(note.Body)
		buf.WriteString("\n")
	}
	if len(note.AccCriteriaSection) > 0 || len(note.AcceptanceCriteria) > 0 {
		criteria := note.AccCriteriaSection
		if len(criteria) == 0 {
			criteria = note.AcceptanceCriteria
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
			plan = note.VerificationPlan
		}
		buf.WriteString("\n## Verification Plan\n")
		for i, v := range plan {
			buf.WriteString(fmt.Sprintf("\n%d. %s", i+1, v))
		}
		buf.WriteString("\n")
	} else if len(note.VerificationPlan) > 0 {
		buf.WriteString("\n## Verification Plan\n")
		for i, v := range note.VerificationPlan {
			buf.WriteString(fmt.Sprintf("\n%d. %s", i+1, v))
		}
		buf.WriteString("\n")
	}
	tmpPath := b.notePath(note.ID) + ".tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write task %s: %w", note.ID, err)
	}
	if err := os.Rename(tmpPath, b.notePath(note.ID)); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("commit task %s: %w", note.ID, err)
	}
	return nil
}

func (b *obsidianTaskBackend) encodeYAML(buf *bytes.Buffer, note taskNote) {
	encode := func(key, value string) {
		if value != "" {
			buf.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}
	encodeArr := func(key string, values []string) {
		if len(values) > 0 {
			buf.WriteString(fmt.Sprintf("%s:\n", key))
			for _, v := range values {
				buf.WriteString(fmt.Sprintf("  - %s\n", v))
			}
		}
	}
	encode("id", note.ID)
	encode("title", note.Title)
	encode("status", note.Status)
	encode("priority", note.Priority)
	encode("model_level", note.ModelLevel)
	encode("task_type", note.TaskType)
	encode("parent_id", note.ParentID)
	encodeArr("tags", note.Tags)
	encode("branch", note.Branch)
	encode("worktree_path", note.WorktreePath)
	encodeArr("acceptance_criteria", note.AcceptanceCriteria)
	encodeArr("verification_plan", note.VerificationPlan)
	encode("created_at", note.CreatedAt)
	encode("updated_at", note.UpdatedAt)
}

func noteToTask(note taskNote) tasks.Task {
	createdAt, _ := time.Parse(time.RFC3339Nano, note.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, note.UpdatedAt)
	body := note.Body
	if body == "" && len(note.BodySection) > 0 {
		body = note.BodySection
	}
	ac := note.AcceptanceCriteria
	if len(ac) == 0 {
		ac = note.AccCriteriaSection
	}
	vp := note.VerificationPlan
	if len(vp) == 0 {
		vp = note.VerPlanSection
	}
	return tasks.Task{
		ID: note.ID, Title: note.Title, Status: note.Status,
		Priority: note.Priority, ModelLevel: note.ModelLevel,
		TaskType: note.TaskType, ParentID: note.ParentID,
		Tags: note.Tags, Branch: note.Branch,
		WorktreePath: note.WorktreePath,
		AcceptanceCriteria: ac, VerificationPlan: vp,
		ProjectionSource: "obsidian_registry",
		CreatedAt: createdAt, UpdatedAt: updatedAt,
		Body: body,
	}
}

func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
