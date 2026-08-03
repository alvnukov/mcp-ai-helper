package mcp

import basemcp "github.com/mark3labs/mcp-go/mcp"

// mcp-go's NewTool seeds all four annotation fields with non-nil defaults
// (ReadOnly=false, Destructive=true, Idempotent=false, OpenWorld=true). A hint
// nobody chose is therefore indistinguishable on the wire from one chosen on
// purpose, and a client deciding whether a call needs confirmation cannot tell
// a claim from a default. Every tool this server registers states all four,
// through one of the profiles below.
//
// The hints answer, in order: does the call leave every store it touches
// unchanged; may it overwrite or drop state that was already there; does
// repeating it with the same arguments add nothing further; does it reach a
// system beyond this machine.
func annotate(readOnly, destructive, idempotent, openWorld bool) basemcp.ToolOption {
	return func(tool *basemcp.Tool) {
		basemcp.WithReadOnlyHintAnnotation(readOnly)(tool)
		basemcp.WithDestructiveHintAnnotation(destructive)(tool)
		basemcp.WithIdempotentHintAnnotation(idempotent)(tool)
		basemcp.WithOpenWorldHintAnnotation(openWorld)(tool)
	}
}

var (
	// readsLocal answers from this machine and writes nothing. It is the only
	// profile a read-only auto-approval policy can accept.
	readsLocal = annotate(true, false, true, false)

	// readsRemote answers from a remote system and writes nothing.
	readsRemote = annotate(true, false, true, true)

	// ensuresLocal brings local state to the shape the caller asked for
	// without overwriting anything, and converges: a second identical call
	// changes nothing more. The task-registry readers sit here rather than in
	// readsLocal because listing notes auto-heals them — scanNotes renames a
	// file whose name disagrees with its frontmatter id — so calling them
	// read-only would be a false claim.
	ensuresLocal = annotate(false, false, true, false)

	// addsLocal appends local state; each call leaves one more record.
	addsLocal = annotate(false, false, false, false)

	// setsLocal writes local state to a value the caller names, overwriting
	// what was there, and converges on repeat.
	setsLocal = annotate(false, true, true, false)

	// rewritesLocal overwrites local state, and what it does depends on the
	// state it finds.
	rewritesLocal = annotate(false, true, false, false)

	// addsRemote reaches a remote system and only appends — there, or to the
	// local cache holding what it brought back.
	addsRemote = annotate(false, false, false, true)

	// setsRemote writes a remote field to a value the caller names.
	setsRemote = annotate(false, true, true, true)

	// rewritesRemote changes remote state in a way that depends on the state
	// it finds.
	rewritesRemote = annotate(false, true, false, true)

	// runsCommands executes what the caller asks on this host: any local
	// effect the command has, and any host the command chooses to reach. Same
	// four values as rewritesRemote, for a different reason.
	runsCommands = annotate(false, true, false, true)
)
