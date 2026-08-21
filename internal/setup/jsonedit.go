package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// This file is a surgical editor for the JSON config files setup writes into.
//
// The clients that own those files accept far more than encoding/json does —
// OpenCode documents JSONC, so comments and trailing commas are legal, and
// every client accepts any key order and any layout — and they keep far more
// in them than the helper's own stanza: model choices, providers, per-server
// settings a user added by hand.
//
// Re-encoding the whole document after a merge destroys all of that quietly:
// comments vanish, keys get re-sorted, one-line entries bloom into five. What
// the user experiences is "running setup broke my config".
//
// So this editor never re-encodes what it did not write. It scans the
// document with a lenient parser that tracks byte spans, and splices: the
// helper's entry, and nothing else, is inserted, replaced or removed. Every
// other byte survives untouched, which makes setup and remove invisible in a
// diff of somebody else's file.

// jsonMember is one "key": value pair with the byte range its value occupies.
type jsonMember struct {
	key      string
	keyStart int
	valStart int
	valEnd   int
}

// jsonObject is a scanned JSON object: where its braces sit, and its members
// in document order.
type jsonObject struct {
	open    int
	close   int
	members []jsonMember
}

// member finds a member by key. A duplicate key answers with the first: the
// one a JSON reader would load.
func (o jsonObject) member(key string) (jsonMember, bool) {
	for _, m := range o.members {
		if m.key == key {
			return m, true
		}
	}
	return jsonMember{}, false
}

// empty reports whether the object holds nothing but whitespace and comments.
func (o jsonObject) empty() bool {
	return len(o.members) == 0
}

// jsonSkip advances past whitespace and JSONC comments, returning the offset
// of the next significant byte. An unterminated comment is refused rather
// than guessed at.
func jsonSkip(doc string, pos int) (int, error) {
	for pos < len(doc) {
		switch doc[pos] {
		case ' ', '\t', '\n', '\r':
			pos++
		case '/':
			if pos+1 >= len(doc) {
				return 0, fmt.Errorf("stray '/' at offset %d", pos)
			}
			switch doc[pos+1] {
			case '/':
				nl := strings.IndexByte(doc[pos+2:], '\n')
				if nl < 0 {
					return len(doc), nil
				}
				pos += nl + 3
			case '*':
				end := strings.Index(doc[pos+2:], "*/")
				if end < 0 {
					return 0, fmt.Errorf("unterminated /* comment at offset %d", pos)
				}
				pos += end + 4
			default:
				return 0, fmt.Errorf("stray '/' at offset %d", pos)
			}
		default:
			// A leading byte-order mark is whitespace to a lenient reader.
			if pos == 0 && strings.HasPrefix(doc, "\ufeff") {
				return jsonSkip(doc, 3)
			}
			return pos, nil
		}
	}
	return pos, nil
}

