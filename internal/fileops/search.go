package fileops

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alvnukov/mcp-ai-helper/internal/safefs"
)

// SearchResult holds structured search results.
type SearchResult struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	// Matches are grep's own file:line:text, one string per match, rather than
	// objects with file, line_number and text keys. Those three key names would
	// repeat on every match, which across a full result is most of the payload,
	// and the reader is a model that has met this exact layout in grep and
	// ripgrep output far more often than in any JSON shape of it.
	Matches []string `json:"matches"`
	Total   int      `json:"total"`
	// Truncated reports that the walk stopped at the match cap, so the tree may
	// hold matches this result does not show. Without it Total reads as a count
	// of everything, and a reader draws a conclusion from a partial answer
	// without knowing that it is partial.
	Truncated bool `json:"truncated"`
}

// SearchOptions extends the literal substring search with rg-like modes. The
// zero value keeps the historical behaviour: a literal, case-sensitive
// substring match.
type SearchOptions struct {
	// Pattern is the search text: a literal substring unless Regex is set.
	Pattern string
	// Regex interprets Pattern as a regular expression.
	Regex bool
	// IgnoreCase matches case-insensitively whatever the pattern's case.
	IgnoreCase bool
	// SmartCase matches case-insensitively only while Pattern carries no
	// uppercase letter (rg -S). IgnoreCase wins when both are set.
	SmartCase bool
	// Glob restricts the search to files matching at least one pattern
	// (rg -g). A pattern without a separator matches the base name; one with
	// a separator matches the repo-relative path.
	Glob []string
	// GlobExclude drops files matching any pattern, same layout as Glob
	// (rg -g '!...').
	GlobExclude []string
	// ContextBefore and ContextAfter add non-matching neighbour lines around
	// each match, marked with '-' instead of ':' as grep does. They do not
	// count towards Total.
	ContextBefore int
	ContextAfter  int
	// FilesOnly reports the paths of files holding at least one match instead
	// of the matching lines (rg -l); Total then counts files.
	FilesOnly bool
	// Invert matches the lines the pattern does not hit (rg -v).
	Invert bool
	// MaxMatches caps total reported matches; 0 means 100.
	MaxMatches int
}

// searchMatcher decides whether one line matches the configured pattern.
type searchMatcher struct {
	regex   *regexp.Regexp
	literal string
	fold    bool
}

func newSearchMatcher(opts SearchOptions) (searchMatcher, error) {
	icase := opts.IgnoreCase
	if !icase && opts.SmartCase && opts.Pattern == strings.ToLower(opts.Pattern) {
		icase = true
	}
	if !opts.Regex {
		literal := opts.Pattern
		if icase {
			literal = strings.ToLower(literal)
		}
		return searchMatcher{literal: literal, fold: icase}, nil
	}
	expr := opts.Pattern
	if icase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return searchMatcher{}, fmt.Errorf("invalid regex pattern %q: %w", opts.Pattern, err)
	}
	return searchMatcher{regex: re}, nil
}

func (m searchMatcher) matches(line string) bool {
	if m.regex != nil {
		return m.regex.MatchString(line)
	}
	if m.fold {
		return strings.Contains(strings.ToLower(line), m.literal)
	}
	return strings.Contains(line, m.literal)
}

// globSet is a precompiled include/exclude glob list. ** crosses directory
// separators (and a trailing **/ may match nothing, so **/*.go finds root
// files too), * and ? do not — the rg -g layout.
type globSet struct {
	regexes []*regexp.Regexp
	byBase  []bool
}

func compileGlobSet(patterns []string) globSet {
	set := globSet{}
	for _, pattern := range patterns {
		var b strings.Builder
		b.WriteString("^")
		for i := 0; i < len(pattern); i++ {
			switch c := pattern[i]; c {
			case '*':
				if i+1 < len(pattern) && pattern[i+1] == '*' {
					if i+2 < len(pattern) && pattern[i+2] == '/' {
						b.WriteString("(?:.*/)?")
						i += 2
					} else {
						b.WriteString(".*")
						i++
					}
				} else {
					b.WriteString("[^/]*")
				}
			case '?':
				b.WriteString("[^/]")
			default:
				b.WriteString(regexp.QuoteMeta(string(c)))
			}
		}
		b.WriteString("$")
		re, err := regexp.Compile(b.String())
		if err != nil {
			// Unreachable — every fragment is quoted or a literal form.
			continue
		}
		set.regexes = append(set.regexes, re)
		set.byBase = append(set.byBase, !strings.Contains(pattern, "/"))
	}
	return set
}

func (g globSet) empty() bool { return len(g.regexes) == 0 }

func (g globSet) matches(relative string) bool {
	for i, re := range g.regexes {
		candidate := relative
		if g.byBase[i] {
			candidate = path.Base(relative)
		}
		if re.MatchString(candidate) {
			return true
		}
	}
	return false
}

// SearchFiles runs a simple text search in a directory and returns structured results.
// It reads each non-binary file under root, splits into lines, and matches pattern.
func SearchFiles(rootPath string, pattern string, maxMatches int) (SearchResult, error) {
	return SearchFilesWithOptions(rootPath, SearchOptions{Pattern: pattern, MaxMatches: maxMatches})
}

