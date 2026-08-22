// Package accesspolicy recognizes ordinary attempts to bypass helper-owned surfaces.
package accesspolicy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type resourceKind string

const (
	resourceTaskRegistry resourceKind = "task_registry"
	resourceHelperConfig resourceKind = "helper_config"
)

var (
	wholeEnvironmentDump = regexp.MustCompile(`(?i)(^|[;&|][[:space:]]*)(env|printenv|set|export[[:space:]]+-p|declare[[:space:]]+-x)[[:space:]]*($|[;&|])`)
	namedSecretPrint     = regexp.MustCompile(`(?i)(printenv[[:space:]]+[A-Za-z_][A-Za-z0-9_]*(TOKEN|SECRET|PASSWORD|API[_-]?KEY)|(echo|printf)[[:space:]]+[^;&|]*[$][{]?[A-Za-z_][A-Za-z0-9_]*(TOKEN|SECRET|PASSWORD|API[_-]?KEY|HELPER_SECRET))`)
)

// Violation explains which generic surface was rejected and what to use instead.
type Violation struct {
	Code       string
	Tool       string
	Action     string
	Resource   resourceKind
	Target     string
	NextAction string
}

func (v *Violation) Error() string {
	return fmt.Sprintf(
		"policy_denied: %s action=%s cannot access protected %s %q through a generic surface; %s",
		v.Tool,
		v.Action,
		v.Resource,
		v.Target,
		v.NextAction,
	)
}

// Policy owns the protected paths for one repository and the active helper config.
type Policy struct {
	repoPath    string
	taskRoots   []string
	configFiles []string
}

// New builds a policy from already-resolved configuration paths.
func New(repoPath string, taskRegistryPath string, helperConfigPath string) *Policy {
	repoPath = cleanAbs(repoPath)
	taskRoots := []string{filepath.Join(repoPath, "obsidian-tasks")}
	if strings.TrimSpace(taskRegistryPath) != "" {
		taskRoots = append(taskRoots, resolveAgainst(repoPath, taskRegistryPath))
	}
	configFiles := []string{filepath.Join(repoPath, ".mcp-ai-helper.yaml")}
	if strings.TrimSpace(helperConfigPath) == "" {
		helperConfigPath = defaultConfigPath()
	}
	configFiles = append(configFiles, resolveAgainst(repoPath, helperConfigPath))
	return &Policy{
		repoPath:    repoPath,
		taskRoots:   uniqueClean(taskRoots),
		configFiles: uniqueClean(configFiles),
	}
}

// CheckPath rejects direct generic access to a protected registry or config.
func (p *Policy) CheckPath(tool string, action string, target string) error {
	resolved := resolveAgainst(p.repoPath, target)
	resource, protected, ok := p.classifyPath(resolved)
	if !ok {
		return nil
	}
	next := taskAlternative()
	if resource == resourceHelperConfig {
		next = configAlternative()
	}
	return &Violation{
		Code:       "generic_surface_bypass",
		Tool:       tool,
		Action:     action,
		Resource:   resource,
		Target:     protected,
		NextAction: next,
	}
}

// CheckCommand rejects high-confidence protected-path and secret-dump patterns.
// It is deliberately a misuse detector, not a shell sandbox.
func (p *Policy) CheckCommand(tool string, action string, command string) error {
	normalized := normalizeCommand(command)
	if hasFileAccessCue(normalized) {
		if resource, target, ok := p.classifyCommandTarget(normalized); ok {
			next := taskAlternative()
			if resource == resourceHelperConfig {
				next = configAlternative()
			}
			return &Violation{
				Code:       "generic_surface_bypass",
				Tool:       tool,
				Action:     action,
				Resource:   resource,
				Target:     target,
				NextAction: next,
			}
		}
	}
	if wholeEnvironmentDump.MatchString(command) || namedSecretPrint.MatchString(command) ||
		strings.Contains(normalized, "/proc/self/environ") ||
		strings.Contains(normalized, "/proc/1/environ") {
		return fmt.Errorf(
			"policy_denied: %s action=%s looks like a secret or environment dump; use secret_handles to inject only the named secret into the intended command and never print or enumerate it",
			tool,
			action,
		)
	}
	return nil
}

// SearchExcludes returns rg-style exclusions for protected resources below root.
func (p *Policy) SearchExcludes(root string) []string {
	root = resolveAgainst(p.repoPath, root)
	var excludes []string
	for _, taskRoot := range p.taskRoots {
		if rel, ok := relativeInside(root, taskRoot); ok {
			excludes = append(excludes, rel, strings.TrimSuffix(rel, "/")+"/**")
		}
	}
	for _, configFile := range p.configFiles {
		if rel, ok := relativeInside(root, configFile); ok {
			excludes = append(excludes, rel)
		}
	}
	sort.Strings(excludes)
	return uniqueStrings(excludes)
}

