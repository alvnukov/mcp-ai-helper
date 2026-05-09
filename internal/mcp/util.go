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
	for _, task := range list {
		switch task.Status {
		case "todo", "in_progress", "blocked":
			current = append(current, task)
		}
	}
	return current
}
