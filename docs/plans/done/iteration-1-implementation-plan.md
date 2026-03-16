# Iteration 1 — Implementation Plan

## Goal

Implement `$MODELR_PATH` resolution, the definition cache, and the `definitions` CLI command. This gives practitioners the ability to maintain custom definition libraries on disk, cache them for performance, and inspect which definitions are available.

## Prerequisites

Iteration 0 is complete. The following exist and work:
- Core types (`internal/model/types.go`) — `SystemModel`, `Component`, `Edge`, etc.
- Parser (`internal/model/parser.go`) — `Parse` returns `ParseResult` with model + inline definitions
- Loader (`internal/loader/`) — `Registry`, `BuildRegistry(inline, inlineRels)`, `LoadEmbedded()`
- Validator (`internal/model/validator.go`) — `Validate(model, registry)`
- Expression evaluator (`internal/check/expr.go`) — `EvaluateComparison`, `EvaluateNumeric`
- Checker (`internal/check/checker.go`) — `Check(model, registry)`
- CLI (`cmd/modelr/main.go`) — `check` and `validate` commands with `--verbose`
- Embedded definitions (`embed/`) — server, datastore, queue nodes; capacity_chain, pooled_capacity_chain relationships

## Scope

### In scope

| Spec Section | What |
|---|---|
| 7.2 | `$MODELR_PATH` resolution — colon-separated dirs, first-match-wins |
| 7.3 | Loading behavior — walk dirs, parse multi-doc YAML, index by kind+name |
| 7.4 | Verbose output — show where each definition was loaded from |
| 7.6 | Definition cache — `$HOME/.modelr/cache/definitions.json`, staleness detection, auto-cache, shadowing |
| 6.1 | `modelr cache status` and `modelr cache refresh` commands |
| 6.1 | `modelr definitions` command |

### Deferred

| Feature | Why |
|---|---|
| Visualization (10) | Independent feature, iteration 2 |
| Behavioral verification (8) | Complex subsystem, iteration 2+ |
| MCP server (6.2) | Integration layer, iteration 2 |

## File Layout

New and modified files:

```
internal/loader/
  types.go          # MODIFIED: Source field on defs, ShadowEvent, DefinitionSources, Cache types
  loader.go         # MODIFIED: BuildRegistry accepts DefinitionSources struct
  loader_test.go    # MODIFIED: update tests for new BuildRegistry signature
  path.go           # NEW: ParseModelrPath, ScanDirectory, ParseDefinitionFile, LoadFromPath
  path_test.go      # NEW
  cache.go          # NEW: Cache I/O, staleness, BuildCache
  cache_test.go     # NEW
cmd/modelr/
  main.go           # MODIFIED: cache + definitions commands, pipeline uses full loader
  main_test.go      # MODIFIED: new command tests + integration tests
```

---

## Phase 1 — Path Scanning [COMPLETE]

Load definitions from `$MODELR_PATH` directories. No cache yet — this is the raw filesystem capability that the cache optimizes.

### Step 1.1: Parse `$MODELR_PATH` (`internal/loader/path.go`) [x]

**Test (red):** `internal/loader/path_test.go`
- `TestParseModelrPathEmpty` — empty string → empty slice
- `TestParseModelrPathUnset` — empty string (simulating unset env) → empty slice
- `TestParseModelrPathSingle` — `"/opt/defs"` → `["/opt/defs"]`
- `TestParseModelrPathMultiple` — `"/opt/defs:/home/user/defs:/proj/defs"` → 3 entries in order
- `TestParseModelrPathSkipsEmptySegments` — `"/opt/defs::/home/user/defs"` → 2 entries, empty segment dropped

**Code (green):**
```go
func ParseModelrPath(envValue string) []string
```
Split on `:`, filter empty strings.

**Refactor:** None expected.

### Step 1.2: Scan directory for YAML files (`internal/loader/path.go`) [x]