// jsonScanString reads the string whose opening quote sits at pos, returning
// the offset just past the closing quote and the decoded text.
func jsonScanString(doc string, pos int) (int, string, error) {
	if pos >= len(doc) || doc[pos] != '"' {
		return 0, "", fmt.Errorf("expected a string at offset %d", pos)
	}
	i := pos + 1
	var b strings.Builder
	for i < len(doc) {
		c := doc[i]
		switch {
		case c == '"':
			return i + 1, b.String(), nil
		case c == '\\' && i+1 < len(doc):
			switch e := doc[i+1]; e {
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'u':
				if i+6 > len(doc) {
					return 0, "", fmt.Errorf("truncated \\u escape at offset %d", i)
				}
				v, err := strconv.ParseUint(doc[i+2:i+6], 16, 16)
				if err != nil {
					return 0, "", fmt.Errorf("bad \\u escape at offset %d: %w", i, err)
				}
				b.WriteRune(rune(v)) // #nosec G115 -- four hex digits cap v at 0xFFFF, always a valid rune
				i += 4
			default:
				return 0, "", fmt.Errorf("unknown escape %q at offset %d", e, i)
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	return 0, "", fmt.Errorf("unterminated string at offset %d", pos)
}

// jsonSkipValue returns the offset just past the value starting at pos,
// tolerating the trailing commas a JSONC reader accepts inside containers.
func jsonSkipValue(doc string, pos int) (int, error) {
	pos, err := jsonSkip(doc, pos)
	if err != nil {
		return 0, err
	}
	if pos >= len(doc) {
		return 0, fmt.Errorf("expected a value at offset %d", pos)
	}
	switch doc[pos] {
	case '"':
		end, _, err := jsonScanString(doc, pos)
		return end, err
	case '{':
		return jsonSkipContainer(doc, pos, '}')
	case '[':
		return jsonSkipContainer(doc, pos, ']')
	default:
		end := pos
		for end < len(doc) {
			c := doc[end]
			if c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '/' {
				break
			}
			end++
		}
		if end == pos {
			return 0, fmt.Errorf("unexpected %q at offset %d", doc[pos], pos)
		}
		return end, nil
	}
}

// jsonSkipContainer walks the container whose opening brace sits at pos,
// returning the offset just past its matching close. Members and elements are
// skipped, never interpreted; a comma directly before the close is legal.
func jsonSkipContainer(doc string, pos int, closing byte) (int, error) {
	pos++
	for {
		next, err := jsonSkip(doc, pos)
		if err != nil {
			return 0, err
		}
		pos = next
		if pos >= len(doc) {
			return 0, fmt.Errorf("unterminated %c", closing)
		}
		switch doc[pos] {
		case closing:
			return pos + 1, nil
		case ',':
			pos++
		case ':':
			// The separator between the key just scanned and its value;
			// the value is skipped here so a nested container is never
			// mistaken for the next member key.
			valueEnd, err := jsonSkipValue(doc, pos+1)
			if err != nil {
				return 0, err
			}
			pos = valueEnd
		default:
			if closing == '}' && doc[pos] != '"' {
				return 0, fmt.Errorf("expected a member key at offset %d", pos)
			}
			if doc[pos] == '"' {
				end, _, err := jsonScanString(doc, pos)
				if err != nil {
					return 0, err
				}
				// A string is either a member key — followed by ':' — or an
				// array element; both continue to the next token the same way.
				pos = end
				continue
			}
			valueEnd, err := jsonSkipValue(doc, pos)
			if err != nil {
				return 0, err
			}
			pos = valueEnd
		}
	}
}

// jsonScanObject scans the object whose '{' sits at open, recording every
// member's key and value span.
func jsonScanObject(doc string, open int) (jsonObject, error) {
	obj := jsonObject{open: open}
	pos := open + 1
	for {
		next, err := jsonSkip(doc, pos)
		if err != nil {
			return obj, err
		}
		pos = next
		if pos >= len(doc) {
			return obj, fmt.Errorf("unterminated object")
		}
		switch doc[pos] {
		case '}':
			obj.close = pos + 1
			return obj, nil
		case ',':
			pos++
			continue
		}
		if doc[pos] != '"' {
			return obj, fmt.Errorf("expected a member key at offset %d", pos)
		}
		keyStart := pos
		keyEnd, key, err := jsonScanString(doc, pos)
		if err != nil {
			return obj, err
		}
		pos, err = jsonSkip(doc, keyEnd)
		if err != nil {
			return obj, err
		}
		if pos >= len(doc) || doc[pos] != ':' {
			return obj, fmt.Errorf("expected ':' after %q at offset %d", key, pos)
		}
		pos, err = jsonSkip(doc, pos+1)
		if err != nil {
			return obj, err
		}
		valEnd, err := jsonSkipValue(doc, pos)
		if err != nil {
			return obj, err
		}
		obj.members = append(obj.members, jsonMember{key: key, keyStart: keyStart, valStart: pos, valEnd: valEnd})
		pos = valEnd
	}
}

// jsonScanRoot scans the document's root object. An empty document scans as
// an empty object parked at offset zero, so a caller can treat "no config
// yet" the way readConfig treats a missing file. Trailing content after the
// root is refused: this editor splices inside the root, and bytes it cannot
// account for are not its to orphan.
func jsonScanRoot(doc string) (jsonObject, error) {
	if strings.TrimSpace(doc) == "" {
		return jsonObject{}, nil
	}
	pos, err := jsonSkip(doc, 0)
	if err != nil {
		return jsonObject{}, err
	}
	if pos >= len(doc) || doc[pos] != '{' {
		return jsonObject{}, fmt.Errorf("existing config is not a JSON object")
	}
	obj, err := jsonScanObject(doc, pos)
	if err != nil {
		return jsonObject{}, fmt.Errorf("existing config is not valid JSON: %w", err)
	}
	tail, err := jsonSkip(doc, obj.close)
	if err != nil {
		return jsonObject{}, err
	}
	if tail != len(doc) {
		return jsonObject{}, fmt.Errorf("existing config has content after the root object")
	}
	return obj, nil
}

// jsonScanValueObject scans a value that must be an object, reporting whether
// it is one.
func jsonScanValueObject(doc string, valStart int) (jsonObject, bool, error) {
	pos, err := jsonSkip(doc, valStart)
	if err != nil {
		return jsonObject{}, false, err
	}
	if pos >= len(doc) || doc[pos] != '{' {
		return jsonObject{}, false, nil
	}
	obj, err := jsonScanObject(doc, pos)
	return obj, true, err
}

// lineIndent returns the leading whitespace of the line containing offset.
func lineIndent(doc string, offset int) string {
	start := strings.LastIndexByte(doc[:offset], '\n') + 1
	for i := start; i < len(doc); i++ {
		if c := doc[i]; c != ' ' && c != '\t' {
			return doc[start:i]
		}
	}
	return doc[start:]
}

// childIndent is the indentation a new member of this object should take.
func (o jsonObject) childIndent(doc string) string {
	if len(o.members) > 0 {
		return lineIndent(doc, o.members[0].keyStart)
	}
	return lineIndent(doc, o.open) + "  "
}

// jsonGapHasComma reports whether the bytes between the last member and the
// close brace already carry the trailing comma a JSONC file may have.
func jsonGapHasComma(doc string, obj jsonObject) bool {
	if len(obj.members) == 0 {
		return false
	}
	last := obj.members[len(obj.members)-1].valEnd
	for i := last; i < obj.close-1; i++ {
		if doc[i] == ',' {
			return true
		}
	}
	return false
}

// jsonInsertMember appends "key": valueText to obj in doc, matching the
// indentation of the members already there, and returns the new document.
// Only bytes before the close brace are touched — and none after the last
// byte of the new value, so removing the member again restores the file
// exactly as it was.
func jsonInsertMember(doc string, obj jsonObject, key, valueText string) string {
	lead := ","
	if len(obj.members) == 0 || jsonGapHasComma(doc, obj) {
		lead = ""
	}
	insertion := lead + "\n" + obj.childIndent(doc) + strconv.Quote(key) + ": " + valueText
	return doc[:obj.close-1] + insertion + doc[obj.close-1:]
}

// jsonDeleteMember removes member i from obj in doc — its separators and its
// line, not its neighbours' — and returns the new document.
func jsonDeleteMember(doc string, obj jsonObject, i int) string {
	m := obj.members[i]
	start, end := m.keyStart, m.valEnd
	switch {
	case i+1 < len(obj.members):
		// Cut through to the next member's key: that span holds this
		// member's trailing comma, and taking it keeps the survivors
		// separated by the previous member's own comma.
		end = obj.members[i+1].keyStart
	case i > 0:
		// The last member: cut from just past the previous value, so the
		// comma that separated them goes with it.
		start = obj.members[i-1].valEnd
	default:
		// The only member: empty the braces rather than delete them, keeping
		// any comment that stood around them.
		return doc[:obj.open+1] + doc[obj.close-1:]
	}
	return doc[:start] + doc[end:]
}

// compactValue renders one JSON value on a single line, without the HTML
// escaping that would rewrite a URL the helper only passed through.
func compactValue(v any) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return "null"
	}
	return strings.TrimRight(buffer.String(), "\n")
}

// managedMember is one entry key the helper owns, with its wanted value.
type managedMember struct {
	key  string
	text string
}

// spliceEntry renders an entry the helper is willing to sign: its own managed
// members first, in a canonical shape, then every other member the existing
// entry carried — each as the exact bytes it already occupies, because those
// belong to the user and re-registering is not a licence to reformat them.
//
// An existing entry that is not an object at all is simply replaced.
func spliceEntry(existingRaw, base string, managed []managedMember) string {
	member := base + "  "
	lines := make([]string, 0, len(managed))
	for _, m := range managed {
		lines = append(lines, member+strconv.Quote(m.key)+": "+m.text)
	}
	for _, raw := range keepUnmanaged(existingRaw, managed) {
		lines = append(lines, member+raw)
	}
	return "{\n" + strings.Join(lines, ",\n") + "\n" + base + "}"
}

// keepUnmanaged extracts the members of an existing entry the helper does not
// manage, as their raw "key": value text.
func keepUnmanaged(existingRaw string, managed []managedMember) []string {
	pos, err := jsonSkip(existingRaw, 0)
	if err != nil || pos >= len(existingRaw) || existingRaw[pos] != '{' {
		return nil
	}
	obj, err := jsonScanObject(existingRaw, pos)
	if err != nil {
		return nil
	}
	owns := make(map[string]bool, len(managed))
	for _, m := range managed {
		owns[m.key] = true
	}
	var kept []string
	for _, m := range obj.members {
		if owns[m.key] {
			continue
		}
		kept = append(kept, existingRaw[m.keyStart:m.valEnd])
	}
	return kept
}

// stringArgv parses a JSON array of strings, refusing anything else: an
// entry the helper cannot read as an argv is one it replaces wholesale.
func stringArgv(doc string, valStart int) []string {
	pos, err := jsonSkip(doc, valStart)
	if err != nil || pos >= len(doc) || doc[pos] != '[' {
		return nil
	}
	var argv []string
	end := jsonSkipContainerBrackets(doc, pos, &argv)
	if end < 0 {
		return nil
	}
	return argv
}

// jsonSkipContainerBrackets walks an array from its '[' collecting string
// elements into out, returning the offset just past ']' or -1 when the array
// holds anything but strings.
func jsonSkipContainerBrackets(doc string, pos int, out *[]string) int {
	pos++
	for {
		next, err := jsonSkip(doc, pos)
		if err != nil {
			return -1
		}
		pos = next
		if pos >= len(doc) {
			return -1
		}
		switch doc[pos] {
		case ']':
			return pos + 1
		case ',':
			pos++
		default:
			if doc[pos] != '"' {
				return -1
			}
			endToken, value, err := jsonScanString(doc, pos)
			if err != nil {
				return -1
			}
			*out = append(*out, value)
			pos = endToken
		}
	}
}

// stripConfigPairs drops the --config pairs from an argv tail: the pair is
// the helper's to manage, set when setup pins a config and taken away when it
// does not, never duplicated.
func stripConfigPairs(flags []string) []string {
	kept := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		if flags[i] == "--config" && i+1 < len(flags) {
			i++
			continue
		}
		kept = append(kept, flags[i])
	}
	return kept
}

