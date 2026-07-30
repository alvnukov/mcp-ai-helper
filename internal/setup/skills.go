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
// The two skills split along the two halves of a piece of repo work — deciding
// what to do, and doing it — because that is how the questions arrive, and
// because a client picks between them on description alone.

// skill is one skill, as the file it becomes.
type skill struct {
	// name is the directory name, which is also what the client uses to invoke
	// it, and which the frontmatter name has to repeat.
	name string
	body string
}

var skills = []skill{
	{name: "mcp-ai-helper-tasks", body: tasksSkill},
	{name: "mcp-ai-helper-edits", body: editsSkill},
}

const tasksSkill = `---
name: mcp-ai-helper-tasks
description: Choose and close repo work through the mcp-ai-helper task registry - read the current task, focus it, then bundle edits, checks, the status transition and the owned-files commit into a single run action=workflow call. Use when starting work in a repo served by mcp-ai-helper, when asking what to do next, when a task is blocked, or when finishing one.
---

# Task work through mcp-ai-helper

The task registry is canonical. Its state lives behind the ` + "`task`" + ` tools, not in
files you can read: parsing or regex-mutating registry source is how a registry
and its repo drift apart, and there is no fallback path that is allowed to do
it.

Call ` + "`assistant_guidance`" + ` first, once per session. It is read from the server's
config, so it is the current statement of the rules; this skill is the shape of
the work, not the policy.

## Order of work

1. ` + "`task action=current`" + `. Start here even when you think you know the task. It
   reports what is executable now, and picks up the model_level a task expects.
   Do not execute a blocked parent or epic task.
2. ` + "`task_context`" + ` for the chosen task. This is the focused view: acceptance
   criteria, owned files, the gate to run. Use ` + "`task_graph`" + ` only when the
   question is about dependencies or parentage.
3. Say out loud, before editing: the selected task, the exact owned_files, the
   files that are off-limits, the acceptance criteria, the minimal gate, and how
   the task will be closed.
4. Inspect with ` + "`file action=read`" + ` and ` + "`file action=search`" + `, narrowly. See the
   mcp-ai-helper-edits skill for the guarded edit contract.
5. Close in one ` + "`run action=workflow`" + ` call.

## Closing a task

One workflow call carries the whole finish. Its steps are
` + "`guarded_replace`" + `, ` + "`command`" + `, ` + "`task_batch_upsert`" + `, ` + "`task_transition`" + `,
` + "`git_commit_owned`" + ` and ` + "`git_prepare_task_worktree`" + ` - run ` + "`run action=schema`" + `
for their exact arguments rather than guessing.

Order the steps: edits, then formatting, then the focused check, then the task
transition, then the commit over ` + "`owned_files`" + `. Passing ` + "`current_task_id`" + ` lets
the workflow move the task to in_progress on start and to blocked on failure by
itself.

A code commit followed by a separate status commit means the task is not done.
The point of the single call is that a failing gate leaves the task open rather
than leaving a repo that claims work it did not finish.

Set ` + "`preview`" + ` to true to see the steps that would run without running them. Use
it when the workflow is long enough that a mistake would be expensive.

## When something is missing

If a tool you need is absent, denied, or too vague to use safely, stop and say
so as surface_mismatch or blocker. Do not route around it with shell, direct
git, or generic file tools - a task closed that way is not closed.

If ` + "`task action=current`" + ` reports repair_required, follow the repair action it
names. Do not fall back to reading legacy task files.

On a failed check: inspect the actual state once, form one new hypothesis, run
the next minimal check. Re-running the same failing command without new
information is the loop to avoid.
`

const editsSkill = `---
name: mcp-ai-helper-edits
description: Read, change, check and commit repo files through mcp-ai-helper - file action=snapshot for a hash, edit action=replace guarded by it, command action=run with retained bounded output, git action=commit over owned files. Use instead of shell, editor, cat, grep or direct git whenever working in a repo served by mcp-ai-helper.
---

# Editing through mcp-ai-helper

These tools exist to keep two things true at once: the change is safe, and the
context stays small. Every one of them returns ids, hashes, offsets and bounded
fragments rather than whole files and whole logs.

## Reading

- ` + "`file action=read`" + ` with ` + "`offset`" + ` and ` + "`limit`" + ` - one file, numbered lines.
  Read the range you need, not the file.
- ` + "`file action=read_many`" + ` - up to eight files in one call. Cheaper than eight
  calls when you already know the list.
- ` + "`file action=search`" + ` - text under a directory, capped by ` + "`max_matches`" + `.
- ` + "`file action=list`" + ` - structured directory listing.

## Changing

An edit is guarded by a hash, and the guard is the point:

1. ` + "`file action=read`" + ` the region you intend to change.
2. ` + "`file action=snapshot`" + ` for the file's current hash and size.
3. ` + "`edit action=replace`" + ` with that ` + "`expected_hash`" + `, an ` + "`old`" + ` span that occurs
   exactly once, and its ` + "`new`" + ` text.

If the file moved underneath you the replace fails instead of landing on text
that is no longer there. Re-snapshot and redo the edit; never drop the guard to
make a failure go away.

Use ` + "`old_b64`" + ` and ` + "`new_b64`" + ` when the text has characters that transport badly.
Use ` + "`edit action=write`" + ` for a whole file - a new one, or a rewrite - and pass
` + "`expected_hash`" + ` there too whenever the file already exists.

## Checking

` + "`command action=run`" + ` runs a narrow command and retains its output. What comes
back is bounded; the rest stays on the server under a ` + "`command_id`" + `.

Reach for that id rather than re-running:

- ` + "`command action=get`" + ` with ` + "`mode`" + ` of status, result, tail or evidence
- ` + "`command action=filter`" + ` to search inside the retained output
- ` + "`command action=abort`" + ` for one still running, ` + "`command action=list`" + ` for what
  is retained

Run the focused gate for what you touched, not the whole suite - unless the
change carries real regression risk.

## Committing

` + "`git action=status`" + ` and ` + "`git action=diff`" + ` to see the state, ` + "`git action=commit`" + `
to record it. Commit the owned files for the task and nothing else.

Inside a workflow this is the ` + "`git_commit_owned`" + ` step, which enforces the
owned-files boundary for you and belongs in the same ` + "`run action=workflow`" + ` call
as the edits and the check. See the mcp-ai-helper-tasks skill for that shape.

` + "`git action=log`" + `, ` + "`git action=blame`" + ` and ` + "`git action=diff`" + ` answer history
questions without a shell. ` + "`git action=prepare_task_worktree`" + ` sets up an
isolated worktree when a task should not share the working tree.
`
