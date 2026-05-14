# TaskRegistryBackend Contract

> Authoritative reference for all task-registry backend implementations.
> Do not deviate from this contract without updating this document first.
> Generated: 2026-05-14 | From: obsidian-registry-capability-inventory

## 1. TaskRegistryBackend Interface (Go)

The canonical backend interface (already exists in `internal/mcp/task_backend.go`):

```go
type taskBackend interface {
    ListCurrent(ctx context.Context, repoPath string) ([]tasks.Task, string, error)
    ListAll(ctx context.Context, repoPath string) ([]tasks.Task, string, error)
    Get(ctx context.Context, repoPath string, id string) (tasks.Task, string, error)
    Upsert(ctx context.Context, req tasks.AddRequest) (taskMutationResult, error)
    SetStatus(ctx context.Context, req tasks.StatusRequest) (taskMutationResult, error)
    BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (taskBatchMutationResult, error)
    Delete(ctx context.Context, req tasks.DeleteRequest) (taskMutationResult, error)
}
```

### 1.1 Result Type Renames (breaking, already planned)

```go
type taskMutationResult struct {
    Task         tasks.Task`   `json:"task"`
    Source       string          `json:"source"`
    Validation   string          `json:"validation"`
    ChangedFiles []string        `json:"changed_files,omitempty"`
}

type taskBatchMutationResult struct {
    Upserted   []tasks.Task  `json:"upserted"`
    Closed     []tasks.Task  `json:"closed"`
    Source     string          `json:"source"`
    Validation string          `json:"validation"`
}
```

Rename `leanMutationResult` -> `taskMutationResult`, `leanBatchMutationResult` -> `taskBatchMutationResult`.
Both backends return the same result types.

### 1.2 Source String

The second return value of read methods (ListCurrent, ListAll, Get) is a source identifier:
- `lean_registry` for Lean backend
- `obsidian_registry` for Obsidian backend

The caller uses this for provenance tracking (setting `projection_source` on records).

### 1.3 Validation Contract

Each mutation result must populate `Validation` with a compact summary (e.g., `lake build` for Lean, `frontmatter parsed + file written` for Obsidian).
- Mutations must **fail closed** on validation errors:	no partial writes.
- Rollback mechanism must be present (Lean: save and restore previous source, Obsidian: write to temp + atomic rename).

## 2. Config Schema for Backend Selection

### 2.1 Global Config Fields (~/.mcp-ai-helper/config.yaml)

```yaml
task_registry:
  backend: lean            # enum: lean, obsidian. Default: lean.
  obsidian:
    path: ""              # required when backend=obsidian. Path to Obsidian vault/task directory.
    vault: ""             # optional. Named vault for path resolution.
```

### 2.2 Per-Repo Override (.mcp-ai-helper.yaml)

```yaml
task_registry:
  backend: obsidian
  obsidian:
    path: "/path/to/user/vaults/my-project/tasks"
```

Per-repo config overrides global config. Both are merged via `MergeRepoConfig`.

### 2.3 Backend Selection Logic

```go
// Server dependency: taskBackend is set at startup via buildDeps.
// Current: always Lean. After this contract: config-based.
func buildDeps(cfg *config.Config) (..., taskBackend) {
    switch cfg.TaskRegistry.Backend {
    case "obsidian":
        return newObsidianTaskBackend(cfg, cmds, store)
    default: // "lean" or empty
        return newLeanTaskBackend(cmds, store)
    }
}
```

### 2.4 Config Validation Rules

| Condition | Behavior |
|----------|----------|
| backend unset or empty | Default to lean (backward compatible) |
| backend=lean | Use Lean registry; no extra config required |
| backend=obsidian, path set, dir exists | Use Obsidian backend; read/write .md files in path |
| backend=obsidian, path missing | Fail at startup: "task_registry.obsidian.path is required" |
| backend=obsidian, path not readable | Fail at startup: "task_registry.obsidian.path not readable" |
| backend=invalid | Fail at startup: "unsupported task_registry.backend: <v>" |
| backend=obsidian, dir empty | Ok; treat as empty registry (no tasks); writes create .md files |
| backend=obsidian, stale .md file | Fail on read: "unexpected frontmatter schema in <file>" |

### 2.5 No Auto-Detection

The backend is selected **solely by config**. No file probing, no format sniffing, no fallback chains.
If the user sets `backend: obsidian` and the path is broken, the helper fails at startup with a clear diagnostic,
not silently falling back to Lean.

## 3. Obsidian Markdown Schema

### 3.1 File Layout

One markdown file per task: `<task_id>.md`.
All files in the configured path are treated as task notes.
Subdirectories are **not** traversed; only direct children of the path are read.

Example directory:
```
tasks/
  obsidian-task-registry-backend.md  # epic (parent)
  obsidian-registry-capability-inventory.md  # child task
  lean-registry-backend-adapter.md
  ...
