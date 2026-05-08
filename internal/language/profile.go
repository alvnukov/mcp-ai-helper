// Package language defines language-aware workflow guardrails.
package language

import (
	"path/filepath"
	"sort"
	"strings"
)

// Profile describes deterministic language-specific safety defaults.
type Profile struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Extensions          []string `json:"extensions"`
	PatchableExtensions []string `json:"patchable_extensions"`
	Formatter           []string `json:"formatter"`
	TargetedTests       []string `json:"targeted_tests"`
	BroadTests          []string `json:"broad_tests"`
	StaticChecks        []string `json:"static_checks"`
	ImportCheck         bool     `json:"import_check"`
	Guardrails          []string `json:"guardrails"`
}

// Registry stores known language profiles by id.
type Registry struct {
	profiles map[string]Profile
}

// DefaultRegistry returns built-in language profiles.
func DefaultRegistry() Registry {
	profiles := map[string]Profile{}
	goProfile := Profile{
		ID:                  "go",
		Name:                "Go",
		Extensions:          []string{".go", "go.mod", "go.sum"},
		PatchableExtensions: []string{".go", "go.mod", "go.sum", ".md", ".yaml", ".yml"},
		Formatter:           []string{"gofmt -w <owned_go_files>"},
		TargetedTests:       []string{"go test <affected_packages>"},
		BroadTests:          []string{"go test ./..."},
		StaticChecks:        []string{"go vet ./...", "golangci-lint run ./... when installed"},
		ImportCheck:         true,
		Guardrails: []string{
			"Run gofmt only on files whose extension is exactly .go.",
			"Run targeted go test for affected packages before broad go test ./... .",
			"Treat missing imports, undefined symbols, and package compile failures as compile blockers, not test failures.",
			"Use guarded symbol/range edits for Go source; do not rely on blind string replacement when symbols can be resolved first.",
			"Commit only owned files after formatter, targeted tests, broad tests, and vet pass.",
		},
	}
	profiles[goProfile.ID] = goProfile
	return Registry{profiles: profiles}
}

// List returns profiles in stable id order.
func (r Registry) List() []Profile {
	ids := make([]string, 0, len(r.profiles))
	for id := range r.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Profile, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.profiles[id])
	}
	return out
}

// Get returns a profile by id.
func (r Registry) Get(id string) (Profile, bool) {
	profile, ok := r.profiles[strings.ToLower(strings.TrimSpace(id))]
	return profile, ok
}

// Detect returns profiles that match the provided file paths.
func (r Registry) Detect(paths []string) []Profile {
	seen := map[string]struct{}{}
	for _, path := range paths {
		ext := filepath.Ext(path)
		base := filepath.Base(path)
		for id, profile := range r.profiles {
			if matchesProfile(profile, ext, base) {
				seen[id] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Profile, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.profiles[id])
	}
	return out
}

func matchesProfile(profile Profile, ext string, base string) bool {
	for _, allowed := range profile.Extensions {
		if allowed == ext || allowed == base {
			return true
		}
	}
	return false
}
