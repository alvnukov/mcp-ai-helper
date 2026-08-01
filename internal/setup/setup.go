// Package setup registers mcp-ai-helper as an MCP server in AI clients, and
// takes it back out again.
//
// Registering the helper with a client is three separate things, and each
// client spells all three differently:
//
//   - the MCP server entry, so the tools exist at all — see mcpconfig.go;
//   - a block in the file the client reads for instructions, so a model knows
//     the tools are worth reaching for — see instructions.go;
//   - skills, which carry the step-by-step workflows without occupying context
//     until they are needed — see skills.go.
//
// Getting any of them wrong fails quietly: the client simply never offers the
// tools, or offers them and never reaches for them.
//
// Both commands are idempotent by construction rather than by care. Every write
// goes through apply, which compares what it is about to write against what is
// on disk and does nothing when they match. One comparison covers all three
// formats — JSON, TOML and markdown — so there is no per-format cleverness to
// get wrong, and a second run reports that everything is already current
// instead of rewriting files to their own contents.
package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// serverName is the key the helper is registered under, and therefore the
// prefix a client puts in front of every tool name it exposes. It is the same
// in every client so that guidance naming a tool is true everywhere.
const serverName = "mcp-ai-helper"

// configFormat is the shape of the client's MCP server list.
type configFormat int

const (
	// claudeJSON is {"mcpServers": {"mcp-ai-helper": {"command": ..., "args": [...]}}}.
	claudeJSON configFormat = iota
	// codexTOML is [mcp_servers.mcp-ai-helper] in TOML.
	codexTOML
	// opencodeJSON is {"mcp": {"mcp-ai-helper": {"type": "local", "command": [...], "enabled": true}}}.
	opencodeJSON
)

// scoped is a path that differs between the project-scoped and the user-wide
// install.
type scoped struct {
	project string
	global  string
}

func (s scoped) resolve(global bool) (string, error) {
	if global {
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, s.global), nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current directory: %w", err)
	}
	return filepath.Join(dir, s.project), nil
}

// clientSpec is a client the helper knows how to register itself with.
type clientSpec struct {
	id     string
	label  string
	format configFormat
	// skills is where this client looks for skills, relative to the project or
	// the home directory. Nil for a client with no skill support to speak of.
	skills *scoped
	// guidance is the file this client reads for instructions.
	guidance scoped
}

// clients records where each client reads what, all of it from the clients' own
// documentation.
//
// Two details are worth keeping in view. Claude Code reads CLAUDE.md and not
// AGENTS.md, so the two files are not interchangeable. And OpenCode also
// searches .claude/skills, but the helper writes its skills to OpenCode's own
// directory anyway: sharing one directory would mean removing the helper from
// Claude Code silently broke it for OpenCode.
var clients = []clientSpec{
	{
		id:     "claude",
		label:  "Claude Code",
		format: claudeJSON,
		skills: &scoped{
			project: filepath.Join(".claude", "skills"),
			global:  filepath.Join(".claude", "skills"),
		},
		guidance: scoped{
			project: "CLAUDE.md",
			global:  filepath.Join(".claude", "CLAUDE.md"),
		},
	},
	{
		id:     "codex",
		label:  "Codex CLI",
		format: codexTOML,
		// Codex documents AGENTS.md but no skill directory, and guessing at one
		// would scatter files it never reads.
		skills: nil,
		guidance: scoped{
			project: "AGENTS.md",
			global:  filepath.Join(".codex", "AGENTS.md"),
		},
	},
	{
		id:     "opencode",
		label:  "OpenCode",
		format: opencodeJSON,
		skills: &scoped{
			project: filepath.Join(".opencode", "skills"),
			global:  filepath.Join(".config", "opencode", "skills"),
		},
		guidance: scoped{
			project: "AGENTS.md",
			global:  filepath.Join(".config", "opencode", "AGENTS.md"),
		},
	},
}

// outcome is what one file ended up doing.
type outcome int

const (
	// outcomeDone means the file was changed.
	outcomeDone outcome = iota
	// outcomeCurrent means it already said exactly this. Nothing was written.
	outcomeCurrent
	// outcomeNothing means there was nothing here to do.
	outcomeNothing
	// outcomeWould is a dry run: this is what would have changed.
	outcomeWould
)