**Test (red):** `internal/loader/path_test.go`
- `TestScanDirectoryFindsYAML` — dir with `a.yaml`, `b.yaml` → returns both paths, sorted alphabetically
- `TestScanDirectoryIgnoresNonYAML` — dir with `.txt`, `.json`, `.yml` files → empty result (spec says `*.yaml` only)
- `TestScanDirectoryDoesNotRecurse` — subdir containing `.yaml` files → not included
- `TestScanDirectoryNotExist` — nonexistent dir → error
- `TestScanDirectoryEmpty` — dir exists but has no YAML files → empty result, no error

**Code (green):**
```go
func ScanDirectory(dir string) ([]string, error)
```
Read directory entries, filter for `*.yaml` suffix, sort alphabetically, return full paths.

**Refactor:** None expected.

### Step 1.3: Parse definition file (`internal/loader/path.go`) [x]

**Test (red):** `internal/loader/path_test.go`

Uses temp files created in test setup:
- `TestParseDefinitionFileNodeOnly` — file with one `kind: node` document → returns 1 `NodeDef`, 0 `RelationshipDef`
- `TestParseDefinitionFileRelOnly` — file with one `kind: relationship` document → returns 0 `NodeDef`, 1 `RelationshipDef`
- `TestParseDefinitionFileMixed` — file with node doc + relationship doc (separated by `---`) → 1 of each
- `TestParseDefinitionFileMultipleDocs` — file with 3 node docs → 3 `NodeDef`s
- `TestParseDefinitionFileSkipsUnknownKind` — document with `kind: other` → skipped, no error
- `TestParseDefinitionFileEmpty` — empty file → empty results, no error
- `TestParseDefinitionFileInvalidYAML` — malformed YAML → error
- `TestParseDefinitionFileSetsSourceField` — parsed defs have `Source` set to the file path

**Code (green):**
```go
func ParseDefinitionFile(path string) ([]NodeDef, []RelationshipDef, error)
```
Open file, use `yaml.Decoder` to read multiple documents. Peek at `kind` field to dispatch. Set `Source` field on each definition to the file path.

**Refactor:** None expected. Note: this function re-uses the multi-doc YAML parsing pattern from the iter 0 parser, but for standalone definition files rather than model files.

### Step 1.4: Load all definitions from path (`internal/loader/path.go`) [x]

**Test (red):** `internal/loader/path_test.go`

Uses temp directories with YAML files:
- `TestLoadFromPathEmpty` — empty dir list → empty results, no warnings
- `TestLoadFromPathSingleDir` — dir with 2 YAML files containing defs → all defs returned
- `TestLoadFromPathMultipleDirs` — 2 dirs → defs from both, first dir's files before second dir's
- `TestLoadFromPathMissingDir` — nonexistent dir in list → warning emitted, other dirs still processed
- `TestLoadFromPathFileOrder` — within a single directory, files processed alphabetically
- `TestLoadFromPathDuplicateAcrossFiles` — same `kind/name` in two files within same dir → first file (alphabetically) wins

**Code (green):**
```go
func LoadFromPath(dirs []string) ([]NodeDef, []RelationshipDef, []string, error)
```
Walk dirs left to right. Within each dir, call `ScanDirectory` then `ParseDefinitionFile` for each file. Collect all defs. Return defs + warnings (for missing dirs). Does not deduplicate — callers (registry or cache builder) handle first-match-wins.

**Refactor:** None expected.

---

## Phase 2 — Registry Extension [COMPLETE]

Extend the Registry from iteration 0 to track definition provenance and shadowing.

### Step 2.1: Source tracking on definition types (`internal/loader/types.go`) [x]

**Test (red):** `internal/loader/loader_test.go`
- `TestNodeDefSourceNotInYAML` — marshal a `NodeDef` with Source set → YAML output does not contain "source" field
- `TestNodeDefSourcePreservedInRegistry` — add NodeDef with Source "embedded" → `LookupNode` returns def with Source "embedded"