```

### 3.2 Frontmatter (YAML)

Every task note must have YAML frontmatter delimited by `---`.

#### 3.2.1 Required Fields

| Frontmatter Key | Type | Task Field | Validation |
|-----------------|------|-------------|------------|
| `id` | string | `ID` | Non-empty, same rules as `cleanTaskID` |
| `title` | string | `Title` | Non-empty, free text |
| `status` | string | `Status` | Enum: todo, in_progress, blocked, done |

Task notes missing any required fields MUST result in a structured error, not a silent default.

#### 3.2.2 Optional Fields

| Frontmatter Key | Type | Task Field | Default |
|-----------------|------|-------------|--------|
| `priority` | string | `Priority` | empty |
| `model_level` | string | `ModelLevel` | empty |
| `task_type` | string | `TaskType` | empty |
| `parent_id` | string | `ParentID` | empty |
| `tags` | []string | `Tags` | [] |
| `branch` | string | `Branch` | derived from task_type + id |
| `worktree_path` | string | `WorktreePath` | derived from id |
| `created_at` | string | `CreatedAt` | empty (backend sets on create) |
| `updated_at` | string | `UpdatedAt` | empty (backend sets on write) |

#### 3.2.3 Lean-Specific Fields (Preserved as-is)

| Frontmatter Key | Type | Task Field | Notes |
|-----------------|------|-------------|------|
| `acceptance_criteria` | []string | `AcceptanceCriteria` | Also in Markdown section below |
| `verification_plan` | []string | `VerificationPlan` | Also in Markdown section below |

These fields are dual-represented: both in frontmatter (as arrays) and in body sections (as bullet lists).
If both are present, frontmatter takes precedence. Writer must output both forms consistently.

### 3.3 Markdown Body Structure

The Markdown body after frontmatter is structured in sections:

```markdown
## Body

Task description text. Multiline supported.
Preserved exactly as written.

## Acceptance Criteria

- First criterion
- Second criterion

## Verification Plan

1. First verification step
2. Second verification step
```

The `Body` section maps to Task.Body.
The `Acceptance Criteria` section maps to Task.AcceptanceCriteria.
The `Verification Plan` section maps to Task.VerificationPlan.

### 3.4 Parsing Rules

1. **Frontmatter delimiters**: In YAML, `---` opens and `---` or `...` closes frontmatter. No nested delimiters in values.
2. **ID normalization**: Filename stem (before `.md`) MUST match `id` in frontmatter exactly.
3. **Status normalization**: Case-insensitive match, exported as lowercase.
4. **Model level normalization**: Same as `NormalizeModelLevel` (snake_case, valid enum).
5. **Priority normalization**: Case-insensitive, valid enum.
6. **Tags**: Frontmatter YAML array; each element trimmed and lowercased.
7. **Acceptance criteria/verification plan**: From frontmatter as YAML array; from body as bullet lists (lines starting with `- ` or `1. `).
8. **Body**: Everything from after frontmatter until the first `##` heading (Body section). From `#` to end of the Body section (until next `##`` or EOF).
9. **Escaping**: YAML frontmatter escaping as per yaml.v3 library. Markdown body is stored as-is without HTML rendering.

### 3.5 Writing Rules

1. **Deterministic output**: Same Task always produces the same .md file (byte-for-byte).
2. **Frontmatter key order**:
  1. `id`
  2. `title`
  3. `status`
  4. `priority`
  5. `model_level`
  6. `task_type`
  7. `parent_id`
  8. `tags`
  9. `branch`
  10. `worktree_path`
  11. `acceptance_criteria`
  12. `verification_plan`
  13. `created_at`
  14. `updated_at`
3. **Timestamps**: RFC3339Nano format, UTC.
4. **No extra whitespace trailing**: No trailing spaces, single trailing newline.
5. **Body sections present only when non-empty**. If Task.Body is empty, no `## Body` section is written.

### 3.6 Complete Example

```markdown
---
id: obsidian-task-registry-backend
title: Dobavit configurable Lean/Obsidian task registry backend bez poteri funktsionala
status: blocked
priority: high
model_level: very_high
task_type: epic
parent_id: null
tags:
  - tasks
  - registry
  - lean
  - obsidian
  - backend
  - config
  - import-export
  - epic
  - decomposed
branch: epic/obsidian-task-registry-backend
worktree_path: .worktrees/obsidian-task-registry-backend
acceptance_criteria:
  - Parent remains non-executable
  - User config exposes explicit backend selection
verification_plan:
  - Review child tasks for explicit Lean parity
created_at: 0001-01-01T00:00:00Z
updated_at: 0001-01-01T00:00:00Z
---

## Body

Parent/epic. Add a configurable task registry backend so the helper can operate against either the existing Lean registry or an Obsidian-style Markdown registry selected by user config.

## Acceptance Criteria

- Parent remains non-executable
- User config exposes explicit backend selection
- Lean remains the default when no backend is configured

## Verification Plan

1. Review child tasks for explicit Lean parity
2. Check config-selection task proves task_current/task_batch_upsert route to selected backend

```