// mergeArgv re-pins the binary and the --config pair of an OpenCode entry and
// keeps every other flag the user put there, in its order: a --repo pinned by
// hand survives a re-registration.
func mergeArgv(existing []string, binary string, configPath string) []string {
	var tail []string
	if len(existing) > 1 {
		tail = stripConfigPairs(existing[1:])
	}
	argv := []string{binary}
	if configPath != "" {
		argv = append(argv, "--config", configPath)
	}
	return append(argv, tail...)
}

// mergeArgs is mergeArgv for Claude's separate args array, which carries no
// binary of its own.
func mergeArgs(existing []string, configPath string) []string {
	tail := stripConfigPairs(existing)
	if configPath == "" {
		return tail
	}
	return append([]string{"--config", configPath}, tail...)
}

// jsonMerge installs the helper's entry under section, touching only the
// bytes of that entry. entryBuild receives the existing entry's raw text —
// empty when there is none — and the indentation its lines should take.
func jsonMerge(doc, section string, entryBuild func(existingRaw, indent string) string) (string, error) {
	entryAt := func(base string) string { return entryBuild("", base) }
	if strings.TrimSpace(doc) == "" {
		return "{\n  " + strconv.Quote(section) + ": {\n    " + strconv.Quote(serverName) +
			": " + entryAt("    ") + "\n  }\n}\n", nil
	}
	root, err := jsonScanRoot(doc)
	if err != nil {
		return "", err
	}
	secMember, ok := root.member(section)
	if !ok {
		secBase := root.childIndent(doc)
		entryBase := secBase + "  "
		sectionText := "{\n" + entryBase + strconv.Quote(serverName) + ": " + entryAt(entryBase) +
			"\n" + secBase + "}"
		return jsonInsertMember(doc, root, section, sectionText), nil
	}
	if doc[secMember.valStart] != '{' {
		return "", fmt.Errorf("%q in the existing config is not a map", section)
	}
	sec, err := jsonScanObject(doc, secMember.valStart)
	if err != nil {
		return "", fmt.Errorf("existing config is not valid JSON: %w", err)
	}
	entry, ok := sec.member(serverName)
	if !ok {
		return jsonInsertMember(doc, sec, serverName, entryBuild("", sec.childIndent(doc))), nil
	}
	indent := lineIndent(doc, entry.keyStart)
	return doc[:entry.valStart] + entryBuild(doc[entry.valStart:entry.valEnd], indent) + doc[entry.valEnd:], nil
}

