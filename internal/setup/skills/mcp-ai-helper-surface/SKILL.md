---
name: mcp-ai-helper-surface
description: Diagnose and repair an mcp-ai-helper surface that is missing or stale — a tool the client does not show, guidance naming tools the server does not have, an opt-in layer that is off, or a repository whose task registry is not initialized. Use when a helper tool call fails as unknown, when tool_manifest disagrees with assistant_guidance, when task tools report repair_required, and always before reporting surface_mismatch as a blocker.
---

# When the helper surface is not what you expected

Every other mcp-ai-helper skill ends by telling you to stop with
surface_mismatch. This one is what to do before you do.

## Establish the real surface

`tool_manifest` is the authority on what exists. `assistant_guidance` is the
authority on policy, and it is read from the server's config file, so it can
name tools that an older or newer build does not have.

When the two disagree, trust `tool_manifest`, and say plainly that the guidance
has drifted. Do not call a tool because guidance mentions it, and do not abandon
a tool because guidance omits it. Stale guidance is a config-file problem to
report, never a reason to fall back to shell, direct git, or generic file tools.

## A tool that should exist and does not

1. Call `tool_manifest` and compare it with what the client is showing. A tool
   the server registers but the client does not list means the client's tool
   cache is stale: ask for an MCP client restart.
2. Some tools are opt-in layers that ship off. `task_graph`, `task_context` and
   `task_export` need layers.task_advanced.enabled. The history, blame and
   worktree actions of `git` need layers.git_advanced.enabled.
3. Read the current state with `config_read`, and `config_schema` for what a
   field means. `config_option_set` writes allowlisted scalars only, and the
   layer flags above are not among them: turning `task_advanced` or
   `git_advanced` on means editing the config file directly, because there is no
   full-replacement tool. `feature_list` and `feature_enable` cover the separate
   feature-flag namespace, where this build registers them.
4. `config_reload` re-reads the config without restarting the client, but tool
   visibility changes only when the server process restarts. Enabling a layer
   and expecting the tool in the same session is the usual mistake.

## A repository the task tools do not serve

`task_registry_init` sets a repository registry up through MCP rather than by
hand. Call it with dry_run first and read the actions it reports before letting
it write.

If `task action=current` reports repair_required and asks for the
repair_lean_task_registry_exporter action, the repository has an ActiveTasks
source but no exporter surface. Repair it by adding the canonical exporter
module, declaring the export executable in the lakefile, then verifying with a
build and `task action=current` again. Legacy task files are not fallback
storage: never read or edit registry sources with `file` or `edit` instead.

## Before you call it blocked

Say which tool or action is missing, what `tool_manifest` actually returned, and
which of the steps above you tried. A blocker carrying that evidence is
something a maintainer can act on. A blocker that only reports a missing tool is
a note that the model gave up.
