---
name: mcp-ai-helper-workflows
description: Build, run, and follow one-call mcp-ai-helper workflows, where guarded edits, checks, the task transition, and the owned-files commit are steps of a single run action=workflow request. Use when closing work atomically, when a reply returns running with a workflow_id, when previewing or diagnosing a workflow, or when choosing between one workflow and step-by-step calls.
---

# One-call workflows through mcp-ai-helper

A workflow carries an entire change in one request: edits, checks, task
transition, commit. Execution is detached — steps keep running server-side
when the call returns early, so a long workflow never loses its result to a
client timeout.

## Before you build

Come with the context that shapes steps: the task (mcp-ai-helper-tasks), the
file regions to edit, and a snapshot hash per edited file. Call
`run action=schema` and build against what it returned, not memory.

Steps are objects — `{"id", "tool", "args"}` — in the `steps` array, never
JSON strings. `guarded_replace`, `write_file`, and `git_commit_owned` exist
only inside steps. Set `preview: true` for a dry run: the reply lists what
would execute, nothing runs. Reach for it whenever the shape is new.

## Steps, conditions, probes

Step tools: `command`, `guarded_replace`, `write_file`, `task_batch_upsert`,
`task_transition`, `git_commit_owned`, `git_prepare_task_worktree`. Fields
every step carries:

- `depends_on` — same-file order the engine infers; declare it for cross-file
  and cross-tool order, above all edit → check → transition → commit.
- `if` — a deterministic condition: `steps.<id>.status == ok`,
  `steps.<id>.exit_code != 0`, `steps.<id>.output_contains text`,
  `file_exists path`, `tasks.<id>.status == todo`, `changed_files contains
  path`, joined with `&&`, `||`, `!`.
- `on_failure` — `stop` by default. `continue` marks a probe: a step whose
  failure feeds a later `if`. A gate — a check that must hold — keeps the
  default, so its failure stops everything downstream and no commit lands.

## The closing shape

Edits, formatting, a focused check, a wider gate only on concrete regression
risk, then `task_transition` guarded by `if: steps.<check>.status == ok`, and
`git_commit_owned` over exactly `owned_files` depending on that transition.
The failure path transitions the task to `blocked` under
`steps.<check>.status != ok` and has no commit step: the commit lands in the
same run as the green gate or the task stays open.

With `current_task_id`, the workflow owns the lifecycle through
`task_on_start` / `task_on_success` / `task_on_failure`; a successful explicit
`task_transition` of that same task takes precedence over the deferred write.

## Follow a running workflow

The call waits up to `mcp_wait_seconds` (600 by default and cap) for the
final result; past the budget it answers `running` plus a `workflow_id` while
the steps keep going. Follow with `run action=workflow_status` and
`wait_seconds`, which blocks until the run finishes.

A `workflow_id` lives for the server process. An unknown id after a restart
means the run finished or died with the process: reconcile once —
`git action=status` for the commit, `command action=list` for the last steps —
and decide from actual state. Replaying the whole workflow is how a
half-finished one becomes twice-finished.

## Values ride as data

Workflow `env` and `vars` reach every `command` step, a step's own map
merging over them; `{{{{NAME}}` substitutes into commands, stdin, and
`write_file` content. Pipe payloads with `stdin` / `stdin_var`, keep secrets
in `secret_handles`. A quoting need is a missing field —
mcp-ai-helper-commands carries the full rule.

## When a workflow fails

Preflight rejects a malformed call before any step runs, so a fix costs
nothing. A failed step stops the workflow before the commit: read that step's
result, form one new hypothesis, snapshot the touched files again — their
hashes moved — and send a corrected workflow. A stuck or lost run goes
through `workflow_status` first; the command records underneath hold the
evidence.
