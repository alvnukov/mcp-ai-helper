package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// TaskContextRequest is the input for building task execution context.
type TaskContextRequest struct {
	RepoPath string `json:"repo_path"`
	TaskID   string `json:"task_id"`
	MaxNodes int    `json:"max_nodes,omitempty"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

// TaskContext is the execution context for a selected task.
type TaskContext struct {
	Task               TaskContextSelected      `json:"task"`
	GoalChain          []TaskContextItem        `json:"goal_chain"`
	Prerequisites      []TaskContextItem        `json:"prerequisites"`
	AlreadyDone        []TaskContextItem        `json:"already_done"`
	PlannedNext        []TaskContextItem        `json:"planned_next"`
	Blockers           []TaskContextItem        `json:"blockers"`
	Boundaries         []string                 `json:"boundaries,omitempty"`
	NonGoals           []string                 `json:"non_goals,omitempty"`
	AcceptanceCriteria []string                 `json:"acceptance_criteria,omitempty"`
	VerificationPlan   []string                 `json:"verification_plan,omitempty"`
	Warnings           []string                 `json:"warnings,omitempty"`
	UsageContract      TaskContextUsageContract `json:"usage_contract"`
	Truncated          *TaskContextTruncation   `json:"truncated,omitempty"`
}

// TaskContextSelected is the primary task being executed.
type TaskContextSelected struct {
	ID         string   `json:"id"`
	Status     string   `json:"status"`
	Title      string   `json:"title"`
	Priority   string   `json:"priority,omitempty"`
	ModelLevel string   `json:"model_level,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	TaskType   string   `json:"task_type,omitempty"`
}

// TaskContextItem is a brief reference to a related task.
type TaskContextItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// TaskContextUsageContract describes intended use, must-not rules, and truncation guidance.
type TaskContextUsageContract struct {
	IntendedUse string `json:"intended_use"`
	MustNot     string `json:"must_not"`
	IfTruncated string `json:"if_truncated"`
}

// TaskContextTruncation records what was omitted due to limits.
type TaskContextTruncation struct {
	OmittedGoalChain     int    `json:"omitted_goal_chain,omitempty"`
	OmittedPrerequisites int    `json:"omitted_prerequisites,omitempty"`
	OmittedAlreadyDone   int    `json:"omitted_already_done,omitempty"`
	OmittedPlannedNext   int    `json:"omitted_planned_next,omitempty"`
	Reason               string `json:"reason,omitempty"`
}

const (
	defaultCtxMaxNodes = 20
	defaultCtxMaxBytes = 4096
)

func defaultContextLimits(req TaskContextRequest) (int, int) {
	maxNodes := req.MaxNodes
	if maxNodes <= 0 {
		maxNodes = defaultCtxMaxNodes
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultCtxMaxBytes
	}
	return maxNodes, maxBytes
}

// BuildTaskContext builds a compact execution context for a selected task.
func BuildTaskContext(all []tasks.Task, req TaskContextRequest) (TaskContext, error) {
	maxNodes, maxBytes := defaultContextLimits(req)

	taskMap := make(map[string]tasks.Task, len(all))
	for i := range all {
		taskMap[all[i].ID] = all[i]
	}

	if strings.TrimSpace(req.TaskID) == "" {
		return TaskContext{}, fmt.Errorf("task_id is required; use task_current to list available tasks")
	}

	var selected *tasks.Task
	for i := range all {
		if all[i].ID == req.TaskID {
			t := all[i]
			selected = &t
			break
		}
	}
	if selected == nil {
		return TaskContext{}, fmt.Errorf("task %q not found; use task_current to list available tasks", req.TaskID)
	}

	grid := buildGoalChain(selected, taskMap)
	if len(grid) == 0 {
		grid = []TaskContextItem{{ID: selected.ID, Title: selected.Title, Status: selected.Status}}
	}
	prereqs := buildPrerequisites(selected, taskMap)
	done, planned, blockers := buildRelatedTasks(selected, all, taskMap)
	boundaries, nonGoals := extractBoundaries(selected.Body)
	warnings := buildWarnings(selected, taskMap, blockers)
	if len(boundaries) == 0 && len(nonGoals) == 0 {
		warnings = append(warnings, "execution boundaries and non-goals not found in task body; verify scope with task_get or parent task")
	}

	result := TaskContext{
		Task:               taskToSelected(*selected),
		AlreadyDone:        done,
		PlannedNext:        planned,
		Blockers:           blockers,
		Boundaries:         boundaries,
		NonGoals:           nonGoals,
		AcceptanceCriteria: nonNilStrings(selected.AcceptanceCriteria),
		VerificationPlan:   nonNilStrings(selected.VerificationPlan),
		Warnings:           warnings,
		UsageContract:      buildUsageContract(),
	}

	result = applyContextLimits(result, grid, prereqs, maxNodes)
	result = enforceContextMaxBytes(result, maxBytes)

	return result, nil
}

