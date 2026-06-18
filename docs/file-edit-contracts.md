# File Edit Contracts

Implementation-ready matrix of safe, idempotent file edit operations.
Each operation guards on file hash, supports dry-run, and returns structured diagnostics.

## Common Contract

All operations share these invariants:

1. **Hash guard**: Every mutation requires `expected_hash` (from `ReadSnapshot`). If the file hash has changed since snapshot, the operation returns `status=conflict` with no mutation.
2. **Idempotency**: Re-applying the same operation to a file that already reflects the desired state returns `status=ok, changed=false` (not an error).
3. **Dry-run**: When `dry_run=true`, the operation computes the would-be result without writing. The response includes `diff` showing what would change.
4. **Path safety**: Paths must be repo-relative when `repo_path` is set. Absolute paths and `..` traversal are rejected.
5. **Encoding**: Text fields support `old/new` (JSON string) or `old_b64/new_b64` (base64-encoded) for content with backslashes or non-UTF8.

### Common Result Fields

```go
type EditResult struct {
    Status  string `json:"status"`       // ok, conflict, not_found, already_present, skipped
    Path    string `json:"path"`
    Changed bool   `json:"changed"`       // true if file was actually modified
    OldHash string `json:"old_hash"`      // hash before edit (empty if file didn't exist)
    NewHash string `json:"new_hash"`      // hash after edit (empty if dry_run or no change)
    Reason  string `json:"reason,omitempty"`
    Diff    string `json:"diff,omitempty"` // unified diff (populated on dry_run or conflict)
}
```

---

## Operations Matrix

### 1. GuardedReplace (EXISTING)

Replaces one unique text span, guarded by hash.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| expected_hash | string | yes | SHA-256 from snapshot |
| old / old_b64 | string | yes | Text to find (exactly one occurrence) |
| new / new_b64 | string | yes | Replacement text |
| dry_run | bool | no | If true, compute diff only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| Hash matches, old found once | ok | true | File updated |
| Hash matches, new already present | ok | false | Idempotent: desired state exists |
| Hash matches, old not found | conflict | false | Includes best partial match |
| Hash matches, old found >1 | conflict | false | Non-unique match |
| Hash mismatch | conflict | false | File changed since snapshot |
| File doesn't exist | conflict | false | Can't replace in missing file |

---

### 2. CreateIfAbsent (NEW)

Creates a file with content only if it doesn't exist.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| content / content_b64 | string | yes | Content for new file |
| mode | int | no | File permission (default 0644) |
| dry_run | bool | no | If true, compute result only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| File doesn't exist | ok | true | File created |
| File exists | already_present | false | No mutation, no error |
| Parent dir doesn't exist | error | false | Returns error, doesn't create dirs |

**Idempotency**: Calling twice is safe — second call returns `already_present`.

---

### 3. AppendUnique (NEW)

Appends text to end of file only if the exact text is not already present.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| expected_hash | string | yes | SHA-256 from snapshot |
| content / content_b64 | string | yes | Text to append |
| separator | string | no | Line separator before content (default: newline) |
| dry_run | bool | no | If true, compute diff only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| Hash matches, content not present | ok | true | Content appended |
| Hash matches, content already present | ok | false | Idempotent |
| Hash matches, file is empty | ok | true | Content written (no separator) |
| Hash mismatch | conflict | false | File changed since snapshot |
| File doesn't exist | conflict | false | Can't append to missing file |

**Detection**: Checks if `content` appears as a substring. For multi-line content, checks if the exact block exists anywhere in the file.

---

### 4. ReplaceByMarker (NEW)

Replaces content between marker comments.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| expected_hash | string | yes | SHA-256 from snapshot |
| start_marker | string | yes | Line marking block start (e.g. `<!-- BEGIN: config -->`) |
| end_marker | string | yes | Line marking block end (e.g. `<!-- END: config -->`) |
| new_content / new_content_b64 | string | yes | Replacement content between markers |
| dry_run | bool | no | If true, compute diff only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| Both markers found, content differs | ok | true | Block replaced |
| Both markers found, content same | ok | false | Idempotent |
| Start marker not found | conflict | false | Can't locate block |
| End marker not found | conflict | false | Malformed markers |
| Multiple start markers | conflict | false | Non-unique block |
| Hash mismatch | conflict | false | File changed since snapshot |

