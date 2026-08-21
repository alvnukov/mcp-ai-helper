package setup

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// The MCP server entry itself: the stanza that makes the helper's tools exist.
//
// Three clients, three shapes. The JSON clients are served by the surgical
// editor in jsonedit.go — their configs hold far more than the helper's own
// entry, so only these entry builders decide content, and the splice under it
// decides bytes. Codex keeps TOML, whose one writer (BurntSushi) round-trips
// the whole file; that is its own contract, unchanged here.

// mergeClaudeJSON registers the helper in a Claude Code config: "command"
// with the binary, "args" with the launch flags, under "mcpServers".
//
// The binary and the --config pair are the helper's to manage. Anything else
// in the entry — env settings, a timeout — and every other byte of the file
// survives the registration untouched.
func mergeClaudeJSON(existing, command, configPath string) (string, error) {
	return jsonMerge(existing, "mcpServers", func(entryRaw, indent string) string {
		managed := []managedMember{
			{key: "command", text: compactValue(command)},
			{key: "args", text: compactValue(mergeArgs(argvFromMember(entryRaw, "args"), configPath))},
		}
		return spliceEntry(entryRaw, indent, managed)
	})
}

// mergeOpenCodeJSON registers the helper in an OpenCode config: the whole
// argv in "command", under "mcp".
//
// OpenCode keeps the executable and its flags in one array, so re-pinning the
// binary has to keep the flags a user added by hand: the old binary and any
// --config pair are replaced, everything after them — a --repo, say — keeps
// its place.
func mergeOpenCodeJSON(existing, command, configPath string) (string, error) {
	return jsonMerge(existing, "mcp", func(entryRaw, indent string) string {
		managed := []managedMember{
			{key: "type", text: compactValue("local")},
			{key: "command", text: compactValue(mergeArgv(commandArgv(entryRaw), command, configPath))},
			{key: "enabled", text: compactValue(true)},
		}
		return spliceEntry(entryRaw, indent, managed)
	})
}

// commandArgv reads the command array out of an existing OpenCode entry. A
// missing or unreadable one reads as empty, which leaves the user's flags
// none the worse: there are none.
func commandArgv(entryRaw string) []string {
	pos, err := jsonSkip(entryRaw, 0)
	if err != nil || pos >= len(entryRaw) || entryRaw[pos] != '{' {
		return nil
	}
	obj, err := jsonScanObject(entryRaw, pos)
	if err != nil {
		return nil
	}
	member, ok := obj.member("command")
	if !ok {
		return nil
	}
	return stringArgv(entryRaw, member.valStart)
}

// argvFromMember reads a string-array member out of an existing entry, for
// the clients that keep their flags in a separate "args".
func argvFromMember(entryRaw, key string) []string {
	pos, err := jsonSkip(entryRaw, 0)
	if err != nil || pos >= len(entryRaw) || entryRaw[pos] != '{' {
		return nil
	}
	obj, err := jsonScanObject(entryRaw, pos)
	if err != nil {
		return nil
	}
	member, ok := obj.member(key)
	if !ok {
		return nil
	}
	return stringArgv(entryRaw, member.valStart)
}

// jsonArgs keeps an absent argument list an empty array rather than null,
// which is what the clients' schemas expect and what makes a re-run compare
// equal.
func jsonArgs(args []string) []string {
	if args == nil {
		return []string{}
	}
	return args
}

// sectionFor is the key each client keeps its MCP server list under.
func sectionFor(format configFormat) string {
	switch format {
	case claudeJSON:
		return "mcpServers"
	case opencodeJSON:
		return "mcp"
	default:
		return "mcp_servers"
	}
}

// registeredCommand returns the executable a client would start for the
// helper, and whether it has an entry for the helper at all.
//
// The clients disagree about the shape: Claude Code and Codex keep the
// executable in "command" with the rest in "args", while OpenCode keeps the
// whole argv in "command". Both are read, because a status check wants the
// same fact either way — which binary this client is going to try to start,
// so that an entry pointing at a helper that has since moved can be told
// apart from no entry at all. The client itself cannot make that distinction
// visible: both look like tools that are not there.
//
// The JSON half reads through the lenient scanner, so a config with comments
// — legal JSONC to OpenCode — answers instead of erroring.
func registeredCommand(existing string, format configFormat) (string, bool, error) {
	if strings.TrimSpace(existing) == "" {
		return "", false, nil
	}
	if format == codexTOML {
		root, err := decodeTOML(existing)
		if err != nil {
			return "", false, err
		}
		table, ok := root["mcp_servers"].(map[string]any)
		if !ok {
			return "", false, nil
		}
		entry, ok := table[serverName].(map[string]any)
		if !ok {
			return "", false, nil
		}
		command, _ := entry["command"].(string)
		return command, true, nil
	}
	return registeredJSONCommand(existing, sectionFor(format))
}