func (o outcome) word(done string, nothing string) string {
	switch o {
	case outcomeDone:
		return done
	case outcomeCurrent:
		return "already up to date"
	case outcomeNothing:
		return nothing
	case outcomeWould:
		return "would be " + done
	default:
		return done
	}
}

// Options are the flags Run and Remove share.
type Options struct {
	// Clients names the clients to touch, by id: claude, codex, opencode.
	Clients []string
	// Global writes the user-wide config instead of the project one.
	Global bool
	// DryRun reports what would change instead of writing it.
	DryRun bool
	// NoInstructions leaves CLAUDE.md / AGENTS.md alone.
	NoInstructions bool
	// NoSkills leaves the skill files alone.
	NoSkills bool
	// ConfigPath pins --config in the command line the client will run. Empty
	// leaves the flag off, so the server falls back to its own default — which
	// is the right answer on a machine where that default is where the config
	// actually lives.
	ConfigPath string
}

// Run registers the helper with every client named in opts, reporting to out.
func Run(opts Options, out io.Writer) error {
	command := helperCommand()
	args := launchArgs(opts.ConfigPath)

	report := make([]string, 0, len(opts.Clients))
	for _, requested := range opts.Clients {
		spec, err := client(requested)
		if err != nil {
			return err
		}
		lines := []string{spec.label}

		path, err := configPath(spec, opts.Global)
		if err != nil {
			return err
		}
		existing, err := readConfig(path)
		if err != nil {
			return err
		}
		var entry string
		switch spec.format {
		case claudeJSON:
			entry, err = mergeJSON(existing, "mcpServers", claudeEntry(command, args))
		case opencodeJSON:
			entry, err = mergeJSON(existing, "mcp", opencodeEntry(command, args))
		case codexTOML:
			entry, err = mergeCodexTOML(existing, command, args)
		}
		if err != nil {
			return err
		}
		result, err := apply(path, &entry, opts.DryRun)
		if err != nil {
			return err
		}
		lines = append(lines, note("server", path, result.word("registered", "nothing to do")))

		if !opts.NoInstructions {
			path, err := spec.guidance.resolve(opts.Global)
			if err != nil {
				return err
			}
			existing, err := readConfig(path)
			if err != nil {
				return err
			}
			wanted, err := withBlock(existing)
			if err != nil {
				return err
			}
			result, err := apply(path, &wanted, opts.DryRun)
			if err != nil {
				return err
			}
			lines = append(lines, note("instructions", path, result.word("added", "nothing to do")))
		}

		if !opts.NoSkills {
			line, err := installSkills(spec, opts.Global, opts.DryRun)
			if err != nil {
				return err
			}
			lines = append(lines, line)
		}

		report = append(report, strings.Join(lines, "\n"))
	}

	if _, err := fmt.Fprintln(out, strings.Join(report, "\n\n")); err != nil {
		return fmt.Errorf("write setup report: %w", err)
	}
	if !opts.DryRun {
		if _, err := fmt.Fprintln(out, "\nRestart the client to pick up the new server."); err != nil {
			return fmt.Errorf("write setup restart notice: %w", err)
		}
	}
	return nil
}