**Boundary**: Markers are matched as full lines (trimmed). The replacement preserves the marker lines themselves.

---

### 5. DeleteExactBlock (NEW)

Deletes an exact multi-line block from a file.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| expected_hash | string | yes | SHA-256 from snapshot |
| block / block_b64 | string | yes | Exact text block to remove |
| dry_run | bool | no | If true, compute diff only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| Block found once | ok | true | Block removed |
| Block not found | ok | false | Already absent (idempotent) |
| Block found >1 | conflict | false | Non-unique block |
| Hash mismatch | conflict | false | File changed since snapshot |

**Normalization**: Empty lines around the block are collapsed to avoid double-blank-lines after deletion.

---

### 6. JSONPatch (NEW)

Applies a structured merge to a JSON file.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| expected_hash | string | yes | SHA-256 from snapshot |
| patch | object | yes | Key-value pairs to merge (top-level only) |
| dry_run | bool | no | If true, compute diff only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| Hash matches, patch applies | ok | true | JSON updated |
| Hash matches, values already match | ok | false | Idempotent |
| Invalid JSON in file | error | false | Parse failure |
| Invalid patch value | error | false | Marshal failure |
| Hash mismatch | conflict | false | File changed since snapshot |

**Scope**: Top-level merge only. Nested object patching is out of scope for v1 (use GuardedReplace for nested changes).

---

### 7. YAMLPatch (NEW)

Applies a structured merge to a YAML file.

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| path | string | yes | File path |
| expected_hash | string | yes | SHA-256 from snapshot |
| patch | object | yes | Key-value pairs to merge (top-level only) |
| dry_run | bool | no | If true, compute diff only |

| Scenario | Status | Changed | Notes |
|----------|--------|---------|-------|
| Hash matches, patch applies | ok | true | YAML updated |
| Hash matches, values already match | ok | false | Idempotent |
| Invalid YAML in file | error | false | Parse failure |
| Hash mismatch | conflict | false | File changed since snapshot |

**Scope**: Top-level merge only. Comments and formatting may change on re-serialization.

---

## Status Priority

```
error > conflict > already_present / not_found / skipped > ok
```

- `error`: Input validation or IO failure. Never mutates.
- `conflict`: Guard mismatch (hash, uniqueness, markers). Never mutates.
- `already_present` / `not_found` / `skipped`: Idempotent no-op. Never mutates.
- `ok`: Operation succeeded (changed=true) or was idempotent (changed=false).

## Failure Modes (Counterexample Matrix)

| Scenario | GuardedReplace | CreateIfAbsent | AppendUnique | ReplaceByMarker | DeleteExactBlock | JSONPatch | YAMLPatch |
|----------|---------------|----------------|--------------|-----------------|------------------|-----------|----------|
| File missing | conflict | **ok (creates)** | conflict | conflict | conflict | conflict | conflict |
| Hash mismatch | conflict | n/a | conflict | conflict | conflict | conflict | conflict |
| Target not found | conflict | already_present | ok (appends) | conflict | ok (no-op) | ok (adds) | ok (adds) |
| Target found multiple | conflict | n/a | n/a | conflict | conflict | n/a | n/a |
| Dry run | diff only | result only | diff only | diff only | diff only | diff only | diff only |
| Bad encoding | error | error | error | error | error | error | error |

## Implementation Notes

1. **No global state**: Each operation is a pure function of inputs + file content.
2. **Atomic write**: Write to temp file, then rename. Prevents partial writes on crash.
3. **B64 priority**: When both `old` and `old_b64` are set, `old_b64` wins (same as current GuardedReplace).
4. **Diff format**: Unified diff with 3 lines of context. Only populated on dry_run or conflict.
5. **Backward compatibility**: Existing `ApplyGuardedReplace` signature unchanged. New operations are additive.
