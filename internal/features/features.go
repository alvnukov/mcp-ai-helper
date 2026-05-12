// Package features resolves and persists helper feature flags.
package features

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/zol/mcp-ai-helper/internal/config"
)

const (
	ScopeGlobal = "global"
	ScopeRepo   = "repo"

	SourceCodeDefault = "code_default"
	SourceGlobal      = "global"
	SourceRepo        = "repo"

	repoGitignoreEntry = ".mcp-ai-helper.yaml"
)

// Definition describes one feature known to this helper binary.
type Definition struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	DefaultEnabled bool     `json:"default_enabled"`
	Stability      string   `json:"stability"`
	RiskNotes      string   `json:"risk_notes,omitempty"`
	Preconditions  []string `json:"preconditions,omitempty"`
	Conflicts      []string `json:"conflicts,omitempty"`
}

// Resolved is a feature definition plus effective state and sources.
type Resolved struct {
	Definition
	GlobalOverride   *bool  `json:"global_override,omitempty"`
	RepoOverride     *bool  `json:"repo_override,omitempty"`
	EffectiveEnabled bool   `json:"effective_enabled"`
	Source           string `json:"source"`
}

// Manager reads and writes feature overrides.
type Manager struct {
	GlobalStatePath string
	Now             func() time.Time
}

// Registry returns the static feature registry compiled into this helper.
func Registry() []Definition {
	items := []Definition{
		{
			ID:             "bounded_external_worker",
			Title:          "Bounded external worker",
			Description:    "Allow future bounded external model worker plumbing when implemented.",
			DefaultEnabled: false,
			Stability:      "experimental",
			RiskNotes:      "Does not grant shell or filesystem access by itself; concrete worker tools still need their own policy checks.",
		},
		{
			ID:             "strict_task_closeout",
			Title:          "Strict task closeout",
			Description:    "Keep strict task closeout guidance and workflow gating enabled by default.",
			DefaultEnabled: true,
			Stability:      "stable",
		},
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

// GlobalStatePathForConfig returns the helper-wide feature state path next to the active config.
func GlobalStatePathForConfig(configPath string) string {
	path := strings.TrimSpace(configPath)
	if path == "" {
		path = config.DefaultConfigPath()
	}
	return filepath.Join(filepath.Dir(path), "features.yaml")
}

// NewManager creates a feature manager.
func NewManager(globalStatePath string) *Manager {
	return &Manager{GlobalStatePath: globalStatePath, Now: time.Now}
}

// List resolves all known features. repoPath is optional; when empty only code/global state applies.
func (m *Manager) List(repoPath string) ([]Resolved, error) {
	globalState, err := loadState(m.GlobalStatePath)
	if err != nil {
		return nil, err
	}
	repoState, err := loadRepoState(repoPath)
	if err != nil {
		return nil, err
	}
	items := Registry()
	out := make([]Resolved, 0, len(items))
	for _, def := range items {
		out = append(out, resolve(def, globalState, repoState))
	}
	return out, nil
}

// Get resolves one known feature.
func (m *Manager) Get(id string, repoPath string) (Resolved, error) {
	def, err := lookup(id)
	if err != nil {
		return Resolved{}, err
	}
	globalState, err := loadState(m.GlobalStatePath)
	if err != nil {
		return Resolved{}, err
	}
	repoState, err := loadRepoState(repoPath)
	if err != nil {
		return Resolved{}, err
	}
	return resolve(def, globalState, repoState), nil
}

// Set writes a global or repo-local override. enabled == nil resets the selected scope.
func (m *Manager) Set(scope string, repoPath string, id string, enabled *bool, reason string) (Resolved, error) {
	def, err := lookup(id)
	if err != nil {
		return Resolved{}, err
	}
	scope = strings.TrimSpace(scope)
	switch scope {
	case ScopeGlobal:
		return m.setGlobal(def, enabled, reason)
	case ScopeRepo:
		if strings.TrimSpace(repoPath) == "" {
			return Resolved{}, errors.New("repo_path is required for repo feature overrides")
		}
		return m.setRepo(repoPath, def, enabled, reason)
	default:
		return Resolved{}, fmt.Errorf("unknown feature scope %q", scope)
	}
}

// RepoStatePath returns the repo-local helper config path.
func RepoStatePath(repoPath string) (string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return "", errors.New("repo_path is required")
	}
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, ".mcp-ai-helper.yaml"), nil
}

func (m *Manager) setGlobal(def Definition, enabled *bool, reason string) (Resolved, error) {
	state, err := loadState(m.GlobalStatePath)
	if err != nil {
		return Resolved{}, err
	}
	previous := resolve(def, state, config.FeatureState{})
	updateState(&state, ScopeGlobal, def.ID, enabled, reason, previous, m.now())
	if err := writeState(m.GlobalStatePath, state); err != nil {
		return Resolved{}, err
	}
	return resolve(def, state, config.FeatureState{}), nil
}

