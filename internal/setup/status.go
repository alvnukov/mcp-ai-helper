package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Status answers the question setup itself cannot: is the helper actually
// installed here, and is what is installed what this build would write?
//
// Every way registration goes wrong fails quietly. A client that never got the
// server entry simply does not offer the tools. An entry left pointing at a
// binary that has since moved looks the same from inside the client. A skill
// file written by an older build reads exactly like a current one. None of these
// announce themselves; they surface as a model behaving worse than expected for
// reasons nobody connects back to the install.
//
// The report is deliberately the same shape as the one Run prints, so the two
// can be read against each other. It returns false when anything is missing or
// out of date, which is what lets a caller turn it into an exit code.
func Status(opts Options, out io.Writer) (bool, error) {
	current := true
	report := make([]string, 0, len(opts.Clients))
	for _, requested := range opts.Clients {
		spec, err := client(requested)
		if err != nil {
			return false, err
		}
		lines := []string{spec.label}

		line, fresh, err := serverStatus(spec, opts.Global)
		if err != nil {
			return false, err
		}
		current = current && fresh
		lines = append(lines, line)

		if !opts.NoInstructions {
			line, fresh, err := instructionsStatus(spec, opts.Global)
			if err != nil {
				return false, err
			}
			current = current && fresh
			lines = append(lines, line)
		}

		if !opts.NoSkills {
			line, fresh, err := skillsStatus(spec, opts.Global)
			if err != nil {
				return false, err
			}
			current = current && fresh
			lines = append(lines, line)
		}

		report = append(report, strings.Join(lines, "\n"))
	}

	if _, err := fmt.Fprintln(out, strings.Join(report, "\n\n")); err != nil {
		return false, fmt.Errorf("write status report: %w", err)
	}
	if !current {
		if _, err := fmt.Fprintln(out, "\nRun setup again to bring this up to date."); err != nil {
			return false, fmt.Errorf("write status advice: %w", err)
		}
	}
	return current, nil
}

func serverStatus(spec clientSpec, global bool) (string, bool, error) {
	path, err := configPath(spec, global)
	if err != nil {
		return "", false, err
	}
	existing, err := readConfig(path)
	if err != nil {
		return "", false, err
	}
	command, registered, err := registeredCommand(existing, spec.format)
	if err != nil {
		return "", false, err
	}
	if !registered {
		return note("server", path, "not registered"), false, nil
	}
	if strings.TrimSpace(command) == "" {
		return note("server", path, "registered, but the entry names no command"), false, nil
	}
	switch _, err := os.Stat(command); {
	case errors.Is(err, os.ErrNotExist):
		return note("server", path, "registered — "+command+" is gone"), false, nil
	case err != nil:
		return "", false, fmt.Errorf("inspect %s: %w", command, err)
	}
	return note("server", path, "registered — "+command), true, nil
}

func instructionsStatus(spec clientSpec, global bool) (string, bool, error) {
	path, err := spec.guidance.resolve(global)
	if err != nil {
		return "", false, err
	}
	existing, err := readConfig(path)
	if err != nil {
		return "", false, err
	}
	start, end, found, err := span(existing)
	if err != nil {
		return "", false, err
	}
	if !found {
		return note("instructions", path, "no helper block"), false, nil
	}
	if existing[start:end] != block() {
		return note("instructions", path, "out of date"), false, nil
	}
	return note("instructions", path, "current"), true, nil
}

func skillsStatus(spec clientSpec, global bool) (string, bool, error) {
	if spec.skills == nil {
		return fmt.Sprintf("  %-13s %s documents no skill directory", "skills", spec.label), true, nil
	}
	dir, err := skillsPath(spec, global)
	if err != nil {
		return "", false, err
	}

	fresh := 0
	stale := make([]string, 0, len(skills))
	missing := make([]string, 0, len(skills))
	for _, skill := range skills {
		state, err := skillState(dir, skill)
		if err != nil {
			return "", false, err
		}
		switch state {
		case "missing":
			missing = append(missing, skill.name)
		case "stale":
			stale = append(stale, skill.name)
		default:
			fresh++
		}
	}

	parts := make([]string, 0, 3)
	if fresh > 0 {
		parts = append(parts, fmt.Sprintf("%d current", fresh))
	}
	if len(stale) > 0 {
		parts = append(parts, "out of date: "+strings.Join(stale, ", "))
	}
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ", "))
	}
	if len(parts) == 0 {
		parts = append(parts, "none installed")
	}
	return note("skills", dir, strings.Join(parts, ", ")), len(stale) == 0 && len(missing) == 0, nil
}

// skillState reports one skill as missing, stale or current.
//
// A skill is only current when every file it installs is present and says
// exactly what this build would write. Half a skill — a SKILL.md an older build
// left behind with no companion metadata — counts as missing rather than stale,
// because that is what the fix amounts to.
func skillState(dir string, s skill) (string, error) {
	state := "current"
	for _, file := range s.files() {
		existing, err := readConfig(filepath.Join(dir, s.name, file.path))
		if err != nil {
			return "", err
		}
		if existing == "" {
			return "missing", nil
		}
		if existing != file.body {
			state = "stale"
		}
	}
	return state, nil
}