// Remove is the inverse of Run: it takes the helper back out of every client
// named in opts, leaving everything it did not put there alone.
func Remove(opts Options, out io.Writer) error {
	report := make([]string, 0, len(opts.Clients))
	changed := false
	for _, requested := range opts.Clients {
		spec, err := client(requested)
		if err != nil {
			return err
		}
		lines := []string{spec.label}

		path, err := configPath(spec, opts.Global)
		if err != nil {
			return err
		}
		existing, err := readConfig(path)
		if err != nil {
			return err
		}
		var entry *string
		switch spec.format {
		case claudeJSON:
			entry, err = withoutJSON(existing, "mcpServers")
		case opencodeJSON:
			entry, err = withoutJSON(existing, "mcp")
		case codexTOML:
			entry, err = withoutCodexTOML(existing)
		}
		if err != nil {
			return err
		}
		result, err := apply(path, entry, opts.DryRun)
		if err != nil {
			return err
		}
		changed = changed || result == outcomeDone
		lines = append(lines, note("server", path, result.word("removed", "not registered")))

		if !opts.NoInstructions {
			path, err := spec.guidance.resolve(opts.Global)
			if err != nil {
				return err
			}
			existing, err := readConfig(path)
			if err != nil {
				return err
			}
			wanted, err := withoutBlock(existing)
			if err != nil {
				return err
			}
			result, err := apply(path, wanted, opts.DryRun)
			if err != nil {
				return err
			}
			changed = changed || result == outcomeDone
			lines = append(lines, note("instructions", path, result.word("removed", "no helper block")))
		}

		if !opts.NoSkills {
			line, touched, err := uninstallSkills(spec, opts.Global, opts.DryRun)
			if err != nil {
				return err
			}
			changed = changed || touched
			lines = append(lines, line)
		}

		report = append(report, strings.Join(lines, "\n"))
	}

	if _, err := fmt.Fprintln(out, strings.Join(report, "\n\n")); err != nil {
		return fmt.Errorf("write removal report: %w", err)
	}
	if changed {
		if _, err := fmt.Fprintln(out, "\nRestart the client to drop the server."); err != nil {
			return fmt.Errorf("write removal restart notice: %w", err)
		}
	}
	return nil
}

func installSkills(spec clientSpec, global bool, dryRun bool) (string, error) {
	if spec.skills == nil {
		return fmt.Sprintf("  %-13s skipped — %s documents no skill directory", "skills", spec.label), nil
	}
	dir, err := spec.skills.resolve(global)
	if err != nil {
		return "", err
	}

	written := make([]string, 0, len(skills))
	current := make([]string, 0, len(skills))
	for _, skill := range skills {
		body := skill.body
		path := filepath.Join(dir, skill.name, "SKILL.md")
		result, err := apply(path, &body, dryRun)
		if err != nil {
			return "", err
		}
		if result == outcomeDone || result == outcomeWould {
			written = append(written, skill.name)
		} else {
			current = append(current, skill.name)
		}
	}

	var detail string
	switch {
	case len(written) == 0:
		detail = "already up to date"
	case len(current) == 0 && dryRun:
		detail = strings.Join(written, ", ") + " would be written"
	case len(current) == 0:
		detail = strings.Join(written, ", ") + " written"
	default:
		detail = fmt.Sprintf("%s written, %s already current", strings.Join(written, ", "), strings.Join(current, ", "))
	}
	return note("skills", dir, detail), nil
}

// uninstallSkills takes out the SKILL.md files the helper put there, then the
// directories they leave behind — but only while those are empty, so a skill
// somebody else added under the same root survives the helper's uninstall.
func uninstallSkills(spec clientSpec, global bool, dryRun bool) (string, bool, error) {
	if spec.skills == nil {
		return fmt.Sprintf("  %-13s skipped — none were installed", "skills"), false, nil
	}
	dir, err := spec.skills.resolve(global)
	if err != nil {
		return "", false, err
	}

	removed := make([]string, 0, len(skills))
	for _, skill := range skills {
		home := filepath.Join(dir, skill.name)
		file := filepath.Join(home, "SKILL.md")
		if _, err := os.Stat(file); err != nil {
			continue
		}
		removed = append(removed, skill.name)
		if dryRun {
			continue
		}
		if err := os.Remove(file); err != nil {
			return "", false, fmt.Errorf("remove %s: %w", file, err)
		}
		// os.Remove refuses a directory that still holds something, which is
		// exactly the check wanted here — so its failure is the answer, not an
		// error to report.
		_ = os.Remove(home)
	}
	if !dryRun && len(removed) > 0 {
		// Then the directories the skills sat in: the skills directory itself
		// and the client directory holding it, each only while it is empty. Two
		// levels is as far as the helper ever created anything, and stopping
		// there is what keeps an empty project root out of reach.
		candidate := dir
		for range 2 {
			if err := os.Remove(candidate); err != nil {
				break
			}
			parent := filepath.Dir(candidate)
			if parent == candidate {
				break
			}
			candidate = parent
		}
	}

	var detail string
	switch {
	case len(removed) == 0:
		detail = "none installed"
	case dryRun:
		detail = strings.Join(removed, ", ") + " would be removed"
	default:
		detail = strings.Join(removed, ", ") + " removed"
	}
	return note("skills", dir, detail), len(removed) > 0 && !dryRun, nil
}