func (m *Manager) setRepo(repoPath string, def Definition, enabled *bool, reason string) (Resolved, error) {
	globalState, err := loadState(m.GlobalStatePath)
	if err != nil {
		return Resolved{}, err
	}
	repoState, err := loadRepoState(repoPath)
	if err != nil {
		return Resolved{}, err
	}
	previous := resolve(def, globalState, repoState)
	updateState(&repoState, ScopeRepo, def.ID, enabled, reason, previous, m.now())
	if err := ensureRepoConfigIgnored(repoPath); err != nil {
		return Resolved{}, err
	}
	if err := writeRepoState(repoPath, repoState); err != nil {
		return Resolved{}, err
	}
	return resolve(def, globalState, repoState), nil
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

func lookup(id string) (Definition, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Definition{}, errors.New("feature id is required")
	}
	for _, def := range Registry() {
		if def.ID == id {
			return def, nil
		}
	}
	return Definition{}, fmt.Errorf("unknown feature id %q", id)
}

func resolve(def Definition, globalState config.FeatureState, repoState config.FeatureState) Resolved {
	result := Resolved{Definition: def, EffectiveEnabled: def.DefaultEnabled, Source: SourceCodeDefault}
	if override, ok := globalState.Overrides[def.ID]; ok {
		value := override.Enabled
		result.GlobalOverride = &value
		result.EffectiveEnabled = value
		result.Source = SourceGlobal
	}
	if override, ok := repoState.Overrides[def.ID]; ok {
		value := override.Enabled
		result.RepoOverride = &value
		result.EffectiveEnabled = value
		result.Source = SourceRepo
	}
	return result
}

func updateState(state *config.FeatureState, scope string, id string, enabled *bool, reason string, previous Resolved, now time.Time) {
	if state.Overrides == nil {
		state.Overrides = map[string]config.FeatureOverride{}
	}
	if enabled == nil {
		delete(state.Overrides, id)
	} else {
		state.Overrides[id] = config.FeatureOverride{Enabled: *enabled, Reason: strings.TrimSpace(reason), UpdatedAt: now.Format(time.RFC3339)}
	}
	next := previous
	if enabled == nil {
		switch scope {
		case ScopeGlobal:
			next.GlobalOverride = nil
			next.EffectiveEnabled = previous.DefaultEnabled
			next.Source = SourceCodeDefault
		case ScopeRepo:
			next.RepoOverride = nil
			if previous.GlobalOverride != nil {
				next.EffectiveEnabled = *previous.GlobalOverride
				next.Source = SourceGlobal
			} else {
				next.EffectiveEnabled = previous.DefaultEnabled
				next.Source = SourceCodeDefault
			}
		}
	} else {
		next.EffectiveEnabled = *enabled
		next.Source = scope
	}
	state.Audit = append(state.Audit, config.FeatureAuditEntry{
		Timestamp:       now.Format(time.RFC3339),
		Scope:           scope,
		FeatureID:       id,
		PreviousEnabled: previous.EffectiveEnabled,
		PreviousSource:  previous.Source,
		NewEnabled:      next.EffectiveEnabled,
		NewSource:       next.Source,
		Reason:          strings.TrimSpace(reason),
	})
}

func loadRepoState(repoPath string) (config.FeatureState, error) {
	repoCfg, err := config.LoadRepoConfig(repoPath)
	if err != nil {
		return config.FeatureState{}, err
	}
	if repoCfg == nil {
		return config.FeatureState{}, nil
	}
	return repoCfg.Features, nil
}

func loadState(path string) (config.FeatureState, error) {
	if strings.TrimSpace(path) == "" {
		return config.FeatureState{}, nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is helper-owned local feature state.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return config.FeatureState{}, nil
		}
		return config.FeatureState{}, err
	}
	var state config.FeatureState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return config.FeatureState{}, fmt.Errorf("invalid feature state %s: %w", path, err)
	}
	return state, nil
}

func writeState(path string, state config.FeatureState) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("feature state path is required")
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600) // #nosec G306 -- helper state must stay user-private.
}

func writeRepoState(repoPath string, state config.FeatureState) error {
	path, err := RepoStatePath(repoPath)
	if err != nil {
		return err
	}
	doc := map[string]any{}
	data, err := os.ReadFile(path) // #nosec G304 -- path is repo-local helper config.
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if len(data) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("invalid repo config %s: %w", path, err)
		}
	}
	doc["features"] = state
	out, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600) // #nosec G306 -- repo-local helper config should be private.
}

func ensureRepoConfigIgnored(repoPath string) error {
	abs, err := filepath.Abs(strings.TrimSpace(repoPath))
	if err != nil {
		return err
	}
	if abs == "" {
		return errors.New("repo_path is required")
	}
	path := filepath.Join(abs, ".gitignore")
	data, err := os.ReadFile(path) // #nosec G304 -- path is repo-local .gitignore.
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == repoGitignoreEntry {
			return nil
		}
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += repoGitignoreEntry + "\n"
	return os.WriteFile(path, []byte(text), 0o644) // #nosec G306,G703 -- .gitignore is intentionally repo-readable under the caller-selected repo.
}
