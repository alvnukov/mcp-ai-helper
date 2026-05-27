package fileops

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyGuardedReplaceWithBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	oldB64 := base64.StdEncoding.EncodeToString([]byte("line1"))
	newB64 := base64.StdEncoding.EncodeToString([]byte("replaced"))
	result, err := ApplyGuardedReplace(ReplaceRequest{
		Path:         path,
		ExpectedHash: snapshot.Hash,
		OldB64:       oldB64,
		NewB64:       newB64,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Changed {
		t.Fatalf("result = %+v, want ok changed", result)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "replaced\nline2\n" {
		t.Fatalf("file content = %q", string(data))
	}
}

func TestApplyGuardedReplaceBase64BackslashText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	// Content with backslashes — difficult to pass through JSON escaping.
	original := []byte("pattern: \\s and \\d\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := ReadSnapshot(path)
	// old text contains literal backslash-s backslash-d
	oldText := `\s and \d`
	newText := `\S and \D`
	result, err := ApplyGuardedReplace(ReplaceRequest{
		Path:         path,
		ExpectedHash: snapshot.Hash,
		OldB64:       base64.StdEncoding.EncodeToString([]byte(oldText)),
		NewB64:       base64.StdEncoding.EncodeToString([]byte(newText)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Changed {
		t.Fatalf("result = %+v, want ok changed. Reason: %s", result, result.Reason)
	}
	data, _ := os.ReadFile(path)
	expected := "pattern: \\S and \\D\n"
	if string(data) != expected {
		t.Fatalf("file content = %q, want %q", string(data), expected)
	}
}

func TestApplyGuardedReplaceBase64FallbackToOld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := ReadSnapshot(path)
	result, err := ApplyGuardedReplace(ReplaceRequest{
		Path:         path,
		ExpectedHash: snapshot.Hash,
		Old:          "hello",
		New:          "world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Changed {
		t.Fatalf("result = %+v, want ok changed", result)
	}
}

func TestApplyGuardedReplaceInvalidBase64(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("data\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := ReadSnapshot(path)
	_, err := ApplyGuardedReplace(ReplaceRequest{
		Path:         path,
		ExpectedHash: snapshot.Hash,
		OldB64:       "!!!not-base64!!!",
		NewB64:       base64.StdEncoding.EncodeToString([]byte("ok")),
	})
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestApplyGuardedReplaceDiagnosticsOnMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := ReadSnapshot(path)
	result, err := ApplyGuardedReplace(ReplaceRequest{
		Path:         path,
		ExpectedHash: snapshot.Hash,
		OldB64:       base64.StdEncoding.EncodeToString([]byte("func main() { println(\"hello\") }")),
		NewB64:       base64.StdEncoding.EncodeToString([]byte("func main() {}")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" {
		t.Fatalf("status = %q, want conflict", result.Status)
	}
	if !strings.Contains(result.Reason, "best partial match near:") {
		t.Fatalf("reason should contain diagnostic hint: %q", result.Reason)
	}
}

func TestApplyGuardedReplaceRejectsHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyGuardedReplace(ReplaceRequest{Path: path, ExpectedHash: "bad", Old: "one", New: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" {
		t.Fatalf("status = %q, want conflict", result.Status)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\n" {
		t.Fatalf("file was modified: %q", string(data))
	}
}

func TestApplyGuardedReplaceIsIdempotentWhenDesiredTextPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyGuardedReplace(ReplaceRequest{Path: path, ExpectedHash: snapshot.Hash, Old: "one", New: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.Changed {
		t.Fatalf("result = %+v, want ok unchanged", result)
	}
}

func TestReadSnapshotInRepoRejectsEscapingPath(t *testing.T) {
	_, err := ReadSnapshotInRepo(t.TempDir(), "../x.txt")
	if err == nil {
		t.Fatal("expected repo escape error")
	}
}
func TestReadFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fc, err := ReadFileContent(path)
	if err != nil {
		t.Fatal(err)
	}
	if !fc.Exists {
		t.Fatal("file should exist")
	}
	if fc.Size == 0 {
		t.Fatal("size should be > 0")
	}
	if len(fc.Lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(fc.Lines))
	}
	if fc.Lines[0].Number != 1 || fc.Lines[0].Text != "line1" {
		t.Fatalf("line[0] = %+v", fc.Lines[0])
	}
	if fc.Lines[1].Number != 2 || fc.Lines[1].Text != "line2" {
		t.Fatalf("line[1] = %+v", fc.Lines[1])
	}
}

func TestReadFileContentNotExist(t *testing.T) {
	fc, err := ReadFileContent("/nonexistent/path/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if fc.Exists {
		t.Fatal("file should not exist")
	}
}

func TestRepoFileOpsRejectLeanSourceFiles(t *testing.T) {
	dir := t.TempDir()
	leanRel := filepath.Join("MCPAIHelperProject", "ActiveTasks.lean")
	writePath := filepath.Join(dir, leanRel)
	if err := os.MkdirAll(filepath.Dir(writePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(writePath, []byte("def secret := 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadFileContentInRepo(dir, leanRel); err == nil || !strings.Contains(err.Error(), "policy_denied") || strings.Contains(err.Error(), "task-owned") {
		t.Fatalf("ReadFileContentInRepo error = %v, want local policy denial", err)
	}
	if _, err := ReadSnapshotInRepo(dir, leanRel); err == nil || !strings.Contains(err.Error(), "protected task registry source") || strings.Contains(err.Error(), "task-owned") {
		t.Fatalf("ReadSnapshotInRepo error = %v, want local policy denial", err)
	}
	if _, err := ApplyGuardedReplace(ReplaceRequest{RepoPath: dir, Path: leanRel, ExpectedHash: "deadbeef", Old: "def", New: "theorem"}); err == nil || !strings.Contains(err.Error(), "protected task registry source") || strings.Contains(err.Error(), "task-owned") {
		t.Fatalf("ApplyGuardedReplace error = %v, want local policy denial", err)
	}
}

func TestSearchFilesAllowsRegularLeanSourceFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Visible.go"), []byte("package p\n// needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Hidden.lean"), []byte("-- needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := SearchFilesInRepo(dir, "", "needle", 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Matches) != 2 {
		t.Fatalf("matches = %#v, want Go and regular Lean matches", result.Matches)
	}
}

func TestSearchFilesSkipsTaskRegistryDirectories(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		filepath.Join("obsidian-tasks", "task-001.md"),
		filepath.Join("tasks", "task-001.lean"),
		filepath.Join("MCPAIHelperProject", "ActiveTasks.lean"),
	} {
		writePath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(writePath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(writePath, []byte("hidden-task-needle\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	visiblePath := filepath.Join(dir, "internal", "visible.go")
	if err := os.MkdirAll(filepath.Dir(visiblePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(visiblePath, []byte("package internal\n// hidden-task-needle\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := SearchFilesInRepo(dir, "", "hidden-task-needle", 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Matches) != 1 || result.Matches[0].File != "internal/visible.go" {
		t.Fatalf("matches = %#v, want only non-task project files", result.Matches)
	}
}

func TestRepoFileOpsAllowRegularLeanSourceFiles(t *testing.T) {
	dir := t.TempDir()
	leanRel := filepath.Join("src", "Module.lean")
	writePath := filepath.Join(dir, leanRel)
	if err := os.MkdirAll(filepath.Dir(writePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(writePath, []byte("def visible := 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fc, err := ReadFileContentInRepo(dir, leanRel)
	if err != nil {
		t.Fatal(err)
	}
	if !fc.Exists || fc.RelativePath != filepath.ToSlash(leanRel) {
		t.Fatalf("file content = %#v", fc)
	}
}

func TestReadFileContentInRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fc, err := ReadFileContentInRepo(dir, "src/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !fc.Exists {
		t.Fatal("file should exist")
	}
	if fc.RelativePath != "src/main.go" {
		t.Fatalf("RelativePath = %q", fc.RelativePath)
	}
}

func TestReadFileContentInRepoRejectsEscape(t *testing.T) {
	_, err := ReadFileContentInRepo(t.TempDir(), "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
}
func TestSearchFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n\nfunc Foo() {\n\tprintln(\"hello\")\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\n\nfunc Bar() {\n\tFoo()\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := SearchFiles(dir, "Foo", 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total < 2 {
		t.Fatalf("total = %d, want >= 2", result.Total)
	}
	for _, m := range result.Matches {
		if !strings.Contains(m.Text, "Foo") {
			t.Fatalf("match should contain pattern: %q", m.Text)
		}
		if m.LineNumber < 1 {
			t.Fatalf("line number should be >= 1: %d", m.LineNumber)
		}
	}
}

func TestSearchFilesMaxMatches(t *testing.T) {
	dir := t.TempDir()
	for i := range 5 {
		name := filepath.Join(dir, fmt.Sprintf("f%d.go", i))
		data := []byte(fmt.Sprintf("package p\nvar x%d = 1\nvar y%d = 2\n", i, i))
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := SearchFiles(dir, "var", 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total > 3 {
		t.Fatalf("total = %d, want <= 3 (max)", result.Total)
	}
}

func TestSearchFilesSkipsHidden(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("secret=foo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("var secret = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := SearchFiles(dir, "secret", 10)
	if err != nil {
		t.Fatal(err)
	}
	// Should find main.go but not .git/config
	for _, m := range result.Matches {
		if strings.Contains(m.File, ".git") {
			t.Fatalf("should skip .git dir: %s", m.File)
		}
	}
}

func TestSearchFilesInRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := SearchFilesInRepo(dir, "", "func main", 10)
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 {
		t.Fatalf("total = %d, want 1", result.Total)
	}
}

func TestReadFileContentInRepoRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	linkPath := filepath.Join(dir, "link.txt")
	if err := os.Symlink("/etc/passwd", linkPath); err != nil {
		t.Fatal(err)
	}
	_, err := ReadFileContentInRepo(dir, "link.txt")
	if err == nil {
		t.Fatal("expected error for symlink escape")
	}
}