**Code (green):**

Add to `NodeDef`:
```go
Source string `yaml:"-" json:"source,omitempty"`
```

Add to `RelationshipDef`:
```go
Source string `yaml:"-" json:"source,omitempty"`
```

The `yaml:"-"` tag ensures the field is ignored during YAML parsing. The `json` tag allows it to be serialized in the cache.

Update `LoadEmbedded()` to set `Source: "embedded"` on all definitions it returns.

**Refactor:** None expected.

### Step 2.2: Shadow event tracking (`internal/loader/types.go`, `internal/loader/loader.go`) [x]

**Test (red):** `internal/loader/loader_test.go`
- `TestRegistryShadowOnDuplicate` — add node "server" (source "inline"), then "server" (source "embedded") → `Shadows()` returns 1 event: `{Kind: "node", Name: "server", Winner: "inline", Shadowed: "embedded"}`
- `TestRegistryNoShadowForNewDef` — add "server" then "datastore" → no shadow events
- `TestRegistryRelShadow` — duplicate relationship name → shadow event recorded
- `TestRegistryMultipleShadows` — 3 defs where 2 shadow → 2 shadow events

**Code (green):**

New type in `types.go`:
```go
type ShadowEvent struct {
    Kind     string // "node" or "relationship"
    Name     string
    Winner   string // source of the def that won
    Shadowed string // source of the def that was skipped
}
```

Extend `Registry`:
```go
type Registry struct {
    nodes   map[string]NodeDef
    rels    map[string]RelationshipDef
    shadows []ShadowEvent
}

func (r *Registry) Shadows() []ShadowEvent
```

Modify `AddNode`: when a duplicate is skipped, append a `ShadowEvent` using the `Source` fields from the existing and incoming defs.

Same for `AddRelationship`.

**Refactor:** None expected.

### Step 2.3: Extend `BuildRegistry` with `DefinitionSources` (`internal/loader/loader.go`) [x]

**Test (red):** `internal/loader/loader_test.go`
- `TestBuildRegistryWithPathDefs` — inline "custom" + path "server" + embedded "server" → path "server" wins over embedded, inline "custom" present, all embedded non-shadowed defs present
- `TestBuildRegistryPathShadowsEmbedded` — path defines "server" → embedded "server" shadowed, shadow event recorded with correct sources
- `TestBuildRegistryInlineShadowsPath` — inline "server" + path "server" → inline wins, shadow event for path's "server"
- `TestBuildRegistryInlineShadowsEmbedded` — inline "server", no path → inline wins over embedded (existing iter 0 behavior preserved)
- `TestBuildRegistrySourcesCorrect` — all defs have correct Source: "inline", file path, or "embedded"
- `TestBuildRegistryEmptyPath` — no path defs → same as iter 0 behavior (inline + embedded)

**Code (green):**

New type in `types.go`:
```go
type DefinitionSources struct {
    InlineNodes []NodeDef
    InlineRels  []RelationshipDef
    PathNodes   []NodeDef
    PathRels    []RelationshipDef
}
```

Refactor `BuildRegistry`:
```go
func BuildRegistry(sources DefinitionSources) (*Registry, error)
```

Resolution order:
1. Add inline defs (Source already set by caller to "inline")
2. Add path defs (Source already set to file path by `ParseDefinitionFile`)
3. Add embedded defs (Source set to "embedded" by `LoadEmbedded`)

**Refactor:** Update all existing callers of `BuildRegistry` (in iter 0 code) to use the new `DefinitionSources` struct with empty `PathNodes`/`PathRels`. This includes:
- `cmd/modelr/main.go` — pipeline helper
- `internal/loader/loader_test.go` — existing tests
- Any test helpers that call `BuildRegistry`

---

## Phase 3 — Definition Cache [COMPLETE]

The cache stores parsed path definitions as JSON so commands don't re-walk `$MODELR_PATH` on every invocation.