// SearchFilesWithOptions is SearchFiles with the full rg-like option set.
func SearchFilesWithOptions(rootPath string, opts SearchOptions) (SearchResult, error) {
	root, err := safefs.Open(rootPath)
	if err != nil {
		return SearchResult{Pattern: opts.Pattern, Path: rootPath}, err
	}
	defer func() { _ = root.Close() }()
	return searchFilesAtRoot(rootPath, root, ".", opts)
}

// SearchFilesInRepo runs a text search under a repo-relative directory.
func SearchFilesInRepo(repoPath string, filePath string, pattern string, maxMatches int) (SearchResult, error) {
	return SearchFilesInRepoWithOptions(repoPath, filePath, SearchOptions{Pattern: pattern, MaxMatches: maxMatches})
}

// SearchFilesInRepoWithOptions is SearchFilesInRepo with the full rg-like option set.
func SearchFilesInRepoWithOptions(repoPath string, filePath string, opts SearchOptions) (SearchResult, error) {
	root, err := safefs.Open(repoPath)
	if err != nil {
		return SearchResult{}, err
	}
	defer func() { _ = root.Close() }()

	if strings.TrimSpace(filePath) == "" {
		return searchFilesAtRoot(root.Path(), root, ".", opts)
	}
	display, relative, err := repoRelativePath(repoPath, filePath)
	if err != nil {
		return SearchResult{}, err
	}
	return searchFilesAtRoot(display, root, relative, opts)
}

func searchFilesAtRoot(displayPath string, root *safefs.Root, walkRoot string, opts SearchOptions) (SearchResult, error) {
	if opts.MaxMatches <= 0 {
		opts.MaxMatches = 100
	}
	matcher, err := newSearchMatcher(opts)
	if err != nil {
		return SearchResult{}, err
	}
	includeGlobs := compileGlobSet(opts.Glob)
	excludeGlobs := compileGlobSet(opts.GlobExclude)
	walkRoot = filepath.ToSlash(filepath.Clean(walkRoot))
	result := SearchResult{Pattern: opts.Pattern, Path: displayPath}
	err = fs.WalkDir(root.FS(), walkRoot, func(entryPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		relative := strings.TrimPrefix(entryPath, walkRoot+"/")
		if entryPath == walkRoot {
			relative = "."
		}
		if d.IsDir() {
			base := path.Base(entryPath)
			if strings.HasPrefix(base, ".") && entryPath != walkRoot {
				return fs.SkipDir
			}
			if base == "node_modules" || base == "__pycache__" || base == "vendor" || isTaskRegistryRelative(relative) {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(path.Ext(entryPath))
		switch ext {
		case ".exe", ".dll", ".so", ".dylib", ".bin", ".jpg", ".png", ".gif", ".ico",
			".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".pdf", ".class", ".pyc", ".pyo":
			return nil
		}
		if isProtectedLeanPath(entryPath) {
			return nil
		}
		if relative == "." {
			// The walk names its root "."; for a search scoped to a single
			// file that root is the target, not something to skip.
			relative = path.Base(entryPath)
		}
		if !includeGlobs.empty() && !includeGlobs.matches(relative) {
			return nil
		}
		if !excludeGlobs.empty() && excludeGlobs.matches(relative) {
			return nil
		}
		data, readErr := root.ReadFile(filepath.FromSlash(entryPath))
		if readErr != nil || len(data) > 1<<20 {
			return nil
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		// A trailing newline splits into one phantom empty line that grep
		// never numbers; it must not become an invert hit or an empty-match
		// regex hit.
		if n := len(lines); n > 0 && lines[n-1] == "" {
			lines = lines[:n-1]
		}
		hitLines := make([]int, 0, 4)
		for i, line := range lines {
			if matcher.matches(line) == opts.Invert {
				continue
			}
			hitLines = append(hitLines, i)
		}
		if len(hitLines) == 0 {
			return nil
		}
		if opts.FilesOnly {
			result.Matches = append(result.Matches, relative)
			result.Total++
			if result.Total >= opts.MaxMatches {
				result.Truncated = true
				return fs.SkipAll
			}
			return nil
		}
		// Context windows of consecutive hits merge into one run, as in grep,
		// and a matching line keeps its ':' marker even when an earlier
		// window already emitted it.
		hitSet := make(map[int]bool, len(hitLines))
		for _, hit := range hitLines {
			hitSet[hit] = true
		}
		emitted := -1
		for _, hit := range hitLines {
			start := hit - opts.ContextBefore
			if start < 0 {
				start = 0
			}
			if start <= emitted {
				start = emitted + 1
			}
			end := hit + opts.ContextAfter
			if end >= len(lines) {
				end = len(lines) - 1
			}
			for i := start; i <= end; i++ {
				sep := "-"
				if hitSet[i] {
					sep = ":"
				}
				result.Matches = append(result.Matches, fmt.Sprintf("%s%s%d:%s", relative, sep, i+1, lines[i]))
				emitted = i
			}
			result.Total++
			if result.Total >= opts.MaxMatches {
				result.Truncated = true
				return fs.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}
