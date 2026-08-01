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
3. When terminal, request mode result or evidence. Use `command action=filter`
   with an include regex or preset before asking for more lines.
4. Abort only that command_id with `command action=abort`, then confirm its
   terminal state with get. Never infer an abort from a client-side timeout.

Execution timeout and MCP wait are different: execution timeout must produce a
durable terminal timeout state; MCP wait only limits one call. Follow a running
workflow through its command_id.

## Evidence

Bound conclusions to retained evidence. Check status, exit_code, truncation, and
omission metadata before claiming success or diagnosing the end of a log. If
output was omitted, retrieve the diagnostic tail or filter retained output. If
omission metadata is absent while output appears cut, say completeness is unknown.

A lost workflow result is not permission to replay mutations. Inspect the same
command_id, then reconcile task and git state once. Report a surface mismatch
when durable status, exact omissions, or terminal state cannot be obtained.

