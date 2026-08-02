---
name: mcp-ai-helper-commands
description: Run, monitor, filter, abort, and reconcile durable commands through mcp-ai-helper without rerunning work or overclaiming truncated evidence. Use when a command, test, build, workflow, or long-running check returns running, times out, truncates output, provides a command_id, or needs compact diagnostic evidence.
---

# Durable commands through mcp-ai-helper

Call `assistant_guidance` and `tool_manifest` once per session. Treat command_id
as the durable identity of one execution.

## Lifecycle

1. Start once with `command action=run` or `run action=pipeline`. Set an execution
   timeout for the work and an MCP wait budget for the initial response.
2. If status is running, the call handed work off; it did not fail. Use
   `command action=get` with the same command_id and mode status. Do not rerun.
   To wait, pass `wait_seconds` to get: it blocks until the command finishes.
   Never sleep inside a shell command to wait for one — that spends a turn per
   poll to learn what get would have handed you, and leaves a waiter behind when
   the estimate is wrong.
3. When terminal, request mode result or evidence. Use `command action=filter`
   with an include regex or preset before asking for more lines.
4. Abort only that command_id with `command action=abort`, then confirm its
   terminal state with get. Never infer an abort from a client-side timeout.

Execution timeout and MCP wait are different: execution timeout must produce a
durable terminal timeout state; MCP wait only limits one call. Follow a running
workflow through its command_id.

## What a command costs

A command is a whole turn. Spending one on a single fact is the most expensive
habit available, and it is the common one: in the durable log, 41% of commands
finished in under a second with nothing wrong and 48% returned three lines or
fewer.

1. Batch read-only probes. `run action=workflow` takes a list of `command` steps,
   and `on_failure: "continue"` keeps the rest running when one fails, so several
   questions cost one turn. Never issue `which`, `ls`, `test -e`, `--version` or
   `git status` as a call of its own when another probe is coming.
2. While iterating, run the narrowest check that can fail: one package, one test
   by name. Run the repository-wide gate once, before the commit — not after each
   edit. `command action=health` is the quick build/vet/test shape.
3. Do not rerun a command that passed. If the reply carries `previous`, this
   command already ran here within the hour; `previous.same_output` means the
   repeat produced byte-identical output and taught you nothing.
4. Verifying a change to a compiled artifact means building it in the same
   command that tests it. Otherwise you are testing the previous binary.
5. Read and search with `file action=read` and `file action=search`, not with
   `cat`, `sed -n` or `grep` — shell reads are unbounded and do not count as
   having read the file. Write with `edit action=replace` or `edit action=write`:
   writing source through the shell is refused.

## Evidence

Bound conclusions to retained evidence. Check status, exit_code, truncation, and
omission metadata before claiming success or diagnosing the end of a log. If
output was omitted, retrieve the diagnostic tail or filter retained output. If
omission metadata is absent while output appears cut, say completeness is unknown.

A zero exit code is not a passing check. A command exits with the status of the
last stage of its pipeline, so `go test ./... | tail -40` reports success
whatever the tests did. When the reply carries `failure_markers`, the output
said it failed and the exit code did not: believe the output.

A lost workflow result is not permission to replay mutations. Inspect the same
command_id, then reconcile task and git state once. Report a surface mismatch
when durable status, exact omissions, or terminal state cannot be obtained.

