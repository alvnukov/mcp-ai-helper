package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

// The MCP server entry itself: the stanza that makes the helper's tools exist.
//
// Three clients, three shapes. Every function here takes the file as it stands
// and returns what it should become, so the caller decides whether that is
// different enough to be worth a write.

func claudeEntry(command string, args []string) map[string]any {
	return map[string]any{"command": command, "args": jsonArgs(args)}
}

func opencodeEntry(command string, args []string) map[string]any {
	argv := append([]string{command}, args...)
	return map[string]any{"type": "local", "command": argv, "enabled": true}
}

// jsonArgs keeps an absent argument list an empty array rather than null, which
// is what the clients' schemas expect and what makes a re-run compare equal.
func jsonArgs(args []string) []string {
	if args == nil {
		return []string{}
	}
	return args
}

// mergeJSON inserts the helper under section, leaving every other key of the
// file intact.
func mergeJSON(existing string, section string, entry map[string]any) (string, error) {
	root, err := decodeJSON(existing)
	if err != nil {
		return "", err
	}

	servers, ok := root[section]
	if !ok || servers == nil {
		servers = map[string]any{}
	}
	table, ok := servers.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%q in the existing config is not a map", section)
	}
	table[serverName] = mergeEntry(table[serverName], entry)
	root[section] = table

	return encodeJSON(root)
}

// mergeEntry writes wanted over the entry that is already there, rather than
// replacing it.
//
// The keys the helper sets are the ones it knows about — the command line, and
// how the client should launch it. Everything else under the entry belongs to
// the user: Codex keeps per-tool approval_mode there, and the JSON clients take
// env and timeout settings. Replacing the entry wholesale would silently
// discard those on every re-registration, which is the sort of loss nobody
// notices until an approval prompt they had turned off comes back.
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

// withoutJSON drops the helper from section, and section itself once it holds
// nothing else, so uninstalling leaves the file as clean as it found it. A nil
// result says the helper was not there — the caller writes nothing at all in
// that case, which is what keeps a re-run from reformatting a config for no
// reason.
func withoutJSON(existing string, section string) (*string, error) {
	if strings.TrimSpace(existing) == "" {
		return nil, nil
	}
	root, err := decodeJSON(existing)
	if err != nil {
		return nil, err
	}

	table, ok := root[section].(map[string]any)
	if !ok {
		return nil, nil
	}
	if _, ok := table[serverName]; !ok {
		return nil, nil
	}
	delete(table, serverName)
	if len(table) == 0 {
		delete(root, section)
	}
	if len(root) == 0 {
		// A config file whose whole content would be {} says nothing that its
		// absence does not. Reporting it as empty lets the caller take the file
		// away instead of leaving a husk behind.
		empty := ""
		return &empty, nil
	}

	text, err := encodeJSON(root)
	if err != nil {
		return nil, err
	}
	return &text, nil
}

// decodeJSON reads the file into a tree, treating an empty file as an empty
// object. Numbers are kept as json.Number so that re-encoding a config the
// helper only passed through cannot round a large integer through float64.
func decodeJSON(existing string) (map[string]any, error) {
	if strings.TrimSpace(existing) == "" {
		return map[string]any{}, nil
	}
	decoder := json.NewDecoder(strings.NewReader(existing))
	decoder.UseNumber()
	var root any
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("existing config is not valid JSON: %w", err)
	}
	table, ok := root.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("existing config is not a JSON object")
	}
	return table, nil
}

func encodeJSON(root map[string]any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	// The clients' configs hold URLs and shell arguments, where escaping &, <
	// and > would rewrite somebody else's entry into something they no longer
	// recognise.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(root); err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	return buffer.String(), nil
}

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
	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(root); err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	return buffer.String(), nil
}
