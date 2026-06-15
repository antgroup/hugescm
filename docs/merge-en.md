# Three-Way Merge in Zeta

## Abstract

This document describes the design and implementation of three-way merge in [Zeta](https://github.com/nicedoc/zeta) (HugeSCM), a Git-compatible version control system built in Go. The implementation covers the full merge pipeline: tree-level diff computation, rename detection, file/directory conflict resolution, and text-level three-way merge using the diff3 algorithm.

The text merge layer (`modules/diferenco`) is a standalone, reusable Go package that implements the diff3 algorithm with multiple diff backends (Histogram, Myers, ONP, Patience, Minimal) and multiple conflict marker styles (merge, diff3, zdiff3). It supports automatic charset detection and transcoding for non-UTF-8 files.

This work may be relevant to the [go-git merge discussion (issue #942)](https://github.com/go-git/go-git/issues/942) as a reference implementation demonstrating how to build a complete three-way merge in pure Go without shelling out to `git merge-file`.

### Key Features

- **Pure Go diff3 implementation** — no external dependencies on `git merge-file` or `diff3` binary (though both are supported as optional external drivers)
- **Multiple diff algorithms** — Histogram (default), Myers, ONP, Patience, Minimal, SuffixArray
- **Multiple conflict styles** — merge (default), diff3, zdiff3 (zealous diff3)
- **Charset-aware merging** — automatic detection and transcoding for GBK, Shift-JIS, etc.
- **Tree-level merge** — rename detection, file/directory conflicts, mode conflicts
- **Binary file handling** — content sniffing + size threshold (50 MiB)
- **Pluggable architecture** — custom merge drivers and text resolvers via function types

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────┐
│  Application Layer (pkg/zeta/)                               │
│  merge_tree.go — CLI entry, merge-base resolution, output    │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Tree Merge Layer (pkg/zeta/odb/)                            │
│  merge.go       — three-way tree merge orchestration         │
│  merge_driver.go — text merge dispatch + charset restoration │
│  merge_text.go  — external merge tool integration            │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  Text Merge Layer (modules/diferenco/)                       │
│  merge.go    — diff3 algorithm, conflict detection & output  │
│  diferenco.go — diff algorithms (Histogram, Myers, ONP, ...) │
│  text.go     — charset detection, binary detection           │
│  sink.go     — line deduplication & indexing                  │
└──────────────────────────────────────────────────────────────┘
```

---

## Part 1: Text-Level Three-Way Merge (`modules/diferenco`)

### Algorithm

The text merge uses the classic **diff3** algorithm as described in:

> Sanjeev Khanna, Keshav Kunal, and Benjamin C. Pierce.
> "A Formal Investigation of Diff3." FSTTCS 2007.
> http://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf

The implementation is ported from [node-diff3](https://github.com/bhousel/node-diff3) (JavaScript) by [epiclabs-io/diff3](https://github.com/epiclabs-io/diff3) (Go), with significant enhancements.

#### Steps

1. **Line indexing**: Text is split into lines and deduplicated via a `Sink` structure. Each unique line maps to an integer index, so diff algorithms operate on `[]int` rather than `[]string`.

2. **Two-way diffs**: Compute `diff(O, A)` and `diff(O, B)` using the selected algorithm (default: Histogram).

3. **Hunk merging**: Merge the two diff results into a unified timeline on O. Overlapping hunks from both sides indicate potential conflicts.

4. **Conflict detection**: For each overlapping region, check if A and B made the same change (false conflict elimination). True conflicts are reported with their A/O/B spans.

5. **Output generation**: Non-conflicting regions are emitted directly. Conflicts are formatted with conflict markers according to the selected style.

### Conflict Styles

```go
const (
    STYLE_DEFAULT      = iota  // <<<<<<< / ======= / >>>>>>>
    STYLE_DIFF3                // <<<<<<< / ||||||| / ======= / >>>>>>>
    STYLE_ZEALOUS_DIFF3        // minimized A/B hunks + full O context
)
```

**Default (merge)**:
```
<<<<<<< ours
line-from-ours
=======
line-from-theirs
>>>>>>> theirs
```

**diff3**: Shows the base version between `|||||||` markers.

**zdiff3 (zealous diff3)**: Like diff3 but minimizes the A/B hunks by extracting common prefix/suffix outside the markers.

### API

```go
// High-level: merge with default options (Histogram algorithm, merge style)
func DefaultMerge(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error)

// Full control: specify algorithm and conflict style
func Merge(ctx context.Context, opts *MergeOptions) (string, bool, error)

// Quick check: does merging these texts produce conflicts?
func HasConflict(ctx context.Context, textO, textA, textB string) (bool, error)
```

### Diff Algorithms

| Algorithm | Description | Best For |
|-----------|-------------|----------|
| Histogram | Frequency-based LCS (like JGit) | General purpose (default) |
| Myers | Classic O(ND) algorithm | Small diffs |
| ONP | O(NP) algorithm by Wu et al. | Balanced edits |
| Patience | Patience diff (unique line matching) | Code with many repeated lines |
| Minimal | Bounded two-sided Myers | Minimal edit scripts |
| SuffixArray | Suffix array based LCS | Large files with repetition |

### Binary Detection

Files are classified as binary using two heuristics:
1. **NUL-byte scan**: First 8000 bytes are checked for NUL bytes (same as Git)
2. **Size threshold**: Files exceeding 100 MiB are rejected (`MAX_DIFF_SIZE`)

### Charset Handling

When `textconv` is enabled:
1. MIME type detection on the first 8000 bytes
2. Charset extraction from MIME parameters
3. Transcoding to UTF-8 for diff/merge operations
4. After merge, the result is encoded back to the original charset (ours side)

Supported charsets include UTF-8, GBK, GB18030, Shift-JIS, EUC-KR, ISO-8859-*, Windows-125*, and others via the `chardet` module.

---

## Part 2: Tree-Level Three-Way Merge (`pkg/zeta/odb`)

### Entry Point

```go
func (d *ODB) MergeTree(ctx context.Context, o, a, b *object.Tree, opts *MergeOptions) (*MergeResult, error)
```

Parameters:
- `o` — ancestor (merge-base) tree
- `a` — "ours" tree
- `b` — "theirs" tree
- `opts` — merge configuration

### Merge Pipeline

#### Step 1: Compute Differences

```go
func (d *ODB) mergeDifferences(ctx context.Context, o, a, b *object.Tree) (*differences, error)
```

Performs `DiffTree(O, A)` and `DiffTree(O, B)` with exact rename detection enabled. The results are merged into a unified `differences` structure:

```go
type differences struct {
    entries map[string]*ChangeEntry  // path → {Ancestor, Our, Their}
    renames map[string]*RenameEntry  // original path → rename info
    ours    map[string]bool          // paths modified by ours
    theirs  map[string]bool          // paths modified by theirs
}
```

The `overrideOur` method processes ours changes first, then `overrideTheir` merges theirs changes into the existing entries. This two-pass approach correctly handles cases where both sides modify the same file.

#### Step 2: Detect Rename Conflicts

For each rename entry, check if both sides renamed the same file to different targets:

```go
func (e *RenameEntry) conflict() bool {
    return e.Our != nil && e.Their != nil && !e.Our.Equal(e.Their)
}
```

If so, report `CONFLICT (rename/rename)`.

#### Step 3: Detect File/Directory Name Conflicts

```go
func (d *differences) nameConflicts() map[string]string
```

Detects cases where one side adds a file `foo` and the other adds a directory `foo/`. Resolution: rename the file to `foo~<branch_name>` and report `CONFLICT (file/directory)`.

#### Step 4: Fast-Path Resolution

For each changed entry, attempt resolution without text merge:

```
Ancestor == Our   → accept Theirs (ours didn't change)
Ancestor == Their → accept Ours (theirs didn't change)
Our == Their      → accept either (both made same change)
Otherwise         → requires text merge
```

#### Step 4.1: Merge-Side Decision Details

The following is the complete decision tree for each `ChangeEntry` in the `MergeTree` main loop:

```
For each entry e in differences.entries:
│
├─ e.Ancestor == e.Our (ours unchanged)
│   ├─ e.Their != nil → accept Their version (theirs' modification takes effect)
│   └─ e.Their == nil → delete file (theirs deleted it, ours didn't touch it)
│
├─ e.Ancestor == e.Their (theirs unchanged)
│   ├─ e.Our != nil → accept Our version (ours' modification takes effect)
│   └─ e.Our == nil → delete file (ours deleted it, theirs didn't touch it)
│
├─ e.Our == e.Their (both made identical changes)
│   ├─ e.Our != nil → accept Our version (both agree, pick either)
│   └─ e.Our == nil → delete file (both deleted it)
│
└─ Otherwise (true conflict, enter mergeEntry)
    │
    ├─ e.Ancestor == nil (both sides added the same path)
    │   ├─ Our.Hash == Their.Hash → CONFLICT(distinct modes), keep ours
    │   ├─ binary/Fragments/large file → CONFLICT(binary), keep ours
    │   └─ text file → three-way merge with empty blob as base
    │       ├─ merge succeeds → accept merged result
    │       └─ has conflicts → CONFLICT(add/add), result contains markers
    │
    ├─ e.Our != nil && e.Their != nil (both modified)
    │   ├─ Our.Hash == Their.Hash → CONFLICT(distinct modes), keep ours
    │   ├─ binary/Fragments/large file → CONFLICT(binary), keep ours
    │   └─ text file → three-way merge with Ancestor as base
    │       ├─ merge succeeds, no mode conflict → accept result + new mode
    │       ├─ merge succeeds, mode conflict → CONFLICT(distinct modes)
    │       └─ content conflict → CONFLICT(content), result contains markers
    │
    └─ e.Our == nil || e.Their == nil (one deletes, other modifies)
        ├─ Our == nil → CONFLICT(modify/delete): ours deleted, theirs modified
        └─ Their == nil → CONFLICT(modify/delete): theirs deleted, ours modified
        └─ keep the modified side's version
```

**File mode (filemode) decision logic**:

When both sides modified a file and content merge succeeds, the final file mode is determined by:

```
Ancestor.Mode == Our.Mode   → use Their.Mode (ours didn't change mode)
Ancestor.Mode == Their.Mode → use Our.Mode (theirs didn't change mode)
Our.Mode != Their.Mode      → CONFLICT(distinct modes), use Our.Mode
Our.Mode == Their.Mode      → use Our.Mode (both changed to same mode)
```

**Default retention policy on conflicts**:

For all conflicts that cannot be auto-resolved, the result tree retains files according to:

| Conflict Type | Version Retained in Result Tree |
|---------------|-------------------------------|
| Content conflict | Merged text (with `<<<<<<<` conflict markers) |
| Binary conflict | Ours side |
| Mode conflict | Ours side |
| Modify/delete conflict | The modified side (regardless of ours/theirs) |
| File/directory conflict | File renamed to `path~branch_name` |
| Rename/rename conflict | Both sides' renames preserved |

**Rename decision logic**:

```
For each entry e in differences.renames:
│
├─ e.Our == nil (only theirs renamed)
│   → no conflict, theirs' rename takes effect
│
├─ e.Their == nil (only ours renamed)
│   → no conflict, ours' rename takes effect
│
├─ e.Our.Path == e.Their.Path (both renamed to same target)
│   → no conflict, accept either
│
└─ e.Our.Path != e.Their.Path (both renamed to different targets)
    → CONFLICT(rename/rename)
```

**File/directory name conflict decision logic**:

```
For each conflict (file_path, dir_path) in nameConflicts:
│
├─ file_path comes from theirs side
│   → rename file to file_path~<Branch2>
│
└─ file_path comes from ours side
    → rename file to file_path~<Branch1>
│
└─ original path removed from entries, new path added
└─ files under the directory are preserved unaffected
```

#### Step 5: Text Merge (mergeEntry)

For entries requiring merge, classify and handle:

| Scenario | Handling |
|----------|----------|
| Both add (ancestor=nil), same hash | Mode conflict only |
| Both add, binary/fragments/large | Binary conflict, keep ours |
| Both add, text | Merge with empty base |
| Both modify, same hash | Mode conflict only |
| Both modify, binary/large | Binary conflict, keep ours |
| Both modify, text | Three-way text merge |
| One deletes, other modifies | modify/delete conflict |

#### Step 6: Build Result Tree

Collect all resolved entries and build a new tree object via `treeMaker.makeTrees()`.

### Pluggable Components

#### MergeDriver

```go
type MergeDriver func(ctx context.Context, o, a, b string, labelO, labelA, labelB string) (string, bool, error)
```

The function that performs actual text merging. Default: `diferenco.DefaultMerge`. Can be replaced with:
- `odb.ExternalMerge` — shells out to `git merge-file`
- `odb.Diff3Merge` — shells out to `diff3`
- Custom drivers for specific file types (e.g., XML merge, JSON merge)

#### TextResolver

```go
type TextResolver func(ctx context.Context, oid plumbing.Hash, textconv bool) (string, string, error)
```

Reads a blob by OID and returns its text content + detected charset. The default implementation reads from local storage; in partial-clone scenarios, it can trigger on-demand fetch of missing objects.

### Conflict Types

```go
const (
    INFO_AUTO_MERGING                        = iota
    CONFLICT_CONTENTS                        // text content conflict
    CONFLICT_BINARY                          // binary file conflict
    CONFLICT_FILE_DIRECTORY                  // file vs directory name clash
    CONFLICT_DISTINCT_MODES                  // file mode (permission) conflict
    CONFLICT_MODIFY_DELETE                   // one side modified, other deleted
    CONFLICT_RENAME_RENAME                   // renamed to different targets
    CONFLICT_RENAME_COLLIDES                 // rename target collides
    CONFLICT_RENAME_DELETE                   // renamed on one side, deleted on other
    CONFLICT_DIR_RENAME_SUGGESTED            // directory rename suggested
    INFO_DIR_RENAME_APPLIED                  // directory rename applied
    INFO_DIR_RENAME_SKIPPED_DUE_TO_RERENAME  // skipped due to re-rename
    CONFLICT_DIR_RENAME_FILE_IN_WAY          // file blocking directory rename
    CONFLICT_DIR_RENAME_COLLISION            // directory rename collision
    CONFLICT_DIR_RENAME_SPLIT                // unclear directory rename split
)
```

### MergeResult

```go
type MergeResult struct {
    NewTree   plumbing.Hash  // merged tree OID (always produced, even with conflicts)
    Conflicts []*Conflict    // list of conflicts (empty = clean merge)
    Messages  []string       // human-readable merge messages
}
```

The merged tree is always produced (with conflict markers embedded in text files), similar to `git merge-tree`. Callers check `len(result.Conflicts) > 0` to determine if manual resolution is needed.

---

## Part 3: Merge-Base Resolution

When multiple merge-bases exist (criss-cross merges), Zeta recursively merges them:

```go
func (r *Repository) resolveAncestorTree0(ctx context.Context, into, from *object.Commit, ...) (*object.Tree, error) {
    bases, _ := into.MergeBase(ctx, from)
    switch len(bases) {
    case 0:
        return r.odb.EmptyTree(), nil  // unrelated histories
    case 1:
        return bases[0].Root(ctx)      // single merge-base
    default:
        // Recursive: merge the merge-bases first
        return r.resolveAncestorTree0(ctx, bases[0], bases[1], ...)
    }
}
```

This matches Git's "recursive" merge strategy behavior.

---

## Part 4: Differences from Git

| Feature | Git (merge-ort) | Zeta |
|---------|----------------|------|
| Rename detection | Fuzzy matching (similarity score) | Exact match only |
| Merge strategies | ours, theirs, octopus, subtree | Default only |
| Conflict style | merge, diff3, zdiff3 | merge, diff3, zdiff3 |
| Diff algorithms | Myers, Histogram, Patience, Minimal | Histogram, Myers, ONP, Patience, Minimal, SuffixArray |
| Large file handling | Git LFS (external) | Built-in Fragments object detection |
| Charset handling | None (assumes UTF-8) | Auto-detect and transcode |
| Binary threshold | Content-based only | 50 MiB size + content detection |
| Hash algorithm | SHA-1 / SHA-256 | BLAKE3 |
| Implementation | C (libgit2) / C (git) | Pure Go |

---

## Part 5: Usage Examples

### Programmatic Usage (Go)

```go
import (
    "context"
    "github.com/antgroup/hugescm/modules/diferenco"
    "github.com/antgroup/hugescm/pkg/zeta/odb"
)

// Text-level merge (standalone, no storage needed)
merged, hasConflict, err := diferenco.DefaultMerge(ctx,
    baseText, oursText, theirsText,
    "base", "ours", "theirs",
)

// Text-level merge with options
merged, hasConflict, err := diferenco.Merge(ctx, &diferenco.MergeOptions{
    TextO:  baseText,
    TextA:  oursText,
    TextB:  theirsText,
    LabelO: "base",
    LabelA: "ours",
    LabelB: "theirs",
    A:      diferenco.Histogram,
    Style:  diferenco.STYLE_ZEALOUS_DIFF3,
})

// Tree-level merge (requires ODB for blob storage)
result, err := db.MergeTree(ctx, ancestorTree, oursTree, theirsTree, &odb.MergeOptions{
    Branch1:      "main",
    Branch2:      "feature",
    Textconv:     true,
    MergeDriver:  diferenco.DefaultMerge,
    TextResolver: readBlobText,
})
if len(result.Conflicts) > 0 {
    // Handle conflicts
    for _, c := range result.Conflicts {
        fmt.Printf("CONFLICT: %s (type=%d)\n", c.Our.Path, c.Types)
    }
}
// result.NewTree is the merged tree OID
```

### Command Line

```shell
# Three-way merge of two branches
zeta merge-tree branch1 branch2

# With explicit merge-base
zeta merge-tree --merge-base=base branch1 branch2

# JSON output (for tooling integration)
zeta merge-tree --json branch1 branch2

# After conflicts: resolve manually or force one side
zeta checkout <rev> -- <file>
```

---

## Source File Index

| File | Layer | Responsibility |
|------|-------|---------------|
| `modules/diferenco/merge.go` | Text | diff3 algorithm, conflict markers, merge styles |
| `modules/diferenco/diferenco.go` | Text | Diff algorithms (Histogram, Myers, ONP, etc.) |
| `modules/diferenco/text.go` | Text | Charset detection, binary detection, transcoding |
| `modules/diferenco/sink.go` | Text | Line deduplication and integer indexing |
| `pkg/zeta/odb/merge.go` | Tree | Tree-level merge orchestration, conflict detection |
| `pkg/zeta/odb/merge_driver.go` | Tree | Text merge dispatch, charset restoration |
| `pkg/zeta/odb/merge_text.go` | Tree | External merge tool integration (git merge-file, diff3) |
| `pkg/zeta/merge_tree.go` | App | CLI entry, merge-base resolution, output formatting |

---

## References

- [A Formal Investigation of Diff3](http://www.cis.upenn.edu/~bcpierce/papers/diff3-short.pdf) — Khanna, Kunal, Pierce (2007)
- [Merging with Diff3](https://blog.jcoglan.com/2017/05/08/merging-with-diff3/) — James Coglan
- [node-diff3](https://github.com/bhousel/node-diff3) — Original JavaScript implementation
- [epiclabs-io/diff3](https://github.com/epiclabs-io/diff3) — Go port
- [Git merge-ort](https://git-scm.com/docs/git-merge) — Git's merge strategy for comparison
