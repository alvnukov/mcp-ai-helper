// Package leanfixture hands tests a Lean task-registry project that has already
// been built.
//
// The registry's tests each need a workspace that `lake exe task_registry_export`
// can run in, and reaching that state from bare sources costs a full Lean
// compile and link — a few seconds of it, repeated once per test. Two packages
// were paying that toll on every test, which is what pushed internal/mcp past
// its own timeout and left the largest package in the tree effectively untested.
//
// A finished .lake directory turns out to be relocatable: copy the whole
// workspace somewhere else and lake accepts the build as current. So the project
// is built once, cached under the system temp directory, and cloned per test.
// The cache key is the content of the sources, so editing a template or moving
// to a new toolchain invalidates it without anyone having to remember to.
package leanfixture

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SourceFiles are the project files copied into every workspace, in the layout
// the Lean project expects. ActiveTasksPath is written separately because tests
// and the bootstrap code both start it out empty.
var SourceFiles = []string{
	"lean-toolchain",
	"lakefile.lean",
	"MCPAIHelperProject.lean",
	"MCPAIHelperProject/ProjectState.lean",
	"MCPAIHelperProject/Samples.lean",
	"MCPAIHelperProject/Registry.lean",
	"MCPAIHelperProject/TaskRegistryExport.lean",
}

// ActiveTasksPath is the one source file the registry rewrites as tasks change.
const ActiveTasksPath = "MCPAIHelperProject/ActiveTasks.lean"

// EmptyActiveTasks is ActiveTasksPath with no tasks in it, which is where every
// fixture starts.
const EmptyActiveTasks = `import MCPAIHelperProject.ProjectState

namespace MCPAIHelperProject
namespace ActiveTasks

def activeArtifacts : List Artifact :=
  []

def activeRelations : List ArtifactRelation :=
  []

end ActiveTasks
end MCPAIHelperProject
`

// readyMarker is written last, so a directory carrying it is known to hold a
// complete build rather than one that was interrupted.
const readyMarker = ".leanfixture-ready"

// buildTimeout bounds the one build the cache ever pays for. A cold Lean compile
// of this project is seconds; minutes means something is wrong and failing is
// more use than hanging.
const buildTimeout = 5 * time.Minute

// Prepare returns a directory holding a built Lean project, building and caching
// one if this is the first call for these sources.
//
// sources supplies SourceFiles under prefix — an embed.FS in one caller and the
// template directory on disk in the other, which is why it is an fs.FS rather
// than a path.
func Prepare(sources fs.FS, prefix string) (fixture string, returnErr error) {
	files, err := readSources(sources, prefix)
	if err != nil {
		return "", err
	}

	cached := filepath.Join(os.TempDir(), "mcp-ai-helper-leanfixture-"+fingerprint(files))
	if isReady(cached) {
		return cached, nil
	}

	staging, err := os.MkdirTemp("", "leanfixture-build-")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(staging); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove staging directory: %w", cleanupErr))
		}
	}()

	if err := writeProject(staging, files); err != nil {
		return "", err
	}
	if err := build(staging); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(staging, readyMarker), nil, 0o600); err != nil {
		return "", fmt.Errorf("mark fixture ready: %w", err)
	}

	if err := os.Rename(staging, cached); err != nil {
		// Another test binary got there first, which is the expected outcome of
		// `go test ./...` running two packages at once. Its copy is built from
		// the same sources, so it is as good as this one.
		if isReady(cached) {
			return cached, nil
		}
		return "", fmt.Errorf("publish fixture to %s: %w", cached, err)
	}
	return cached, nil
}

// Clone copies a prepared fixture into dest, which tests are free to mutate.
func Clone(prepared string, dest string) error {
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return fmt.Errorf("create fixture destination: %w", err)
	}
	return filepath.WalkDir(prepared, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(prepared, path)
		if err != nil {
			return err
		}
		if relative == "." || relative == readyMarker {
			return nil
		}
		target := filepath.Join(dest, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			// Lean's build tree holds no symlinks or devices today; skipping
			// them keeps a surprise from turning into a copy that escapes dest.
			return nil
		}
		return copyFile(path, target)
	})
}

func readSources(sources fs.FS, prefix string) (map[string][]byte, error) {
	files := make(map[string][]byte, len(SourceFiles)+1)
	for _, name := range SourceFiles {
		data, err := fs.ReadFile(sources, path.Join(prefix, name))
		if err != nil {
			return nil, fmt.Errorf("read fixture source %s: %w", name, err)
		}
		files[name] = data
	}
	files[ActiveTasksPath] = []byte(EmptyActiveTasks)
	return files, nil
}

// fingerprint hashes the sources so that a change to any of them — including the
// pinned toolchain — asks for a new build instead of reusing a stale one.
func fingerprint(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	digest := sha256.New()
	for _, name := range names {
		digest.Write([]byte(name))
		digest.Write([]byte("\n"))
		digest.Write(strconv.AppendInt(nil, int64(len(files[name])), 10))
		digest.Write([]byte("\n"))
		digest.Write(files[name])
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

func writeProject(root string, files map[string][]byte) error {
	for name, data := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(name), err)
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// build runs the exporter rather than plain `lake build`, because linking the
// executable is the expensive half and running it is what the tests go on to do.
func build(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, "lake", "exe", "task_registry_export", "--list-active")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build lean fixture: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func isReady(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, readyMarker))
	return err == nil
}

func copyFile(source string, target string) (returnErr error) {
	in, err := os.Open(source) // #nosec G304 -- both paths come from the fixture the package just built.
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close fixture source: %w", closeErr))
		}
	}()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	// The build tree holds executables, and a copy that lost the bit would fail
	// far from here with a confusing "permission denied".
	mode := os.FileMode(0o600)
	if info.Mode()&0o100 != 0 {
		mode = 0o700
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) // #nosec G304 -- see above.
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		copyErr := fmt.Errorf("copy fixture file: %w", err)
		if closeErr := out.Close(); closeErr != nil {
			return errors.Join(copyErr, fmt.Errorf("close fixture destination: %w", closeErr))
		}
		return copyErr
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close fixture destination: %w", err)
	}
	return nil
}