func taskAlternative() string {
	return "use task action=current/list/search/get to inspect tasks and task action=upsert/set_status/batch_upsert/delete to mutate them"
}

func configAlternative() string {
	return "inspect the current helper config with config_read; mutate allowlisted scalars with config_option_set/config_option_reset; for any unsupported field report needs_user_action and ask the user to edit it, then call config_reload"
}

func (p *Policy) classifyPath(target string) (resourceKind, string, bool) {
	for _, root := range p.taskRoots {
		if sameOrInside(root, target) {
			return resourceTaskRegistry, root, true
		}
	}
	if isLegacyTaskPath(p.repoPath, target) {
		return resourceTaskRegistry, target, true
	}
	for _, configFile := range p.configFiles {
		if samePath(configFile, target) {
			return resourceHelperConfig, configFile, true
		}
	}
	return "", "", false
}

func (p *Policy) classifyCommandTarget(command string) (resourceKind, string, bool) {
	for _, root := range p.taskRoots {
		for _, marker := range commandMarkers(p.repoPath, root) {
			if marker != "" && strings.Contains(command, marker) {
				return resourceTaskRegistry, marker, true
			}
		}
	}
	for _, marker := range []string{
		"mcpaihelperproject/activetasks.lean",
		"mcpaihelperproject/taskregistry",
	} {
		if strings.Contains(command, marker) {
			return resourceTaskRegistry, marker, true
		}
	}
	if strings.Contains(command, "tasks/") && strings.Contains(command, ".lean") {
		return resourceTaskRegistry, "tasks/*.lean", true
	}
	for _, configFile := range p.configFiles {
		for _, marker := range commandMarkers(p.repoPath, configFile) {
			if marker != "" && strings.Contains(command, marker) {
				return resourceHelperConfig, marker, true
			}
		}
	}
	if strings.Contains(command, "~/.mcp-ai-helper/config.yaml") ||
		strings.Contains(command, ".mcp-ai-helper/config.yaml") {
		return resourceHelperConfig, ".mcp-ai-helper/config.yaml", true
	}
	return "", "", false
}

func hasFileAccessCue(command string) bool {
	cues := []string{
		"cat ", "sed ", "awk ", "head ", "tail ", "less ", "more ",
		"grep ", "rg ", "find ", "ls ", "stat ", "file ", "source ",
		"cp ", "mv ", "rm ", "touch ", "chmod ", "chown ",
		"git ", "python ", "python3 ", "perl ", "ruby ", "node ",
		"tar ", "zip ", "unzip ", "xargs ", "<", ">", "tee ",
	}
	padded := " " + command + " "
	for _, cue := range cues {
		if strings.Contains(padded, " "+cue) ||
			strings.Contains(command, ";"+cue) ||
			strings.Contains(command, "|"+cue) ||
			strings.Contains(command, "&&"+cue) {
			return true
		}
	}
	return false
}

func commandMarkers(repoPath string, target string) []string {
	cleanTarget := filepath.Clean(target)
	markers := []string{normalizeCommand(cleanTarget)}
	rel, err := filepath.Rel(filepath.Clean(repoPath), cleanTarget)
	if err == nil && rel != "." && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		markers = append(markers, normalizeCommand(rel))
	}
	return uniqueStrings(markers)
}

func isLegacyTaskPath(repoPath string, target string) bool {
	rel, err := filepath.Rel(repoPath, target)
	if err != nil {
		return false
	}
	normalized := normalizeCommand(rel)
	if normalized == "mcpaihelperproject/activetasks.lean" {
		return true
	}
	if strings.HasPrefix(normalized, "mcpaihelperproject/taskregistry") &&
		strings.HasSuffix(normalized, ".lean") {
		return true
	}
	return strings.HasPrefix(normalized, "tasks/") && strings.HasSuffix(normalized, ".lean")
}

func relativeInside(root string, target string) (string, bool) {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func sameOrInside(root string, target string) bool {
	_, ok := relativeInside(root, target)
	return ok
}

func samePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func resolveAgainst(root string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return cleanAbs(root)
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return cleanAbs(filepath.Join(root, value))
}

func cleanAbs(value string) string {
	abs, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return filepath.Clean(value)
	}
	return abs
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".mcp-ai-helper", "config.yaml")
	}
	return filepath.Join(home, ".mcp-ai-helper", "config.yaml")
}

func normalizeCommand(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, "\\", "/"))
}

func uniqueClean(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			cleaned = append(cleaned, filepath.Clean(value))
		}
	}
	return uniqueStrings(cleaned)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