func buildGoalChain(selected *tasks.Task, taskMap map[string]tasks.Task) []TaskContextItem {
	chain := make([]TaskContextItem, 0)
	currentID := selected.ParentID
	for currentID != "" {
		parent, ok := taskMap[currentID]
		if !ok {
			break
		}
		chain = append(chain, TaskContextItem{ID: parent.ID, Title: parent.Title, Status: parent.Status})
		currentID = parent.ParentID
	}
	// Reverse so goal is first
	if len(chain) > 1 {
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
	}
	return chain
}

func buildPrerequisites(selected *tasks.Task, taskMap map[string]tasks.Task) []TaskContextItem {
	prereqs := make([]TaskContextItem, 0)
	currentID := selected.ParentID
	for currentID != "" {
		parent, ok := taskMap[currentID]
		if !ok {
			break
		}
		if parent.Status != "done" {
			prereqs = append(prereqs, TaskContextItem{ID: parent.ID, Title: parent.Title, Status: parent.Status})
		}
		currentID = parent.ParentID
	}
	return prereqs
}

func buildRelatedTasks(selected *tasks.Task, all []tasks.Task, taskMap map[string]tasks.Task) (done, planned, blockers []TaskContextItem) {
	done = make([]TaskContextItem, 0)
	planned = make([]TaskContextItem, 0)
	blockers = make([]TaskContextItem, 0)

	parentID := selected.ParentID
	for _, t := range all {
		if t.ID == selected.ID {
			continue
		}
		// Siblings or children. Root-level tasks have no useful sibling scope,
		// so avoid treating the whole top-level backlog as selected-task context.
		isSibling := parentID != "" && t.ParentID == parentID
		isChild := t.ParentID == selected.ID
		if isSibling || isChild {
			switch t.Status {
			case "done":
				done = append(done, TaskContextItem{ID: t.ID, Title: t.Title, Status: t.Status})
			case "blocked":
				blockers = append(blockers, TaskContextItem{ID: t.ID, Title: t.Title, Status: t.Status})
			default:
				planned = append(planned, TaskContextItem{ID: t.ID, Title: t.Title, Status: t.Status})
			}
		}
	}

	// If parent is blocked, add it as a blocker
	if parentID != "" {
		if parent, ok := taskMap[parentID]; ok && parent.Status == "blocked" {
			blockers = append(blockers, TaskContextItem{ID: parent.ID, Title: parent.Title, Status: parent.Status})
		}
	}

	sortContextItems(done)
	sortContextItems(planned)
	sortContextItems(blockers)
	return
}

func buildWarnings(selected *tasks.Task, taskMap map[string]tasks.Task, blockers []TaskContextItem) []string {
	warnings := make([]string, 0)
	if selected.Status == "blocked" {
		warnings = append(warnings, "selected task is blocked")
	}
	for _, b := range blockers {
		warnings = append(warnings, fmt.Sprintf("blocker: %s (%s)", b.ID, b.Status))
	}
	if selected.ParentID != "" {
		if parent, ok := taskMap[selected.ParentID]; ok && parent.Status == "blocked" {
			warnings = append(warnings, fmt.Sprintf("parent task %s is blocked", parent.ID))
		}
	}
	return warnings
}

func extractBoundaries(body string) ([]string, []string) {
	var boundaries, nonGoals []string
	var inScope, inOutOfScope bool
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			inScope = false
			inOutOfScope = false
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "required scope") || strings.HasPrefix(lower, "scope") && !strings.HasPrefix(lower, "out of scope") {
			inScope = true
			inOutOfScope = false
			continue
		}
		if strings.HasPrefix(lower, "out of scope") {
			inOutOfScope = true
			inScope = false
			continue
		}
		if inScope && (strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*")) {
			item := strings.TrimLeft(line, "-* ")
			item = strings.TrimSpace(item)
			if item != "" {
				boundaries = append(boundaries, item)
			}
		}
		if inOutOfScope && (strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*")) {
			item := strings.TrimLeft(line, "-* ")
			item = strings.TrimSpace(item)
			if item != "" {
				nonGoals = append(nonGoals, item)
			}
		}
	}
	return boundaries, nonGoals
}