### Step 3.1: Cache data types (`internal/loader/cache.go`) [x]

**Test (red):** `internal/loader/cache_test.go`
- `TestCacheRoundTripJSON` — create `Cache` struct, marshal to JSON, unmarshal → identical struct
- `TestCacheEntryNodeDef` — cache entry with `NodeDef` → marshals/unmarshals with full property schemas preserved
- `TestCacheEntryRelDef` — cache entry with `RelationshipDef` → resolve bindings and checks preserved

**Code (green):**

```go
type Cache struct {
    Path        string       `json:"path"`
    RefreshedAt time.Time    `json:"refreshed_at"`
    Files       []CacheFile  `json:"files"`
    Entries     []CacheEntry `json:"entries"`
}

type CacheFile struct {
    Path  string    `json:"path"`
    Mtime time.Time `json:"mtime"`
    Size  int64     `json:"size"`
}

type CacheEntry struct {
    Kind     string           `json:"kind"`
    Name     string           `json:"name"`
    File     string           `json:"file"`
    DocIndex int              `json:"doc"`
    NodeDef  *NodeDef         `json:"node_def,omitempty"`
    RelDef   *RelationshipDef `json:"rel_def,omitempty"`
}
```

**Refactor:** None expected.

### Step 3.2: Write cache to disk (`internal/loader/cache.go`) [x]

**Test (red):** `internal/loader/cache_test.go`
- `TestWriteCacheCreatesFile` — write cache to temp home dir → file exists at `.modelr/cache/definitions.json`
- `TestWriteCacheCreatesDir` — `.modelr/cache/` doesn't exist → created automatically
- `TestWriteCacheContent` — write then read file bytes → valid JSON matching cache struct

**Code (green):**
```go
func CachePath(homeDir string) string  // returns $HOME/.modelr/cache/definitions.json

func WriteCache(cache *Cache, homeDir string) error
```
Creates directory structure if needed. Marshals cache to indented JSON. Writes atomically (write to temp file, then rename).

**Refactor:** None expected.

### Step 3.3: Read cache from disk (`internal/loader/cache.go`) [x]

**Test (red):** `internal/loader/cache_test.go`
- `TestReadCacheExists` — cache file exists with valid JSON → returns `*Cache`
- `TestReadCacheNotExists` — no cache file → returns `nil`, no error
- `TestReadCacheCorruptJSON` — invalid JSON content → returns error
- `TestReadCacheEmptyFile` — 0-byte file → returns error

**Code (green):**
```go
func ReadCache(homeDir string) (*Cache, error)
```
Returns `nil, nil` when file does not exist (os.IsNotExist). Returns error for corrupt files.

**Refactor:** None expected.

### Step 3.4: Staleness detection (`internal/loader/cache.go`) [x]

**Test (red):** `internal/loader/cache_test.go`

Tests use a pre-built `Cache` struct compared against filesystem state:
- `TestStalenessPathChanged` — cached path is `"/a:/b"`, current path is `"/a:/b:/c"` → stale, reason mentions `$MODELR_PATH`
- `TestStalenessFileModified` — cached file mtime is older than actual mtime → stale, reason mentions modified file
- `TestStalenessFileDeleted` — cached file no longer exists → stale, reason mentions missing file
- `TestStalenessNewFileAppeared` — new `*.yaml` file in a path dir not in cache → stale, reason mentions new file
- `TestStalenessFresh` — everything matches → not stale
- `TestStalenessEmptyCache` — cache with empty path and no files, current path also empty → fresh

**Code (green):**
```go
type StalenessResult struct {
    Stale  bool
    Reason string // human-readable, empty if fresh
}

func CheckStaleness(cache *Cache, currentPath string) (*StalenessResult, error)
```

Checks in order:
1. Compare `cache.Path` to `currentPath`
2. For each `CacheFile`: check existence, compare mtime
3. Walk current path dirs, check for new `*.yaml` files not in cache