// registeredJSONCommand reads the helper's command through the surgical
// scanner: tolerant of comments and trailing commas, deaf to layout.
func registeredJSONCommand(existing, section string) (string, bool, error) {
	root, err := jsonScanRoot(existing)
	if err != nil {
		return "", false, err
	}
	secMember, ok := root.member(section)
	if !ok {
		return "", false, nil
	}
	sec, isObject, err := jsonScanValueObject(existing, secMember.valStart)
	if err != nil || !isObject {
		return "", false, err
	}
	entry, ok := sec.member(serverName)
	if !ok {
		return "", false, nil
	}
	entryObject, isObject, err := jsonScanValueObject(existing, entry.valStart)
	if err != nil || !isObject {
		// Registered, but not an entry the helper can read: the caller
		// reports the binary as unknown rather than absent.
		return "", true, err
	}
	command, ok := entryObject.member("command")
	if !ok {
		return "", true, nil
	}
	raw := existing[command.valStart:command.valEnd]
	if strings.HasPrefix(raw, "\"") {
		if _, decoded, err := jsonScanString(existing, command.valStart); err == nil {
			return decoded, true, nil
		}
		return "", true, nil
	}
	if argv := stringArgv(existing, command.valStart); len(argv) > 0 {
		return argv[0], true, nil
	}
	return "", true, nil
}

// withoutJSON drops the helper from section — its entry and, once that is
// all the section held, the section — leaving every other byte of the file
// as it found it. A nil result says the helper was not there, and the caller
// writes nothing at all, which is what keeps a re-run from reformatting a
// config for no reason.
func withoutJSON(existing string, section string) (*string, error) {
	return jsonWithout(existing, section)
}

// mergeCodexTOML inserts the helper into Codex's TOML config. TOML has no
// surgical editor here: BurntSushi's decoder is the only reader this repo
// has for it, and re-encoding the whole file is the contract Codex users
// already live with.
func mergeCodexTOML(existing string, command string, args []string) (string, error) {
	root, err := decodeTOML(existing)
	if err != nil {
		return "", err
	}

	servers, ok := root["mcp_servers"]
	if !ok || servers == nil {
		servers = map[string]any{}
	}
	table, ok := servers.(map[string]any)
	if !ok {
		return "", fmt.Errorf("'mcp_servers' is not a TOML table")
	}
	table[serverName] = mergeEntry(table[serverName], map[string]any{"command": command, "args": jsonArgs(args)})
	root["mcp_servers"] = table

	return encodeTOML(root)
}

// withoutCodexTOML is the TOML counterpart of withoutJSON, with the same
// contract.
func withoutCodexTOML(existing string) (*string, error) {
	if strings.TrimSpace(existing) == "" {
		return nil, nil
	}
	root, err := decodeTOML(existing)
	if err != nil {
		return nil, err
	}

	table, ok := root["mcp_servers"].(map[string]any)
	if !ok {
		return nil, nil
	}
	if _, ok := table[serverName]; !ok {
		return nil, nil
	}
	delete(table, serverName)
	if len(table) == 0 {
		delete(root, "mcp_servers")
	}

	text, err := encodeTOML(root)
	if err != nil {
		return nil, err
	}
	return &text, nil
}

// mergeEntry writes wanted over the entry that is already there, rather than
// replacing it. The keys the helper sets are the ones it knows about;
// everything else under the entry belongs to the user. (TOML path only — the
// JSON clients go through spliceEntry, which keeps the bytes too.)
func mergeEntry(existing any, wanted map[string]any) map[string]any {
	merged, ok := existing.(map[string]any)
	if !ok {
		return wanted
	}
	for key, value := range wanted {
		merged[key] = value
	}
	return merged
}

func decodeTOML(existing string) (map[string]any, error) {
	root := map[string]any{}
	if strings.TrimSpace(existing) == "" {
		return root, nil
	}
	if _, err := toml.Decode(existing, &root); err != nil {
		return nil, fmt.Errorf("existing config is not valid TOML: %w", err)
	}
	return root, nil
}

func encodeTOML(root map[string]any) (string, error) {
	var buffer strings.Builder
	if err := toml.NewEncoder(&buffer).Encode(root); err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	return buffer.String(), nil
}