// jsonWithout removes the helper's entry from under section, and the section
// itself once it holds nothing else. It returns nil when the helper is not
// there, and a pointer to an empty string when the document becomes a husk
// the caller should take away.
func jsonWithout(doc, section string) (*string, error) {
	if strings.TrimSpace(doc) == "" {
		return nil, nil
	}
	root, err := jsonScanRoot(doc)
	if err != nil {
		return nil, err
	}
	secMember, ok := root.member(section)
	if !ok {
		return nil, nil
	}
	sec, isObject, err := jsonScanValueObject(doc, secMember.valStart)
	if err != nil || !isObject {
		return nil, err
	}
	index := -1
	for i, m := range sec.members {
		if m.key == serverName {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, nil
	}
	doc = jsonDeleteMember(doc, sec, index)

	// Deletions shift every offset, so re-scan before deciding what else —
	// if anything — is now empty.
	root, err = jsonScanRoot(doc)
	if err != nil {
		return nil, err
	}
	if secMember, ok := root.member(section); ok {
		if sec, isObject, err := jsonScanValueObject(doc, secMember.valStart); err == nil && isObject && sec.empty() {
			for i, m := range root.members {
				if m.key == section {
					doc = jsonDeleteMember(doc, root, i)
					break
				}
			}
			if root, err = jsonScanRoot(doc); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}
	if root.empty() {
		empty := ""
		return &empty, nil
	}
	return &doc, nil
}
