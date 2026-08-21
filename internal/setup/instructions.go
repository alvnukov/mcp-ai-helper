package setup

import (
	"fmt"
	"strings"
)

// The block the helper keeps in the file its client reads for instructions.
//
// Registering the server only makes the tools available. What decides whether a
// model reaches for them, rather than falling back to grep, is whether the
// project's instruction file says they are worth reaching for. That file is
// CLAUDE.md for Claude Code and AGENTS.md for Codex and OpenCode; the caller
// picks, this file only edits.
//
// The block is delimited by HTML comments, which is what makes it removable:
// the helper can find its own section in a file a human otherwise owns, and
// replace or excise exactly that. Claude Code strips block-level HTML comments
// before the file enters the model's context, so the markers themselves cost
// nothing to keep.

const (
	blockStart = "<!-- mcp-ai-helper:start -->"
	blockEnd   = "<!-- mcp-ai-helper:end -->"
)

// blockBody is what the helper asks an agent to know about itself.
//
// Deliberately short. This text is loaded in full at the start of every session,
// so it earns its place only by answering the question a model actually has —
// should I use this instead of what I would have done? — rather than by
// restating the tool schemas, which the client already has. Past that it
// carries the few working patterns a model trips over when nothing else is
// loaded, because no other text in the session is guaranteed to be read: the
// guidance tool and the skills only speak when the model goes to them. The
// rules that do change live behind assistant_guidance, which is read at run
// time from the server's config and is therefore current in a way this block
// cannot be.
const blockBody = "" +
	`## mcp-ai-helper

This project is served by the ` + "`mcp-ai-helper`" + ` MCP server. Call
` + "`assistant_guidance`" + ` before anything else: the rules it returns are read from
the server's own config, so they are current in a way this block cannot be, and
they take precedence over habit.

Most of the surface is a handful of tools dispatching on an ` + "`action`" + `:

- ` + "`file`" + ` — read, read_many, list, search, snapshot
- ` + "`edit`" + ` — replace, write
- ` + "`command`" + ` — run, get, filter, list, abort, cleanup, health
- ` + "`git`" + ` — status, diff, commit, log, blame, prepare_task_worktree
- ` + "`task`" + ` — current, get, list, search, upsert, set_status, batch_upsert
- ` + "`run`" + ` — pipeline, workflow, schema

Prefer them over shell, editor, direct git and generic web tools. They return
ids, hashes and bounded fragments rather than whole files and whole logs, which
is the point: the same answer for a fraction of the context.

Four patterns keep the work honest when nothing else is loaded:

- ` + "`task action=current`" + ` before repo work: one fitting task, not a blocked parent.
- Search before you read: ` + "`file action=search`" + ` with context, then ` + "`read`" + ` ranges.
- A command returns ` + "`command_id`" + `: wait with ` + "`get`" + `+` + "`wait_seconds`" + `, narrow with ` + "`filter`" + `, stop with ` + "`abort`" + `; never rerun or sleep-poll.
- Before believing output, read ` + "`exit_code`" + `, ` + "`truncated`" + `, ` + "`failure_markers`" + `: a
  green tail can hide a failed command; a truncated log is not evidence.

The procedures live in skills, which cost nothing until they are loaded:
` + "`mcp-ai-helper-tasks`" + `, ` + "`mcp-ai-helper-edits`" + `, ` + "`mcp-ai-helper-commands`" + `,
` + "`mcp-ai-helper-web`" + ` and ` + "`mcp-ai-helper-surface`" + `. Load the one that matches the
work rather than reconstructing the protocol from the tool schemas.

Editing is guarded. Read the file, take ` + "`file action=snapshot`" + ` for its hash, then
hand that hash to ` + "`edit action=replace`" + ` — the write fails rather than silently
landing on a file that moved underneath you.

Finish a task in one ` + "`run action=workflow`" + ` call. The edits, the focused check, the
task status transition and the owned-files commit belong in the same call; a
commit followed by a separate status commit means the task is not done.

If these tools are not visible, call ` + "`tool_manifest`" + ` and follow
` + "`mcp-ai-helper-surface`" + `: some are opt-in layers that ship off, and a client's
tool cache goes stale. Do not substitute shell, filesystem or direct git
fallbacks.`

// block is the block as it should appear, markers included.
func block() string {
	return fmt.Sprintf("%s\n%s\n%s\n", blockStart, blockBody, blockEnd)
}

// withBlock returns existing with the helper's block present and current.
//
// An existing block is replaced where it stands rather than moved to the end, so
// upgrading the helper does not shuffle a file somebody else maintains.
func withBlock(existing string) (string, error) {
	wanted := block()
	start, end, found, err := span(existing)
	if err != nil {
		return "", err
	}
	if !found {
		if strings.TrimSpace(existing) == "" {
			return wanted, nil
		}
		return strings.TrimRight(existing, "\n\t ") + "\n\n" + wanted, nil
	}
	return existing[:start] + wanted + existing[end:], nil
}

// withoutBlock returns existing without the helper's block, or nil when there
// was no block to take out — the caller then leaves the file entirely alone.
func withoutBlock(existing string) (*string, error) {
	start, end, found, err := span(existing)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	before := strings.TrimRight(existing[:start], "\n\t ")
	after := strings.TrimLeft(existing[end:], "\n")
	var result string
	switch {
	case before == "" && after == "":
		result = ""
	case before == "":
		result = after
	case after == "":
		result = before + "\n"
	default:
		result = before + "\n\n" + after
	}
	return &result, nil
}

// span reports where the helper's block sits, as a byte range covering the
// markers and the newline after the closing one.
//
// A file with one marker and not the other has been edited by hand into a state
// where any guess about the intended boundary could destroy text. That is
// refused rather than guessed at.
func span(text string) (int, int, bool, error) {
	start := strings.Index(text, blockStart)
	end := strings.Index(text, blockEnd)
	switch {
	case start < 0 && end < 0:
		return 0, 0, false, nil
	case start >= 0 && end > start:
		end += len(blockEnd)
		if strings.HasPrefix(text[end:], "\n") {
			end++
		}
		return start, end, true, nil
	default:
		return 0, 0, false, fmt.Errorf(
			"the instructions file has a damaged mcp-ai-helper block: it must contain %q followed by %q, or neither — fix it by hand and run this again",
			blockStart, blockEnd)
	}
}
