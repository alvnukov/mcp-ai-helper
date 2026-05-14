# Task Registry Capability Inventory — Parity Matrix

> Generated: 2026-05-14 | Source: MCP helper task tools + Go source inspection
> For: obsidian-registry-backend-contract task

## 1. Canonical Task Record Fields

| # | Field | Go Type | Required | Source | Obsidian Parity | Notes |
|---|-------|---------|----------|---------|-----------------|------|
| 1 | `id` | string | **required** | user/derived | **required** | Cleaned: lowercase, `ineverything-is-fine  ` | 
 | 2 | `title` | string | **required** | user | **required** | Free text, trimmed |
| 3 | `status` | string | required | user/system | **required** | Enum: `todo`, `in_progress`, `blocked`, `done` |
| 4 | `body` | string | optional | user | **required** | Multiline Markdown Body |
| 5 | `priority` | string | optional | user | **required** | Enum: `low`, `medium`, `high`, `critical` |
| 6 | `model_level` | string | optional | user | **required** | Enum: `low`, `medium`, `high`, `very_high`; normalized (snake_case) |
| 7 | `task_type` | string | optional | user | **required** | Enum: `feature`, `bug`, `hotfix`, `chore`, `docs`, `refactor`, `test`, `ci`, `design`, `epic`, `implementation`, `tests` |
| 8 | `parent_id` | string | optional | user | **required** | Links to parent task; empty = root |
| 9 | `tags` | []string | optional | user | **required** | Free-form, lowercase normalized |
| 10 | `acceptance_criteria` | []string | optional | user | **required** | Structured list |
| 11 | `verification_plan` | []string | optional | user | **required** | Structured list |
| 12 | `branch` | string | derived | system | **derived** | `<task_type>/<id>`; enforced by NormalizeWorktreeFields |
| 13 | `worktree_path` | string | derived | system | **derived** | `.worktrees/<id`; enforced by NormalizeWorktreeFields |
| 14 | `code_path` | string | derived | system | **derived** | `$REPO_ROOT/.worktrees/<id>`; from WithWorktreeContext |
| 15 | `worktree_exists` | bool | derived | system | **derived** | `os.Stat` check; from WithWorktreeContext |
| 16 | `projection_source` | string | derived | system | **derived** | `lean_registry` for Lean; would be `obsidian_registry` for Obsidian |
| 17 | `created_at` | time.Time | derived | system | **derived** | RPC3339Nano; stored as string in Lean source |
| 18 | `updated_at` | time.Time | derived | system | **derived** | RPC3339Nano; stored as string in Lean source |

## 2. Task Statuses

| Status | Semantics | Obsidian Parity |
|--------|-----------|-----------------|
| `todo` | Ready for execution | **required** |
| `in_progress` | Currently being executed | **required** |
| `blocked` | Waiting for external input/dependency | **required** |
| `done` | Completed and verified | **required** |

## 3. Task Types

| Type | Branch Prefix | Obsidian Parity |
|------|---------------|-----------------|
| `feature` | `feature/` | **required** |
| `bug` | `bug/` | **required** |
| `hotfix` | `hotfix/` | **required** |
| `chore` | `chore/` | **required** |
| `docs` | `docs/` | **required** |
| `refactor` | `refactor/` | **required** |
| `test` | `test/` | **required** |
| `ci` | `ci/` | **required** |
| `design` | `design/` | **required** |
| `epic` | `epic/` | **required** |
| `implementation` | `implementation/` | **required** |
| `tests` | `tests/` | **required** |

## 4. TaskBackend Interface Contract

| Method | Input | Output | Obsidian Parity |
|-------|-------|-------|-----------------|
| `ListCurrent` | repoPath | ([]Task, source, error) | **required** — returns active (todo + in_progress) tasks |
| `ListAll` | repoPath | ([]Task, source, error) | **required** — returns all tasks |
| `Get` | repoPath, id | (Task, source, error) | **required** — single task lookup |
| `Upsert` | AddRequest | (mutationResult, error) | **required** — create or replace |
| `SetStatus` | StatusRequest | (mutationResult, error) | **required** ℓ transition with rollback support |
| `BatchUpsert` | BatchUpsertRequest | (batchMutationResult, error) | **required** — bulk sync with close_missing |
| `Delete` | DeleteRequest | (mutationResult, error) | **required** ℓ single task deletion |

### Mutation Result Types

| Go Type | Fields | Obsidian Parity |
|---------|-------|-----------------|
| `leanMutationResult` | Task, Source, Validation, ChangedFiles | **required** — rename to `taskMutationResult` |
| `leanBatchMutationResult` | Upserted, Closed, Source, Validation | **required** ℓ rename to `taskBatchMutationResult` |

## 5. Task Tool Surface → Backend Mapping

| Tool | Backend Method | Config-Dependent |
|------|---------------|-----------------|
| `task_add` | Upsert | yes |
| `task_upsert` | Upsert | yes |
| `task_update` | Get → merge → Upsert | yes |
| `task_batch_upsert` | BatchUpsert | yes |
| `task_set_status` | SetStatus | yes |
| `task_current` | ListCurrent | yes |
| `task_list` | ListAll (+ filter) | yes |
| `task_search` | ListAll (+ filter) | yes |
| `task_get` | Get | yes |
| `task_delete` | Delete | yes |
| `task_graph` | ListAll → BuildTaskGraph | yes |
| `task_context` | ListAll → BuildTaskContext | yes |
| `task_tree` | ListAll → buildTaskTree | yes |

## 6. Current Architecture (Lean Registry)

