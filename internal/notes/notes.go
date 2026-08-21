// Package notes stores durable model-facing notes as markdown files with YAML
// frontmatter, one file per note, in a per-repository notebook or one global
// notebook shared across repositories.
package notes

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Scope selects which notebook a call addresses.
type Scope string

// The two notebooks: repo notes live with the repository and may be committed
// with it; global notes live in the helper's own data root and travel across
// repositories.
const (
	ScopeRepo   Scope = "repo"
	ScopeGlobal Scope = "global"
)

const (
	repoNotesDir = ".mcp-ai-helper/notes"
	maxList      = 200
	snippetWidth = 80
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

var errNotANote = errors.New("not a note file: expected --- frontmatter with id, title, created_at and updated_at")

// Note is one stored note: frontmatter and body together.
type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	Scope     Scope     `json:"scope"`
	Path      string    `json:"path"`
}

// Summary is a listing entry: the identifying fields without the body.
type Summary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Tags      []string  `json:"tags,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
	SizeBytes int64     `json:"size_bytes"`
}

// Match is one search hit: a bounded snippet and the byte offset it starts at.
type Match struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Offset  int    `json:"offset"`
}

// UpdateFields carries what an Update replaces; nil leaves a field alone, and
// non-nil Tags replaces the whole tag list, so tags: [] clears it.
type UpdateFields struct {
	Title *string
	Body  *string
	Tags  []string
}

type frontmatter struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Tags      []string `yaml:"tags,omitempty"`
	CreatedAt string   `yaml:"created_at"`
	UpdatedAt string   `yaml:"updated_at"`
}

// Store resolves the two notebooks and serialises writes within the process.
type Store struct {
	globalDir string
	mu        sync.Mutex
}

// NewStore builds a store whose global notebook is <helperRoot>/notes.
func NewStore(helperRoot string) *Store {
	return &Store{globalDir: filepath.Join(helperRoot, "notes")}
}

// Dir returns the directory of the selected notebook.
func (s *Store) Dir(scope Scope, repoPath string) (string, error) {
	if scope == ScopeGlobal {
		return s.globalDir, nil
	}
	if strings.TrimSpace(repoPath) == "" {
		return "", errors.New("scope repo requires repo_path")
	}
	return filepath.Join(repoPath, filepath.FromSlash(repoNotesDir)), nil
}

// Add writes a new note and returns it with its generated id.
func (s *Store) Add(scope Scope, repoPath, title, body string, tags []string) (Note, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Note{}, errors.New("title is required")
	}
	if strings.TrimSpace(body) == "" {
		return Note{}, errors.New("body is required")
	}
	dir, err := s.Dir(scope, repoPath)
	if err != nil {
		return Note{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Note{}, fmt.Errorf("create notebook %s: %w", dir, err)
	}
	now := time.Now().UTC()
	note := Note{
		Title:     title,
		Tags:      cleanTags(tags),
		CreatedAt: now,
		UpdatedAt: now,
		Body:      strings.Trim(body, "\n"),
		Scope:     normalizeScope(scope),
	}
	for attempt := 0; attempt < 8; attempt++ {
		suffix := make([]byte, 2)
		if _, err := rand.Read(suffix); err != nil {
			return Note{}, fmt.Errorf("generate note id: %w", err)
		}
		note.ID = now.Format("20060102-150405") + "-" + hex.EncodeToString(suffix)
		data, err := marshalNote(note)
		if err != nil {
			return Note{}, err
		}
		path := filepath.Join(dir, note.ID+".md")
		err = writeFileExclusive(path, data)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return Note{}, fmt.Errorf("write note %s: %w", path, err)
		}
		note.Path = path
		return note, nil
	}
	return Note{}, errors.New("note id collision, retry the call")
}

// List returns note summaries, newest first, optionally narrowed to one tag.
// A file that does not parse is skipped and counted, not fatal: a notebook the
// model cannot list because one file is odd is worse than a list with a hole.
func (s *Store) List(scope Scope, repoPath, tag string) ([]Summary, int, error) {
	dir, err := s.Dir(scope, repoPath)
	if err != nil {
		return nil, 0, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Summary{}, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read notebook %s: %w", dir, err)
	}
	tag = strings.TrimSpace(tag)
	summaries := []Summary{}
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			skipped++
			continue
		}
		note, err := parseNote(data, strings.TrimSuffix(entry.Name(), ".md"))
		if err != nil {
			skipped++
			continue
		}
		if tag != "" && !hasTag(note.Tags, tag) {
			continue
		}
		summaries = append(summaries, Summary{
			ID:        note.ID,
			Title:     note.Title,
			Tags:      note.Tags,
			UpdatedAt: note.UpdatedAt,
			SizeBytes: int64(len(data)),
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].UpdatedAt.Equal(summaries[j].UpdatedAt) {
			return summaries[i].ID > summaries[j].ID
		}
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})
	if len(summaries) > maxList {
		summaries = summaries[:maxList]
	}
	return summaries, skipped, nil
}

// Get returns one full note by id.
func (s *Store) Get(scope Scope, repoPath, id string) (Note, error) {
	path, err := s.notePath(scope, repoPath, id)
	if err != nil {
		return Note{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Note{}, fmt.Errorf("note %q not found in %s notebook", strings.TrimSpace(id), normalizeScope(scope))
	}
	if err != nil {
		return Note{}, fmt.Errorf("read note %s: %w", path, err)
	}
	note, err := parseNote(data, strings.TrimSpace(id))
	if err != nil {
		return Note{}, fmt.Errorf("read note %s: %w", path, err)
	}
	note.Scope = normalizeScope(scope)
	note.Path = path
	return note, nil
}

// Search returns case-insensitive substring matches, at most maxResults, each
// with a bounded snippet around the first hit in the body.
func (s *Store) Search(scope Scope, repoPath, query string, maxResults int) ([]Match, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("query is required")
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 50 {
		maxResults = 50
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	dir, err := s.Dir(scope, repoPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Match{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read notebook %s: %w", dir, err)
	}
	matches := []Match{}
	for _, entry := range entries {
		if len(matches) >= maxResults {
			break
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		note, err := parseNote(data, strings.TrimSuffix(entry.Name(), ".md"))
		if err != nil {
			continue
		}
		offset := strings.Index(strings.ToLower(note.Body), needle)
		if offset < 0 {
			if !strings.Contains(strings.ToLower(note.Title), needle) {
				continue
			}
			offset = 0
		}
		matches = append(matches, Match{ID: note.ID, Title: note.Title, Snippet: snippetAround(note.Body, offset), Offset: offset})
	}
	return matches, nil
}

// Update replaces the fields named in fields and bumps updated_at. The file is
// rewritten through a temp file and rename, so a reader never sees a torn note.
func (s *Store) Update(scope Scope, repoPath, id string, fields UpdateFields) (Note, error) {
	if fields.Title == nil && fields.Body == nil && fields.Tags == nil {
		return Note{}, errors.New("update requires at least one of title, body, tags")
	}
	if fields.Title != nil && strings.TrimSpace(*fields.Title) == "" {
		return Note{}, errors.New("title cannot be empty")
	}
	if fields.Body != nil && strings.TrimSpace(*fields.Body) == "" {
		return Note{}, errors.New("body cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.notePath(scope, repoPath, id)
	if err != nil {
		return Note{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Note{}, fmt.Errorf("note %q not found in %s notebook", strings.TrimSpace(id), normalizeScope(scope))
	}
	if err != nil {
		return Note{}, fmt.Errorf("read note %s: %w", path, err)
	}
	note, err := parseNote(data, strings.TrimSpace(id))
	if err != nil {
		return Note{}, fmt.Errorf("read note %s: %w", path, err)
	}
	if fields.Title != nil {
		note.Title = strings.TrimSpace(*fields.Title)
	}
	if fields.Body != nil {
		note.Body = strings.Trim(*fields.Body, "\n")
	}
	if fields.Tags != nil {
		note.Tags = cleanTags(fields.Tags)
	}
	note.UpdatedAt = time.Now().UTC()
	out, err := marshalNote(note)
	if err != nil {
		return Note{}, err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, out, 0o644); err != nil {
		return Note{}, fmt.Errorf("write note %s: %w", temp, err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return Note{}, fmt.Errorf("replace note %s: %w", path, err)
	}
	note.Scope = normalizeScope(scope)
	note.Path = path
	return note, nil
}

// Delete removes one note file.
func (s *Store) Delete(scope Scope, repoPath, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.notePath(scope, repoPath, id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("note %q not found in %s notebook", strings.TrimSpace(id), normalizeScope(scope))
		}
		return fmt.Errorf("delete note %s: %w", path, err)
	}
	return nil
}

func (s *Store) notePath(scope Scope, repoPath, id string) (string, error) {
	id = strings.TrimSpace(id)
	if !safeIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid note id %q", id)
	}
	dir, err := s.Dir(scope, repoPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".md"), nil
}

func parseNote(data []byte, expectedID string) (Note, error) {
	text := string(data)
	const fence = "---\n"
	if !strings.HasPrefix(text, fence) {
		return Note{}, errNotANote
	}
	rest := text[len(fence):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return Note{}, errNotANote
	}
	var fm frontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return Note{}, errNotANote
	}
	created, err := time.Parse(time.RFC3339Nano, fm.CreatedAt)
	if err != nil {
		return Note{}, errNotANote
	}
	updated, err := time.Parse(time.RFC3339Nano, fm.UpdatedAt)
	if err != nil {
		return Note{}, errNotANote
	}
	if fm.ID == "" || fm.Title == "" || (expectedID != "" && fm.ID != expectedID) {
		return Note{}, errNotANote
	}
	body := strings.Trim(rest[end+len("\n---"):], "\n")
	return Note{ID: fm.ID, Title: fm.Title, Tags: fm.Tags, CreatedAt: created, UpdatedAt: updated, Body: body}, nil
}

func marshalNote(note Note) ([]byte, error) {
	head, err := yaml.Marshal(frontmatter{
		ID:        note.ID,
		Title:     note.Title,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: note.UpdatedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, fmt.Errorf("encode note %s frontmatter: %w", note.ID, err)
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(head)
	buf.WriteString("---\n\n")
	buf.WriteString(note.Body)
	buf.WriteString("\n")
	return buf.Bytes(), nil
}

func writeFileExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func snippetAround(body string, index int) string {
	start := index - snippetWidth
	if start < 0 {
		start = 0
	}
	end := index + snippetWidth
	if end > len(body) {
		end = len(body)
	}
	snippet := body[start:end]
	for len(snippet) > 0 && !utf8.RuneStart(snippet[0]) {
		snippet = snippet[1:]
	}
	for len(snippet) > 0 && !utf8.RuneStart(snippet[len(snippet)-1]) {
		snippet = snippet[:len(snippet)-1]
	}
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(body) {
		snippet += "…"
	}
	return strings.TrimSpace(snippet)
}

func cleanTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func normalizeScope(scope Scope) Scope {
	if scope == ScopeGlobal {
		return ScopeGlobal
	}
	return ScopeRepo
}