Returns on first stale condition found.

**Refactor:** None expected.

### Step 3.5: Build cache from `$MODELR_PATH` (`internal/loader/cache.go`) [x]

**Test (red):** `internal/loader/cache_test.go`

Uses temp directories with YAML definition files:
- `TestBuildCacheEntries` — dir with 2 YAML files → cache entries match parsed definitions
- `TestBuildCacheFileMetadata` — cache records correct file path, mtime, and size for each file
- `TestBuildCachePath` — cache stores the `$MODELR_PATH` value
- `TestBuildCacheRefreshedAt` — `RefreshedAt` is set to approximately now
- `TestBuildCacheShadowsEmbedded` — path def named "server" → shadow event returned (path shadows embedded)
- `TestBuildCacheShadowsWithinPath` — dir1 and dir2 both define "server" → shadow event (dir1 shadows dir2)
- `TestBuildCacheEmptyPath` — empty `$MODELR_PATH` → cache with empty entries and files

**Code (green):**
```go
func BuildCache(modelrPath string) (*Cache, []ShadowEvent, error)
```

1. Parse path → dirs
2. Call `LoadFromPath(dirs)` to get all defs
3. Collect file metadata (stat each file for mtime/size)
4. Build cache entries with full definition data
5. Detect shadows: compare path defs against each other (by load order) and against embedded defaults
6. Return cache + shadow events

**Refactor:** Extract shadow detection into a helper that compares a list of defs against a set of known names. This is reusable by `BuildRegistry`.

---

## Phase 4 — CLI Cache Commands [COMPLETE]

### Step 4.1: `cache refresh` command (`cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestCacheRefreshCreatesFile` — set `$MODELR_PATH` to temp dir with YAML files, run `modelr cache refresh` → cache file created at expected location
- `TestCacheRefreshRebuildDeletesExisting` — existing cache file, run with `--rebuild` → cache recreated (refreshed_at updated)
- `TestCacheRefreshAutoWhenStale` — stale cache + `--auto` → cache refreshed
- `TestCacheRefreshAutoWhenFresh` — fresh cache + `--auto` → no-op, output indicates cache is current
- `TestCacheRefreshMutuallyExclusiveFlags` — `--rebuild --auto` → error
- `TestCacheRefreshEmptyPath` — `$MODELR_PATH` empty → cache created with no entries
- `TestCacheRefreshPrintsShadowing` — path shadows embedded → info messages printed to stderr

**Code (green):**

Register `cache` command group with `refresh` subcommand:
```go
&cli.Command{
    Name: "cache",
    Commands: []*cli.Command{
        {
            Name: "refresh",
            Flags: []cli.Flag{
                &cli.BoolFlag{Name: "rebuild"},
                &cli.BoolFlag{Name: "auto"},
            },
            Action: cacheRefreshAction,
        },
    },
}
```

`cacheRefreshAction`:
1. Check mutually exclusive flags
2. If `--auto`: read existing cache, check staleness, skip if fresh
3. Call `BuildCache(os.Getenv("MODELR_PATH"))`
4. Print shadow events as `info:` messages to stderr
5. Call `WriteCache`

**Refactor:** None expected.

### Step 4.2: `cache status` command (`cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestCacheStatusNoCache` — no cache file → prints "No cache found. Run 'modelr cache refresh' to build."
- `TestCacheStatusFresh` — fresh cache → prints last refreshed time, relative age, file size, "Status: current"
- `TestCacheStatusStale` — stale cache → prints metadata + "Status: stale" + reason
- `TestCacheStatusEntryCount` — cache with 5 entries → output includes "5 definitions cached"

**Code (green):**

Add `status` subcommand under `cache`:
```go
{
    Name:   "status",
    Action: cacheStatusAction,
}
```

`cacheStatusAction`:
1. Read cache
2. If nil: print "no cache found" message
3. Check staleness
4. Format output:
```
Last refreshed: 2026-03-16T14:32:00Z (2 hours ago)
Definitions:    5
Size on disk:   4.2 KB
Status:         current
```

