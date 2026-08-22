package mcp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/alvnukov/mcp-ai-helper/internal/accesspolicy"
	"github.com/alvnukov/mcp-ai-helper/internal/config"
	"github.com/alvnukov/mcp-ai-helper/internal/pipeline"
)

func (s *Server) inspectGenericAccess(tool string, action string, args map[string]any, repoCfg *config.RepoConfig) error {
	if s == nil {
		return nil
	}
	base, _, _, _, _ := s.loadDeps()
	if base == nil {
		return nil
	}
	repoPath, _ := args["repo_path"].(string)
	merged, err := config.MergeRepoConfig(base, repoCfg, repoPath)
	if err != nil {
		return fmt.Errorf("resolve access policy: %w", err)
	}
	policy := accesspolicy.New(
		repoPath,
		merged.TaskRegistry.Obsidian.ResolvedPath,
		base.SourcePath,
	)
	switch tool {
	case "file":
		return inspectFileAccess(policy, action, args)
	case "edit":
		path, _ := args["path"].(string)
		return policy.CheckPath(tool, action, path)
	case "command":
		if action == "run" {
			command, _ := args["command"].(string)
			return policy.CheckCommand(tool, action, command)
		}
	case "run":
		return inspectRunAccess(policy, action, args)
	}
	return nil
}

func inspectFileAccess(policy *accesspolicy.Policy, action string, args map[string]any) error {
	if action == "read_many" {
		for _, path := range stringSlice(args["paths"]) {
			if err := policy.CheckPath("file", action, path); err != nil {
				return err
			}
		}
		return nil
	}
	path, _ := args["path"].(string)
	if action == "search" {
		if strings.TrimSpace(path) != "" {
			if err := policy.CheckPath("file", action, path); err != nil {
				return err
			}
		}
		root, _ := args["repo_path"].(string)
		if strings.TrimSpace(path) != "" {
			root = filepath.Join(root, path)
		}
		args["glob_exclude"] = appendUniqueStrings(
			stringSlice(args["glob_exclude"]),
			policy.SearchExcludes(root)...,
		)
		return nil
	}
	return policy.CheckPath("file", action, path)
}

func inspectRunAccess(policy *accesspolicy.Policy, action string, args map[string]any) error {
	if action == "pipeline" {
		command, _ := args["command"].(string)
		return policy.CheckCommand("run", action, command)
	}
	if action != "workflow" {
		return nil
	}
	for _, step := range objectSlice(args["steps"]) {
		stepTool, _ := step["tool"].(string)
		stepArgs, _ := step["args"].(map[string]any)
		switch stepTool {
		case "command":
			command, _ := stepArgs["command"].(string)
			if err := policy.CheckCommand("run", action, command); err != nil {
				return err
			}
		case "guarded_replace", pipeline.WorkflowStepWriteFile:
			path, _ := stepArgs["path"].(string)
			if err := policy.CheckPath("run", action, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func objectSlice(value any) []map[string]any {
	switch values := value.(type) {
	case []map[string]any:
		return values
	case []any:
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			if object, ok := value.(map[string]any); ok {
				out = append(out, object)
			}
		}
		return out
	default:
		return nil
	}
}

func stringSlice(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if item, ok := value.(string); ok {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	out := make([]string, 0, len(values)+len(additions))
	for _, value := range append(values, additions...) {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
