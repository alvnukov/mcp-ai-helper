package setup

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
)

// The skills the helper installs alongside its server.
//
// A skill is a SKILL.md under a directory named for it, in the Agent Skills
// format that Claude Code and OpenCode both read. It differs from the
// instructions block in when it is paid for: the block is loaded into every
// session, so it has to stay short, while a skill's body loads only once the
// client decides it is relevant. That is where the step-by-step workflows
// belong.
//
// The skills split task orchestration, guarded file work, durable command
// lifecycle, web research, and surface recovery so clients can load only the
// procedure relevant to a request.
//
// The text lives in skills/ as the markdown and YAML that are installed, rather
// than as Go string literals, so that what a model will read can be reviewed as
// what a model will read. Adding a skill is a directory plus one line in
// skillNames.

//go:embed skills
var skillFS embed.FS

// skillNames is the installed set, in the order the install report lists them.
//
// The order is deliberate rather than alphabetical: a model scanning the list
// should meet task orchestration first, because every other skill is something
// it does in the middle of a task.
var skillNames = []string{
	"mcp-ai-helper-tasks",
	"mcp-ai-helper-edits",
	"mcp-ai-helper-commands",
	"mcp-ai-helper-web",
	"mcp-ai-helper-surface",
}

// skill is one installed skill with its model instructions and Codex UI metadata.
type skill struct {
	// name is the directory name, invocation name, and frontmatter name.
	name  string
	body  string
	agent string
}

type skillFile struct {
	path string
	body string
}

func (s skill) files() []skillFile {
	return []skillFile{
		{path: "SKILL.md", body: s.body},
		{path: path.Join("agents", "openai.yaml"), body: s.agent},
	}
}

var skills = loadSkills()

func loadSkills() []skill {
	loaded := make([]skill, 0, len(skillNames))
	for _, name := range skillNames {
		loaded = append(loaded, skill{
			name:  name,
			body:  embedded(name, "SKILL.md"),
			agent: embedded(name, path.Join("agents", "openai.yaml")),
		})
	}
	return loaded
}

// embedded panics rather than returning an error: the files are compiled into
// the binary, so a name in skillNames with nothing behind it is a mistake in
// this package and not a condition any caller could act on. The tests reach the
// panic on the first run after it is introduced.
func embedded(name string, file string) string {
	data, err := skillFS.ReadFile(path.Join("skills", name, file))
	if err != nil {
		panic(fmt.Sprintf("skill %s is missing %s: %v", name, file, err))
	}
	return string(data)
}

// embeddedSkillDirs reports the directories actually compiled in, so a test can
// catch a skill added to skills/ and never registered in skillNames — which
// would otherwise be a file that exists, reviews fine, and is never installed.
func embeddedSkillDirs() ([]string, error) {
	entries, err := fs.ReadDir(skillFS, "skills")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}