func buildUsageContract() TaskContextUsageContract {
	return TaskContextUsageContract{
		IntendedUse: "execution context for a single selected task. Use task_current to discover active tasks, and task_context to get detailed context before editing.",
		MustNot:     "do not use as a substitute for task_current or full registry dumps; do not fill missing data with speculation; do not mutate task state without task tools",
		IfTruncated: "if truncated, retry with larger max_nodes or max_bytes, or use task_graph for broader overview",
	}
}

func applyContextLimits(ctx TaskContext, grid, prereqs []TaskContextItem, maxNodes int) TaskContext {
	if len(grid) > maxNodes {
		ctx.Truncated = &TaskContextTruncation{
			OmittedGoalChain: len(grid) - maxNodes,
			Reason:           fmt.Sprintf("max_nodes limit reached (%d)", maxNodes),
		}
		grid = grid[len(grid)-maxNodes:] // keep closest to selected
	}
	ctx.GoalChain = grid

	if len(prereqs) > maxNodes {
		prereqs = prereqs[:maxNodes]
		if ctx.Truncated == nil {
			ctx.Truncated = &TaskContextTruncation{}
		}
		ctx.Truncated.OmittedPrerequisites = len(prereqs)
		if ctx.Truncated.Reason == "" {
			ctx.Truncated.Reason = fmt.Sprintf("max_nodes limit reached (%d)", maxNodes)
		}
	}
	ctx.Prerequisites = prereqs

	total := len(ctx.AlreadyDone) + len(ctx.PlannedNext)
	if total > maxNodes {
		cut := total - maxNodes
		if cut >= len(ctx.PlannedNext) {
			ctx.PlannedNext = nil
			cut -= len(ctx.PlannedNext)
			if cut > 0 && len(ctx.AlreadyDone) >= cut {
				ctx.AlreadyDone = ctx.AlreadyDone[:cut]
			}
		} else {
			ctx.PlannedNext = ctx.PlannedNext[:cut]
		}
		if ctx.Truncated == nil {
			ctx.Truncated = &TaskContextTruncation{}
		}
		ctx.Truncated.OmittedAlreadyDone = cut
		if ctx.Truncated.Reason == "" {
			ctx.Truncated.Reason = fmt.Sprintf("max_nodes limit reached (%d)", maxNodes)
		}
	}

	return ctx
}

func taskToSelected(t tasks.Task) TaskContextSelected {
	selected := TaskContextSelected{
		ID: t.ID, Status: t.Status, Title: t.Title,
		Priority: t.Priority, ModelLevel: t.ModelLevel, TaskType: t.TaskType,
	}
	if len(t.Tags) > 0 {
		selected.Tags = make([]string, len(t.Tags))
		copy(selected.Tags, t.Tags)
	}
	return selected
}

func sortContextItems(items []TaskContextItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func enforceContextMaxBytes(ctx TaskContext, maxBytes int) TaskContext {
	if maxBytes <= 0 {
		return ctx
	}
	// Trim one item at a time from non-essential sections until under limit
	sections := []struct {
		slice *[]string
	}{
		{&ctx.NonGoals},
		{&ctx.Boundaries},
		{&ctx.VerificationPlan},
		{&ctx.AcceptanceCriteria},
		{&ctx.Warnings},
	}
	for {
		data, err := json.Marshal(ctx)
		if err != nil || len(data) <= maxBytes {
			if ctx.Truncated == nil && len(ctx.Warnings) < cap(ctx.Warnings) {
				// We trimmed something
			}
			return ctx
		}
		trimmed := false
		for _, sec := range sections {
			if len(*sec.slice) > 0 {
				*sec.slice = (*sec.slice)[:len(*sec.slice)-1]
				trimmed = true
				break
			}
		}
		if !trimmed {
			break
		}
	}
	if ctx.Truncated == nil {
		ctx.Truncated = &TaskContextTruncation{}
	}
	ctx.Truncated.Reason = fmt.Sprintf("max_bytes limit reached (%d)", maxBytes)
	return ctx
}

// validateTaskContextRequest checks request arguments before context construction.
func validateTaskContextRequest(req TaskContextRequest) error {
	if strings.TrimSpace(req.RepoPath) == "" {
		return fmt.Errorf("repo_path is required")
	}
	if strings.TrimSpace(req.TaskID) == "" {
		return fmt.Errorf("task_id is required; use task_current to list available tasks")
	}
	if req.MaxNodes < 0 {
		return fmt.Errorf("max_nodes must be >= 0, got %d", req.MaxNodes)
	}
	if req.MaxBytes < 0 {
		return fmt.Errorf("max_bytes must be >= 0, got %d", req.MaxBytes)
	}
	return nil
}