## 4. Config Selection Diagram

```
startup/buildDeps
 |
v-- backend unset? --> use Lean (default)
 |
v-- backend=lean --> use Lean (same as default)
 |
v-- backend=obsidian, path set, readable --> use Obsidian
 |
v-- backend=obsidian, path missing --> FAIL at startup
 |
v-- backend=invalid --> FAIL at startup
```

## 5. Import/Export Contract

This section defines the import/export semantics that the `task-registry-import-export` task must implement.

#### 5.1 Operations

| Operation | Source | Target | Dry-run? |
|-----------|-------|-------|---------|
| export | Lean | Obsidian | Yes |
| import | Obsidian | Lean | Yes |
| export | Obsidian | Lean | No (already the same format) |
| import | Lean | Obsidian | No (already the same format) |

### 5.2 Loss Report

When a field cannot be represented in the target format, either:
- Fail closed with a compact loss report (default), or
- Skip the field with a warning (only if field is marked as optional and loss was explicitly accepted via a cli flag).

The loss report must include:
- Task ID
- Field name
- Original value
- Reason (e.g., "parent_id: Obsidian links are one-directional")

### 5.3 Conflict Detection

- Stale target: If the target registry has changed since the last read (checked via file hash or updated_at timestamp), fail with a conflict error.
- Duplicate ID: If a task with the same ID already exists in the target, fail with a conflict error unless --overwrite is set.
- Dry-run: Reports what would change without mutating anything.

## 6. Test Fixtures (contract)

These fixtures are part of the contract. Each implementation task must create tests derived directly from these.

#### Fixture 1: Valid Parent Epic

*Filename`: `obsidian-task-registry-backend.md`

*Expected behavior**: Parses successfully into `Task` with all fields populated.
Get() returns the task with `projection_source: obsidian_registry`.

#### Fixture 2: Executable Child Task

*Filename`: `lean-registry-backend-adapter.md`

*Expected behavior**: Parses successfully with `parent_id: obsidian-task-registry-backend`.
Full round-trip: Parse -> Task -> Write -> Parse preserves parent_id.

#### Fixture 3: Multiline Body

*Filename`: `task-with-multiline-body.md`

*Expected behavior**: Body content preserved exactly, including line breaks.
Round-trip does not add or remove blank lines.

#### Fixture 4: Invalid Frontmatter

*Filename`: `bad-frontmatter.md`

*Content*:
Version with missing closing `---`.

Value with unescaped colon in multiline section.

*Expected behavior**: Structured error: "frontmatter parse failed in bad-frontmatter.md: yaml: line X: mapping values are not allowed".

#### Fixture 5: Missing Required Fields

*Filename`: `missing-title.md`

*Expected behavior**: Structured error: "task in missing-title.md: required field 'title' is missing".

#### Fixture 6: Lossy Field (Import)

*Filename`: `lossy-lean-fields.md`

*Content*:
A task note with a field that Obsidian does not support (e.g., custom field like `foobar`: 123`).

*Expected behavior during import**: Loss report listing the unsupported fields. Fail-closed by default.

### Fixture 7: Round-trip (Lean -> Obsidian -> Lean)

*Expected behavior**: Export a Lean task with all fields to Obsidian .md, then import back to Lean. All fields preserved.

## 7. Backward Compatibility Guarantees

1. **Unchanged config**: If `task_registry.backend` is not set in config, behavior is identical to current.
2. **List{Current,All} return format**: The returned Task structs are identical regardless of backend.
3. **All existing task tools get their backend from `server.loadTaskBackend()` without changing their signatures.
4. **Mutation result type rename**: The rename of `leanMutationResult` to `taskMutationResult` is a compile-level change; the JSON wire format is unchanged.

## 8. Implementation Task Mapping

| Task | What it builds | Contract sections used |
|------|----------------|--------------------------|
| `lean-registry-backend-adapter` | Rename result types, preserve Lean path | _1; _7 |
| `obsidian-markdown-registry-backend`
| Implement parser/writer for Obsidian .md | _3; _6 |
| `task-registry-backend-config-selection` | Add config parsing, wire backend selection | _2; _4 |
| `task-registry-import-export` | Build import/export from _5 | _5 |
| `obsidian-registry-roundtrip-fixtures` | Add fixtures using _6 expectations | _6 |
