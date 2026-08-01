package setup

// The skills the helper installs alongside its server.
//
// A skill is a SKILL.md under a directory named for it, in the Agent Skills
// format that Claude Code and OpenCode both read. It differs from the
// instructions block in when it is paid for: the block is loaded into every
// session, so it has to stay short, while a skill's body loads only once the
// client decides it is relevant. That is where the step-by-step workflows
// belong.
//
// The three skills split task orchestration, guarded file work, and durable
// command lifecycle so clients can load only the procedure relevant to a request.

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
		{path: "agents/openai.yaml", body: s.agent},
	}
}

var skills = []skill{
	{name: "mcp-ai-helper-tasks", body: tasksSkill, agent: tasksAgent},
	{name: "mcp-ai-helper-edits", body: editsSkill, agent: editsAgent},
	{name: "mcp-ai-helper-commands", body: commandsSkill, agent: commandsAgent},
}

const tasksAgent = `interface:
  display_name: "mcp-ai-helper Tasks"
  short_description: "Plan and close helper-managed repo tasks"
  default_prompt: "Use $mcp-ai-helper-tasks to select and complete the next repository task safely."
`

const editsAgent = `interface:
  display_name: "mcp-ai-helper Edits"
  short_description: "Safely inspect, edit, verify, and commit files"
  default_prompt: "Use $mcp-ai-helper-edits to make a guarded repository change through the helper."
`

const commandsAgent = `interface:
  display_name: "mcp-ai-helper Commands"
  short_description: "Run and inspect bounded durable commands"
  default_prompt: "Use $mcp-ai-helper-commands to run and inspect a command without losing its durable result."
`

const tasksSkill = "---\n" +
	"name: mcp-ai-helper-tasks\n" +
	"description: Select, execute, and close repository tasks through mcp-ai-helper with live surface discovery, bounded context, explicit ownership, acceptance gates, and one atomic run action=workflow finalization. Use when starting or finishing task-backed work, choosing the next task, handling a blocker or surface mismatch, or working in a dirty repository served by mcp-ai-helper.\n" +
	"---\n" +
	"\n" +
	"# Task work through mcp-ai-helper\n" +
	"\n" +
	"Treat the task registry behind the `task` tool as canonical. Never parse or mutate\n" +
	"task registry files directly.\n" +
	"\n" +
	"Call `assistant_guidance` and `tool_manifest` once per session. Guidance is the\n" +
	"current policy; the manifest is the current surface. Never infer hidden tools.\n" +
	"\n" +
	"## Order of work\n" +
	"\n" +
	"1. Call `task action=current`, even when a task id was supplied. Select only an\n" +
	"   executable task whose model_level matches the current model; never execute a\n" +
	"   blocked parent or epic.\n" +
	"2. Call `task action=get` for the chosen id. If the manifest exposes\n" +
	"   `task_context`, use it as an additional bounded packet. Otherwise continue\n" +
	"   with task get and narrow reads; do not invent the tool.\n" +
	"3. Call `git action=status`. Preserve dirty files outside task ownership.\n" +
	"4. State the selected task, exact owned_files, forbidden files, acceptance\n" +
	"   criteria, minimal gate, and atomic close path before editing.\n" +
	"5. Read only relevant ranges with `file action=read`, `file action=read_many`,\n" +
	"   `file action=search`, and `file action=snapshot`.\n" +
	"6. Call `run action=schema`, then close in one `run action=workflow` call.\n" +
	"\n" +
	"## Closing a task\n" +
	"\n" +
	"Use only workflow steps returned by the schema. The current step set includes\n" +
	"`guarded_replace`, `command`, `task_batch_upsert`, `task_transition`,\n" +
	"`git_commit_owned`, and `git_prepare_task_worktree`.\n" +
	"\n" +
	"Explicitly order edits, formatting, focused checks, risk-based wider gates,\n" +
	"`task_transition`, then `git_commit_owned` over exactly `owned_files`. Make every\n" +
	"check depend on all relevant edits so a DAG cannot validate stale code.\n" +
	"\n" +
	"The done transition and owned-files commit belong in the same workflow. A failed\n" +
	"or running check, timeout, missing result, skipped gate, or separate status\n" +
	"commit is not completion. If a workflow loses its result, inspect its command_id\n" +
	"and reconcile task and git state once; never replay mutations blindly.\n" +
	"\n" +
	"## When something is missing\n" +
	"\n" +
	"Stop with `surface_mismatch` or `blocked` when a required tool, action, workflow\n" +
	"step, ownership fact, or status is absent. Do not route around the helper with\n" +
	"shell, direct git, generic file tools, or manual task-file edits.\n" +
	"\n" +
	"Obey repository instructions that require command execution or polling to be\n" +
	"delegated. If the required delegate lacks the helper surface, report the blocker.\n" +
	"On a failed check, inspect actual state once, form a new hypothesis, and reuse\n" +
	"its command_id instead of rerunning unchanged work.\n" +
	"\n"

