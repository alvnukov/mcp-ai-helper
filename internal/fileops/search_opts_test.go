package fileops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regex mode treats the pattern as a regular expression; the default stays a
// literal substring, so the same pattern text must answer differently per mode.
func TestSearchFilesRegexPattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("func main() {}\nfunc  spaced() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	byRegex, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: `func\s+spaced`, Regex: true, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if byRegex.Total != 1 || len(byRegex.Matches) != 1 || !strings.Contains(byRegex.Matches[0], "a.go:2:func  spaced()") {
		t.Fatalf("regex result = %#v, want the one regex match", byRegex)
	}

	asLiteral, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: `func\s+spaced`, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if asLiteral.Total != 0 {
		t.Fatalf("literal result = %#v, want zero matches for regex syntax read literally", asLiteral)
	}
}

// An unparsable regex must fail loudly: a silent empty answer from a typo'd
// pattern reads as "nothing matches anywhere in the repo".
func TestSearchFilesInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	if _, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: `func(`, Regex: true, MaxMatches: 10}); err == nil {
		t.Fatal("expected an error for an invalid regex pattern")
	}
}

// smart_case matches case-insensitively while the pattern is all-lowercase and
// case-sensitively once it carries an uppercase letter (rg -S); ignore_case is
// case-insensitive regardless.
func TestSearchFilesCaseModes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("Hello helper\nhelper talks\nHELPER shouts\nHelper exact\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	smart, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "helper", SmartCase: true, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if smart.Total != 4 {
		t.Fatalf("smart_case total = %d, want all four case-tolerant lines", smart.Total)
	}

	smartUpper, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "Helper", SmartCase: true, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if smartUpper.Total != 1 || !strings.Contains(smartUpper.Matches[0], "notes.md:4:Helper exact") {
		t.Fatalf("smart_case with uppercase result = %#v, want only the exact-case line", smartUpper)
	}

	ignore, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "Helper", IgnoreCase: true, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if ignore.Total != 4 {
		t.Fatalf("ignore_case total = %d, want all four case variants", ignore.Total)
	}
}

// Glob narrows the walk to matching files; GlobExclude drops matches. A
// pattern without a separator matches the base name, one with a separator the
// repo-relative path.
func TestSearchFilesGlobFilters(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"a.go":          "needle here\n",
		"b.md":          "needle there\n",
		"pkg/c.go":      "needle deep\n",
		"pkg/notes.txt": "needle wider\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	goOnly, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "needle", Glob: []string{"*.go"}, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if goOnly.Total != 2 || !strings.Contains(strings.Join(goOnly.Matches, "\n"), "pkg/c.go") {
		t.Fatalf("glob *.go = %#v, want a.go and pkg/c.go only", goOnly)
	}

	byPath, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "needle", Glob: []string{"pkg/*.go"}, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if byPath.Total != 1 || !strings.Contains(byPath.Matches[0], "pkg/c.go:1:needle deep") {
		t.Fatalf("glob pkg/*.go = %#v, want only the deep file", byPath)
	}

	noMd, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "needle", GlobExclude: []string{"*.md"}, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if noMd.Total != 3 || strings.Contains(strings.Join(noMd.Matches, "\n"), "b.md") {
		t.Fatalf("glob_exclude *.md = %#v, want every file except b.md", noMd)
	}
}

// Context lines ride along with each match: matches keep ':' as the line
// separator, context lines use '-', and overlapping windows merge instead of
// repeating a line.
func TestSearchFilesContextLines(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte("one\ntwo\nhit\nfour\nhit\nsix\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "hit", ContextBefore: 1, ContextAfter: 1, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, context must not add matches", result.Total)
	}
	want := []string{
		"log.txt-2:two",
		"log.txt:3:hit",
		"log.txt-4:four",
		"log.txt:5:hit",
		"log.txt-6:six",
	}
	if len(result.Matches) != len(want) {
		t.Fatalf("matches = %#v, want %d merged lines", result.Matches, len(want))
	}
	for i, wantLine := range want {
		if result.Matches[i] != wantLine {
			t.Fatalf("matches[%d] = %q, want %q", i, result.Matches[i], wantLine)
		}
	}
}

// files_only answers with the paths of files holding matches, not the lines —
// the cheap first pass when scouting a large tree.
func TestSearchFilesFilesOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("needle\nneedle again\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "two.txt"), []byte("nothing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "needle", FilesOnly: true, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Matches) != 1 || result.Matches[0] != "one.txt" {
		t.Fatalf("files_only result = %#v, want exactly one.txt", result)
	}
}

// Invert flips the predicate per line (rg -v): every non-matching line is
// reported, matching ones are not.
func TestSearchFilesInvert(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mix.txt"), []byte("keep this\nskip me\nkeep that\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "skip", Invert: true, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Fatalf("invert total = %d, want the two non-matching lines", result.Total)
	}
	joined := strings.Join(result.Matches, "\n")
	if !strings.Contains(joined, "mix.txt:1:keep this") || strings.Contains(joined, "skip me") {
		t.Fatalf("invert matches = %#v, want non-matching lines only", result.Matches)
	}
}

// rg-style globs reach through directories: **/*.go must find a .go file at
// any depth, which path.Match alone cannot express.
func TestSearchFilesGlobstar(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "deep.go"), []byte("needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shallow.md"), []byte("needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "needle", Glob: []string{"**/*.go"}, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || !strings.Contains(result.Matches[0], "a/b/deep.go") {
		t.Fatalf("globstar result = %#v, want the deep .go file", result)
	}
}

// Consecutive matching lines must each keep their ':' match marker: a merged
// context window may not relabel a matching line as '-' context.
func TestSearchFilesContextAdjacentHits(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte("one\ntwo\nhit\nhit\nfive\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := SearchFilesInRepoWithOptions(dir, "", SearchOptions{Pattern: "hit", ContextAfter: 1, MaxMatches: 10})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(result.Matches, "\n")
	if result.Total != 2 || !strings.Contains(joined, "log.txt:3:hit") || !strings.Contains(joined, "log.txt:4:hit") {
		t.Fatalf("adjacent hits = %#v, want both hit lines marked with ':'", result)
	}
	if strings.Contains(joined, "log.txt-4:") {
		t.Fatalf("adjacent hits = %#v, line 4 must not be context", result)
	}
}