func note(what string, path string, detail string) string {
	return fmt.Sprintf("  %-13s %s — %s", what, path, detail)
}

// apply writes wanted to path, unless it is already exactly that.
//
// This is the whole of the helper's idempotence. Because the comparison is
// against the finished text, it holds for every format the two commands touch
// without any of them knowing about it, and it means a re-run cannot reformat a
// file, bump its mtime, or duplicate a block.
//
// A nil wanted says there was nothing to do in the first place, and a wanted
// holding nothing at all says the file should go: a config left holding {}, or
// an instructions file left holding one blank line, is a husk the helper
// created and should take away again.
func apply(path string, wanted *string, dryRun bool) (outcome, error) {
	if wanted == nil {
		return outcomeNothing, nil
	}
	existing, err := readConfig(path)
	if err != nil {
		return outcomeNothing, err
	}
	if existing == *wanted {
		return outcomeCurrent, nil
	}
	if dryRun {
		return outcomeWould, nil
	}
	if strings.TrimSpace(*wanted) == "" {
		if err := os.Remove(path); err != nil {
			return outcomeNothing, fmt.Errorf("remove %s: %w", path, err)
		}
		return outcomeDone, nil
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return outcomeNothing, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(*wanted), 0o600); err != nil {
		return outcomeNothing, fmt.Errorf("write %s: %w", path, err)
	}
	return outcomeDone, nil
}

// readConfig returns the file as it stands, treating one that is not there as
// empty: registering on a machine with no config yet has to work, and removing
// from a file that does not exist is already the outcome asked for.
//
// Every other read failure is fatal. A file that exists but cannot be read must
// never be mistaken for an absent one, because both commands write the merged
// result straight back — so treating an unreadable file as empty would replace
// somebody's whole server list with a file holding only the helper.
func readConfig(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the client table, not from tool input
	switch {
	case err == nil:
		return string(data), nil
	case errors.Is(err, os.ErrNotExist):
		return "", nil
	default:
		return "", fmt.Errorf("read %s: %w", path, err)
	}
}

// client returns the client id names, or an error listing the ones the helper
// knows.
func client(id string) (clientSpec, error) {
	for _, candidate := range clients {
		if candidate.id == id {
			return candidate, nil
		}
	}
	known := make([]string, 0, len(clients))
	for _, candidate := range clients {
		known = append(known, candidate.id)
	}
	return clientSpec{}, fmt.Errorf("unknown client %q — mcp-ai-helper knows: %s", id, strings.Join(known, ", "))
}

// helperCommand is the command a client should run. An absolute path to this
// very binary beats the bare name: a client started from a GUI often has a PATH
// that does not include wherever the helper was installed.
func helperCommand() string {
	path, err := os.Executable()
	if err != nil {
		return serverName
	}
	return path
}

func launchArgs(configPath string) []string {
	if strings.TrimSpace(configPath) == "" {
		return nil
	}
	return []string{"--config", configPath}
}

func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("home directory is not set")
	}
	return home, nil
}

func configPath(spec clientSpec, global bool) (string, error) {
	switch {
	// Claude Code reads a project-scoped .mcp.json, which is the one worth
	// committing; --global falls back to the user-wide config.
	case spec.id == "claude" && !global:
		return scoped{project: ".mcp.json"}.resolve(false)
	case spec.id == "claude":
		return scoped{global: ".claude.json"}.resolve(true)
	// Codex has no project scope: it is always the user config.
	case spec.id == "codex":
		return scoped{global: filepath.Join(".codex", "config.toml")}.resolve(true)
	case spec.id == "opencode" && !global:
		return scoped{project: "opencode.json"}.resolve(false)
	case spec.id == "opencode":
		return scoped{global: filepath.Join(".config", "opencode", "opencode.json")}.resolve(true)
	default:
		return "", fmt.Errorf("unknown client %q", spec.id)
	}
}
