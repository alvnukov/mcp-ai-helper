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

func NewObsidianTaskBackend(dir string) taskBackend {
	return newObsidianTaskBackend(dir)
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
		WorktreePath:       req.WorktreePath,
		AcceptanceCriteria: yamlStringList(req.AcceptanceCriteria),
		VerificationPlan:   yamlStringList(req.VerificationPlan),
		CreatedAt:          createdAt, UpdatedAt: now.Format(time.RFC3339Nano),
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
		all, err := b.readAll()
		if err != nil {
			return taskBatchMutationResult{}, err
		}
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
			return nil, fmt.Errorf("read obsidian task note %s: %w", id, err)
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
		repaired := quotePlainScalarFrontmatter(fm)
		if repaired == fm {
			return taskNote{}, fmt.Errorf("frontmatter parse failed in %s.md: %w", expectedID, err)
		}
		if retryErr := yaml.Unmarshal([]byte(repaired), &note); retryErr != nil {
			return taskNote{}, fmt.Errorf("frontmatter parse failed in %s.md: %w", expectedID, err)
		}
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
