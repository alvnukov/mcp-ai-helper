---
name: mcp-ai-helper-commands
description: Run, monitor, filter, abort, and reconcile durable commands through mcp-ai-helper without rerunning work or overclaiming truncated evidence. Use when a command, test, build, workflow, or long-running check returns running, times out, truncates output, provides a command_id, needs batched read-only probes, or needs compact diagnostic evidence.
---

# Durable commands through mcp-ai-helper

A command keeps its output, its exit status, and its identity: the
`command_id`. Everything below follows from treating that identity as
durable.

## Lifecycle

1. Start once with `command action=run` or `run action=pipeline`. Set an
   execution timeout for the work and an MCP wait budget for the first
   response.
2. A running status handed work off; it is not a failure. Wait with
   `command action=get` and `wait_seconds` on the same command_id — get
   blocks until the command finishes, while a sleep inside a shell command
   spends a turn to learn less and leaves a waiter behind when the estimate
   is wrong.
3. When terminal, request mode result or evidence. Narrow with
   `command action=filter` and an include regex or preset before asking for
   more lines.
4. Abort with `command action=abort` on that command_id, then confirm the
   terminal state with get. A client-side timeout is not an abort.

Execution timeout and MCP wait are different budgets: the timeout produces
a durable terminal state, the wait limits one call. Follow a running
workflow through its command_id.

## What a command costs

A command is a whole turn, and spending one per fact is the expensive
habit: in the durable log, 41% of commands finished in under a second with
nothing wrong, and 48% returned three lines or fewer.

1. Batch read-only probes: `run action=workflow` takes a list of `command`
   steps, and `on_failure: "continue"` keeps later probes running when one
   fails, so several questions cost one turn.
2. While iterating, run the narrowest check that can fail — one package,
   one test by name. The repository-wide gate runs once, before the commit.
   `command action=health` is the quick build/vet/test shape.
3. A command that passed stays passed. When a reply carries `previous`,
   the command already ran here within the hour, and `previous.same_output`
   means the repeat produced byte-identical output and taught nothing.
4. Verifying a compiled artifact means building it in the same command
   that tests it, or you are testing the previous binary.
5. Read and search with `file action=read` and `file action=search`;
   write with `edit action=replace` or `edit action=write`. Shell reads
   are unbounded, and writing source through the shell is refused.

## Values as data

Compose commands from data instead of nested quotes:

- `env` injects `$NAME`, always double-quoted.
- `vars` substitute `{{{{NAME}}` into command and stdin before the shell
  parses anything; write `{{{{{{{{` for a literal `{{{{`.
- `stdin` or `stdin_var` pipe content in place of a heredoc.
- `secret_handles` resolve to `{{{{NAME}}`, `$NAME` and
  `$HELPER_SECRET_NAME` while staying masked in output.

An unknown `{{{{NAME}}` fails closed and lists the known names, so a typo
in a var is an error you can read rather than a command that almost did
the right thing. Substitution never touches repo_path or other path
arguments.

## Evidence

Bound conclusions to retained evidence: status, exit_code, truncation, and
omission metadata before claiming success or diagnosing the end of a log.
When output was omitted, retrieve the diagnostic tail or filter the
retained output; when omission metadata is absent but output looks cut,
say completeness is unknown.

A zero exit code is not a passing check: a command exits with the status
of its pipeline's last stage, so `go test ./... | tail -40` reports success
whatever the tests did. When the reply carries `failure_markers`, the
output said it failed and the exit code did not — believe the output.

A lost workflow result is not permission to replay mutations. Inspect the
same command_id, then reconcile task and git state once. When durable
status, exact omissions, or terminal state cannot be obtained, report a
surface mismatch.