const editsSkill = "---\n" +
	"name: mcp-ai-helper-edits\n" +
	"description: Inspect, change, verify, and commit repository files exclusively through mcp-ai-helper using bounded reads, hash-guarded edits, explicit ownership, and workflow gates. Use for any file or git change in a repository served by mcp-ai-helper, especially dirty worktrees, conflicting edits, new files, or atomic task finalization.\n" +
	"---\n" +
	"\n" +
	"# Editing through mcp-ai-helper\n" +
	"\n" +
	"Call `assistant_guidance` and `tool_manifest` before relying on an action. Keep\n" +
	"context bounded and ownership explicit. Never substitute shell, direct file APIs,\n" +
	"or direct git when the helper surface is required.\n" +
	"\n" +
	"## Read\n" +
	"\n" +
	"- Use `file action=read` with offset and limit for one range.\n" +
	"- Use `file action=read_many` for up to eight known files.\n" +
	"- Use `file action=search` with a narrow path and capped matches.\n" +
	"- Use `file action=list` for structured directory discovery.\n" +
	"- Use `file action=snapshot` immediately before a guarded change.\n" +
	"\n" +
	"## Change\n" +
	"\n" +
	"For an existing file:\n" +
	"\n" +
	"1. Read the intended region.\n" +
	"2. Snapshot the file.\n" +
	"3. Call `edit action=replace` with expected_hash and an old span that occurs once.\n" +
	"\n" +
	"Use old_b64 and new_b64 when transport escaping is risky. If the hash conflicts,\n" +
	"re-read and re-snapshot; never remove the guard.\n" +
	"\n" +
	"Use `edit action=write` for a whole or new file, with expected_hash when it\n" +
	"already exists. Before a new-file task that requires atomic completion, verify\n" +
	"`run action=schema` exposes a write/create step. If it does not, report\n" +
	"surface_mismatch; do not create outside the workflow and claim atomic closure.\n" +
	"\n" +
	"## Verify\n" +
	"\n" +
	"Start a narrow check with `command action=run`. Retain its command_id. Use\n" +
	"`command action=get` and `command action=filter` rather than rerunning it. Run\n" +
	"wider gates only for a concrete regression risk.\n" +
	"\n" +
	"## Commit\n" +
	"\n" +
	"Inspect with `git action=status` and `git action=diff`. Use `git action=commit`\n" +
	"only over explicit owned files. For task-backed changes, prefer\n" +
	"`run action=workflow` with `git_commit_owned` after every gate and the task\n" +
	"transition; include exactly owned_files and preserve unrelated dirty files.\n" +
	"\n" +
	"`git action=log`, `git action=blame`, and `git action=prepare_task_worktree`\n" +
	"provide history and isolation without shell.\n" +
	"\n"

const commandsSkill = "---\n" +
	"name: mcp-ai-helper-commands\n" +
	"description: Run, monitor, filter, abort, and reconcile durable commands through mcp-ai-helper without rerunning work or overclaiming truncated evidence. Use when a command, test, build, workflow, or long-running check returns running, times out, truncates output, provides a command_id, or needs compact diagnostic evidence.\n" +
	"---\n" +
	"\n" +
	"# Durable commands through mcp-ai-helper\n" +
	"\n" +
	"Call `assistant_guidance` and `tool_manifest` once per session. Treat command_id\n" +
	"as the durable identity of one execution.\n" +
	"\n" +
	"## Lifecycle\n" +
	"\n" +
	"1. Start once with `command action=run` or `run action=pipeline`. Set an execution\n" +
	"   timeout for the work and an MCP wait budget for the initial response.\n" +
	"2. If status is running, the call handed work off; it did not fail. Use\n" +
	"   `command action=get` with the same command_id and mode status. Do not rerun.\n" +
	"3. When terminal, request mode result or evidence. Use `command action=filter`\n" +
	"   with an include regex or preset before asking for more lines.\n" +
	"4. Abort only that command_id with `command action=abort`, then confirm its\n" +
	"   terminal state with get. Never infer an abort from a client-side timeout.\n" +
	"\n" +
	"Execution timeout and MCP wait are different: execution timeout must produce a\n" +
	"durable terminal timeout state; MCP wait only limits one call. Follow a running\n" +
	"workflow through its command_id.\n" +
	"\n" +
	"## Evidence\n" +
	"\n" +
	"Bound conclusions to retained evidence. Check status, exit_code, truncation, and\n" +
	"omission metadata before claiming success or diagnosing the end of a log. If\n" +
	"output was omitted, retrieve the diagnostic tail or filter retained output. If\n" +
	"omission metadata is absent while output appears cut, say completeness is unknown.\n" +
	"\n" +
	"A lost workflow result is not permission to replay mutations. Inspect the same\n" +
	"command_id, then reconcile task and git state once. Report a surface mismatch\n" +
	"when durable status, exact omissions, or terminal state cannot be obtained.\n" +
	"\n"