| Component | Detail |
|-----------|--------|
| Storage | MCPAIHelperProject/ActiveTasks — embedded template source file |
| Read RPC | `TaskRegistryExport.taskList` / `TaskRegistryExport.taskGet` via Lake server |
| Write RPC | `TaskRegistryExport.taskUpsertApply` / `TaskRegistryExport.taskBatchUpsertApply` / `TaskRegistryExport.taskTransitionApply` / `TaskRegistryExport.taskDeleteApply` |
| Bootstrap | Embedded Go templates | 
| Pre-mutation gate | lake build validation |
| Post-mutation gate | lake build validation; rollback via `activeTasksWrite` on failure |
| Source tracking | `PreviousSource` preserved for rollback in mutation payloads |
| Schema version | `schema_version: 1` enforced in RPC envelopes |

## 7. Current Config Surface

| Field | Exists | Notes |
|------|--------|------|
| `layers.tasks.enabled` | yes | Bool toggle for task layer |
| `task_registry_backend` | **no** | Must be added; enum: `lean`, `obsidian` |
| `task_registry.obsidian_path` | **no** | Required when backend=obsidian |
| `task_registry.obsidian_vault` | **no** | Optional vault name setting |
| `repo_config` (.mcp-ai-helper.yaml) | yes | Per-repo overrides; backend selection must support repo scope |

## 8. Workflow & Safety Semantics

| Semantic | Current Behavior | Obsidian Parity |
|----------|------------------|-----------------|
| Pre-mutation build check | lake build before write | **equivalent gate needed** (e.g., file consistency check) |
| Post-mutation build check | lake build after write | **equivalent gate needed** |
| Rollback on failure | `activeTasksWrite` with previous source | **required** — atomic write with rollback |
| Conflict detection | Schema version check in envelope | **required** — stale edit detection |
| Validation reporting | `Validation.Checked` + `Summary` | **required** — structured diagnostics |
| Changed files tracking | `ChangedFiles` in mutation envelopes | **required** — for commit scoping |
| ID normalization | `cleanTaskID`: lowercase, sanitize | **required** — same rules |
| Branch/worktree derivation | `NormalizeWorktreeFields` enforces `<type>/<id>` and `.worktrees/<id>` | **required** — same rules |
| Model level normalization | `NormalizeModelLevel`: snake_case, valid enum | **required** — same rules |

## 9. Confirmed vs Assumed vs Unknown

### Confirmed (from code inspection)
- All 18 Task struct fields and their types
- taskBackend interface with 7 methods
- Lean RPC read/write pipeline (lake serve → TaskRegistryExport)
- Bootstrap template list and embedding
- Pre/post mutation lake build gates
-  Rollback mechanism on build failure
- ID/branch/model_level normalization rules
- Config schema: no backend selection field exists yet
- All 13 task MCP tools and their backend method routing
- Worktree context derivation (WithWorktreeContext)

### Assumed (reasonable, need contract validation)
- Obsidian will use one-markdown-note-per-task layout
- Import/export will be separate from backend CRUD
- Config will use `task_registry_backend` field name (not yet confirmed by contract task)
- Per-repo config overrides global config for backend selection

### Unknown (blockers for implementation)
- Exact Obsidian vault path resolution rules (contract task must define)
- Whether Obsidian backend needs Lean/Lake workspace at all
- Frontmatter parser library choice (yaml.v3 vs custom)
- How `ChangedFiles` maps to Obsidian (file-per-task vs directory)
- Whether `close_missing` in batch_upsert applies to Obsidian notes (delete? archive?)

## 10. Test/Fixture Checklist for Implementation Tasks

#### For `lean-registry-backend-adapter`
- [ ] Existing task_current returns same tasks before/after adapter introduction
- [ ] Existing task_batch_upsert behavior unchanged
- [ ] taskBackend interface unchanged except result type renames
- [ ] `go test ./internal/mcp/...` passes
- [ ] `go vet ./internal/mcp/...` passes

#### For `obsidian-markdown-registry-backend`
- [ ] Parse valid parent epic note → canonical Task with all fields
- [ ] Parse valid child task note → canonical Task with parent_id set
- [ ] Parse note with multiline body → body preserved exactly
- [ ] Parse note with acceptance_criteria and verification_plan → arrays preserved
- [ ] Parse note with tags and model_level → fields populated
- [ ] Parse invalid/malformed frontmatter → structured error
- [ ] Parse note missing required fields → structured error
- [ ] Write canonical Task → deterministic Markdown
- [ ] Round-trip: Task → Write → Parse → Task (all fields equal)
- [ ] Round-trip: parent+child → Write → Parse → parent_id relationship preserved

#### For `task-registry-backend-config-selection`
- [ ] Unset backend → uses Lean (backward compatible)
- [ ] backend=lean → uses Lean explicitly
- [ ] backend=obsidian with valid path → uses Obsidian
- [ ] backend=obsidian with missing path → structured error
- [ ] backend=invalid → structured error
- [ ] task_current routes to selected backend
- [ ] task_batch_upsert routes to selected backend

#### For `task-registry-import-export`
- [ ] Lean → Obsidian dry-run produces expected Markdown files
- [ ] Obsidian → Lean dry-run validates without mutation
- [ ] Round-trip Lean → Obsidian → Lean preserves all required fields
- [ ] Lossy field scenario → fail-closed with loss report
- [ ] Duplicate ID in target → conflict error
- [ ] Stale target (concurrent edit) → conflict error

#### For `obsidian-registry-roundtrip-fixtures`
- [ ] Fixture: parent epic with all fields
- [ ] Fixture: child task with parent_id
- [ ] Fixture: task with multiline body
- [ ] Fixture: task with Lean-specific fields that must not be silently dropped
- [ ] At least one fixture fails if a required Lean field is dropped

### Lint/Check Commands for Implementation Tasks
```sh
go test ./internal/mcp/...
go test ./internal/tasks/...
go vet ./internal/...
```