**Refactor:** None expected.

### Step 4.3: Auto-cache and stale warning in pipeline (`internal/loader/loader.go` or `cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestPipelineAutoCacheOnFirstRun` — `$MODELR_PATH` set, no cache exists → auto-builds cache before loading, path defs available in registry
- `TestPipelineAutoCacheShadowToStderr` — auto-cache triggers shadow detection → info messages to stderr
- `TestPipelineStaleWarning` — stale cache exists → warning to stderr: `"warning: definition cache is stale (run 'modelr cache refresh' to update)"`, cached defs still used
- `TestPipelineNoCacheNoPath` — `$MODELR_PATH` empty, no cache → no auto-cache, no warning, registry has only inline+embedded
- `TestPipelineFreshCacheUsed` — fresh cache → defs loaded from cache, no warning

**Code (green):**

Create a pipeline helper:
```go
func LoadPathDefinitions(modelrPath string, homeDir string, stderr io.Writer) ([]NodeDef, []RelationshipDef, error)
```

Logic:
1. If `modelrPath` is empty: return empty (no path defs to load)
2. Read cache
3. If cache is nil (doesn't exist):
   - Auto-build: `BuildCache(modelrPath)`
   - Print shadow events to stderr
   - Write cache
   - Return defs from cache entries
4. If cache exists:
   - Check staleness
   - If stale: print warning to stderr, but still use cached defs
   - Return defs from cache entries

Update the shared pipeline in `cmd/modelr/main.go` to call `LoadPathDefinitions` and pass results into `BuildRegistry` via `DefinitionSources.PathNodes` and `DefinitionSources.PathRels`.

**Refactor:** Extract the existing pipeline logic (parse → build registry → validate) into a named function if not already done, so it can be shared cleanly between `check`, `validate`, and future commands.

---

## Phase 5 — Definitions Command [COMPLETE]

### Step 5.1: `definitions` command (`cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestDefinitionsCommandEmbeddedOnly` — `$MODELR_PATH` empty → lists 3 node defs + 2 relationship defs from embedded, each with name, description, and "(embedded)" source
- `TestDefinitionsCommandWithPath` — `$MODELR_PATH` set with custom defs → path defs shown with file path as source
- `TestDefinitionsCommandShadowing` — path redefines "server" → only path version shown, source shows file path
- `TestDefinitionsCommandVerbose` — `--verbose` → each definition shows its property list (name, type, unit, default)
- `TestDefinitionsCommandOutputFormat` — verify output structure matches expected format

**Code (green):**

Register `definitions` command:
```go
&cli.Command{
    Name:        "definitions",
    Description: "List available node and relationship definitions",
    Flags: []cli.Flag{
        &cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}},
    },
    Action: definitionsAction,
}
```

`definitionsAction`:
1. Load path definitions (via `LoadPathDefinitions`, handles cache)
2. Build registry with `DefinitionSources{PathNodes: ..., PathRels: ...}` (no inline since there's no model file)
3. Print node definitions:
```
Node Definitions:
  server           Maximum concurrent connections, scaling       (embedded)
  datastore        Database with connection limits               (embedded)
  queue            Message queue with backpressure               (embedded)
  postgres         PostgreSQL database                           (/opt/defs/postgres.yaml)
```
4. Print relationship definitions:
```
Relationship Definitions:
  capacity_chain           Upstream concurrency vs downstream     (embedded)
  pooled_capacity_chain    Pooled connection check                (embedded)
```
5. With `--verbose`, add property details under each node definition:
```
Node Definitions:
  server           Maximum concurrent connections, scaling       (embedded)
    max_connections    number   connections
    min_instances      number   instances      default: 1
    max_instances      number   instances      default: 1
```

**Refactor:** Extract a `ListDefinitions` helper in `internal/loader/` that returns a structured representation (rather than building strings in the CLI layer). This allows the MCP `list_definitions` tool to reuse the same logic in a future iteration.

---

## Phase 6 — Integration [COMPLETE]

### Step 6.1: Update `check` and `validate` commands (`cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestCheckWithPathDefinitions` — model uses type `postgres` (defined in `$MODELR_PATH`), not in embedded → type validated, defaults from path def applied
- `TestValidateWithPathDefinitions` — same model with `validate` → assumptions reference path definition as source
- `TestVerboseShowsDefinitionSources` — `--verbose` flag on `check` → output shows loading sources matching spec format:
  ```
  Loading definitions:
    node/server                       ← embedded
    node/postgres                     ← /tmp/test/postgres.yaml
    relationship/capacity_chain       ← embedded
  ```
- `TestCheckInlineShadowsPathShadowsEmbedded` — model with inline "server" + path "server" + embedded "server" → inline wins, shadow info shown

**Code (green):**

Modify the existing pipeline helper to:
1. Call `LoadPathDefinitions` with `$MODELR_PATH` and `$HOME`
2. Pass path defs into `BuildRegistry` via `DefinitionSources`
3. When `--verbose`: iterate `Registry` sources and print loading table

Verbose output format (per spec 7.4):
```go
if verbose {
    fmt.Fprintln(w, "Loading definitions:")
    for _, src := range registry.Sources() {
        fmt.Fprintf(w, "  %-36s ← %s\n", src.Kind+"/"+src.Name, src.Origin)
    }
}
```

Add a `Sources()` method to `Registry` that returns all tracked definitions with their sources, ordered by kind then name.

**Refactor:** None expected.

### Step 6.2: End-to-end integration tests (`cmd/modelr/main_test.go`) [x]

**Test (red):**
- `TestEndToEndPathShadowsEmbedded` — `$MODELR_PATH` dir redefines "datastore" with different defaults → model check uses path's defaults, assumptions reference path file
- `TestEndToEndInlineShadowsPath` — model file has inline "server" def + path also has "server" → inline wins, `.checked.yaml` reflects inline definition's schema
- `TestEndToEndCacheRefreshThenCheck` — `cache refresh` → `check` on model using path types → path defs used
- `TestEndToEndCacheStaleThenCheck` — modify a path YAML file after cache build → `check` emits stale warning, still uses cached (pre-modification) defs
- `TestEndToEndDefinitionsWithPath` — `$MODELR_PATH` set → `definitions` command lists path + embedded defs
- `TestEndToEndDefinitionsEmbeddedOnly` — no `$MODELR_PATH` → `definitions` lists only embedded

**Code (green):**
These should pass with existing code from Phases 1–5. Create test fixture directories in `testdata/`:
```
testdata/
  path_defs/
    dir1/
      postgres.yaml     # node/postgres + relationship/pooled_capacity_chain
    dir2/
      custom.yaml       # node/custom_server
    shadow/
      server.yaml       # node/server that shadows embedded
  models/
    uses_postgres.model.yaml
    uses_custom.model.yaml
```

**Refactor:** Create a test helper that sets up `$MODELR_PATH`, `$HOME` (for cache), and cleans up after. This avoids repetition across integration tests.

---

## Self-Critique and Revisions

### Issues found and addressed

1. **`BuildRegistry` signature change breaks iter 0 callers** — Changing from `BuildRegistry(inline, inlineRels)` to `BuildRegistry(sources DefinitionSources)` breaks all existing call sites. **Resolution:** Step 2.3 explicitly includes refactoring existing callers. Since all callers are in `internal/` and `cmd/`, this is a contained change. The old tests just wrap their args in `DefinitionSources{InlineNodes: ..., InlineRels: ...}` with empty path fields.

2. **Registry needs a `Sources()` method for verbose output** — The verbose loading table (spec 7.4) needs to know all definitions and their sources. **Resolution:** Added `Sources()` method to Registry in Step 6.1. This returns a slice of `DefinitionSource` structs (kind, name, origin) ordered for display.

3. **Shadow detection happens in two places** — `BuildCache` detects shadows for the info messages, and `BuildRegistry` detects shadows via `AddNode`/`AddRelationship`. **Resolution:** Both are needed. `BuildCache` detects path-vs-embedded shadows (for `cache refresh` output). `BuildRegistry` detects inline-vs-path and inline-vs-embedded shadows (for runtime messages). The Registry's `Shadows()` method captures all shadow events that occur during registry building.

4. **Cache stores full definitions, not just an index** — The spec example JSON only shows kind/name/file/doc. But to avoid re-parsing, we store full `NodeDef`/`RelDef` in each cache entry. **Resolution:** `CacheEntry` has optional `NodeDef`/`RelDef` fields. The file/doc fields serve as metadata for debugging and display, while the full definition avoids re-parsing YAML.

5. **`definitions` command has no model file** — It can't resolve inline definitions since there's no model. **Resolution:** The command loads from cache/path + embedded only. Inline definitions are inherently model-specific and aren't relevant to the `definitions` command's purpose of listing available building blocks.

6. **Walk order within a directory matters** — If two files in the same dir define the same kind/name, which wins? **Resolution:** Files within a directory are processed alphabetically (Step 1.2 sorts filenames). Within a file, documents appear in order. First encountered definition wins.

7. **The `cache refresh` output format for shadow events** — Spec says `info:` prefix. **Resolution:** Step 4.1 prints to stderr: `info: node/server shadowed (embedded → /path/to/server.yaml)`. The arrow shows the resolution direction (what was replaced → what replaced it).

8. **`LoadPathDefinitions` returns defs from cache entries, not raw parsing** — When cache is used, need to extract `NodeDef`/`RelDef` from `CacheEntry` structs. **Resolution:** `LoadPathDefinitions` handles this extraction internally. Callers receive `[]NodeDef` and `[]RelationshipDef` regardless of whether they came from cache or fresh parsing.

9. **Missing: `Registry.Sources()` ordering** — For verbose output, definitions should be displayed in a consistent order. **Resolution:** Sort by kind (node first, relationship second), then alphabetically by name within each kind.

10. **Edge case: `$HOME` not set** — Cache operations need `$HOME`. **Resolution:** `ReadCache` and `WriteCache` take `homeDir` as a parameter rather than reading the env directly. The CLI layer reads `$HOME` and passes it in. Tests use temp directories.

### Ordering validation

- Phase 1 (path scanning) has no dependencies on new code — correct start
- Phase 2 (registry extension) uses types from Phase 1 (`Source` field, `ShadowEvent`) — correct
- Phase 3 (cache) depends on Phase 1 (path scanning to build cache entries) and Phase 2 (shadow detection) — correct
- Phase 4 (CLI cache commands) depends on Phase 3 (cache I/O) — correct
- Phase 5 (definitions) depends on Phase 2 (registry with sources) and Phase 4 (auto-cache behavior) — correct
- Phase 6 (integration) depends on all — correct final phase

### What this plan does NOT change

- The expression evaluator (`internal/check/expr.go`) — untouched
- The constraint checker (`internal/check/checker.go`) — untouched (it receives a `Registry`, doesn't care where defs came from)
- The validator (`internal/model/validator.go`) — untouched (same: operates on Registry)
- The parser (`internal/model/parser.go`) — untouched
- Embedded definitions (`embed/`) — untouched

---

## Estimated Step Count

| Phase | Steps | Tests |
|-------|-------|-------|
| 1. Path Scanning | 4 | ~20 |
| 2. Registry Extension | 3 | ~12 |
| 3. Cache | 5 | ~17 |
| 4. CLI Cache Commands | 3 | ~13 |
| 5. Definitions Command | 1 | ~5 |
| 6. Integration | 2 | ~10 |
| **Total** | **18** | **~77** |
