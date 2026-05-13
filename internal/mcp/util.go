package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func bind(req basemcp.CallToolRequest, target any) error {
	data, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func structured(value any) (*basemcp.CallToolResult, error) {
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return basemcp.NewToolResultStructured(value, string(text)), nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
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

func currentTasks(list []tasks.Task) []tasks.Task {
	current := make([]tasks.Task, 0, len(list))
	goals := make([]tasks.Task, 0)
	for _, task := range list {
		switch task.Status {
		case "todo", "in_progress", "blocked":
			if hasTag(task.Tags, "goal") {
				goals = append(goals, task)
			} else {
				current = append(current, task)
			}
		}
	}
	return append(goals, current...)
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

func buildTaskTree(list []tasks.Task) map[string]any {
	var goal *tasks.Task
	for i := range list {
		if hasTag(list[i].Tags, "goal") && list[i].ParentID == "" {
			g := list[i]
			goal = &g
			break
		}
	}
	if goal == nil {
		return map[string]any{
			"goal":     nil,
			"children": []map[string]any{},
			"diagnostic": map[string]string{
				"code":      "task_tree_no_goal_root",
				"message":   "no root task with tag 'goal' and empty parent_id was found",
				"next_call": "task_graph",
			},
		}
	}
	children := map[string][]tasks.Task{}
	for _, t := range list {
		if t.ParentID != "" {
			children[t.ParentID] = append(children[t.ParentID], t)
		}
	}
	return map[string]any{"goal": goal, "children": buildSubTree(goal.ID, children)}
}

func buildSubTree(parentID string, children map[string][]tasks.Task) []map[string]any {
	var result []map[string]any
	for _, child := range children[parentID] {
		node := map[string]any{"task": child}
		if sub := buildSubTree(child.ID, children); sub != nil {
			node["children"] = sub
		}
		result = append(result, node)
	}
	return result
}
