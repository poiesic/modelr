package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Cache data types ---

func TestCacheRoundTripJSON(t *testing.T) {
	original := &Cache{
		Path:        "/opt/defs:/home/user/defs",
		RefreshedAt: time.Date(2026, 3, 16, 14, 32, 0, 0, time.UTC),
		Files: []CacheFile{
			{Path: "/opt/defs/pg.yaml", Mtime: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), Size: 1024},
		},
		Entries: []CacheEntry{
			{Kind: "node", Name: "postgres", File: "/opt/defs/pg.yaml", DocIndex: 0},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Cache
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.Path, restored.Path)
	assert.Equal(t, original.RefreshedAt, restored.RefreshedAt)
	assert.Len(t, restored.Files, 1)
	assert.Len(t, restored.Entries, 1)
	assert.Equal(t, "node", restored.Entries[0].Kind)
	assert.Equal(t, "postgres", restored.Entries[0].Name)
	assert.Equal(t, "/opt/defs/pg.yaml", restored.Entries[0].File)
	assert.Equal(t, 0, restored.Entries[0].DocIndex)
}

func TestCacheEntryIsIndexOnly(t *testing.T) {
	entry := CacheEntry{Kind: "node", Name: "pg", File: "/defs/pg.yaml", DocIndex: 0}
	data, err := json.Marshal(entry)
	require.NoError(t, err)

	// Should NOT contain "node_def", "rel_def", "properties", etc.
	assert.NotContains(t, string(data), "node_def")
	assert.NotContains(t, string(data), "rel_def")
	assert.NotContains(t, string(data), "properties")
}

// --- Write cache ---

func TestWriteCacheCreatesFile(t *testing.T) {
	home := t.TempDir()
	cache := &Cache{Path: "", RefreshedAt: time.Now().UTC()}

	err := WriteCache(cache, home)
	require.NoError(t, err)

	_, err = os.Stat(CachePath(home))
	assert.NoError(t, err)
}

func TestWriteCacheCreatesDir(t *testing.T) {
	home := t.TempDir()
	cache := &Cache{Path: "", RefreshedAt: time.Now().UTC()}

	err := WriteCache(cache, home)
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(home, ".modelr", "cache"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestWriteCacheContent(t *testing.T) {
	home := t.TempDir()
	cache := &Cache{
		Path:        "/opt/defs",
		RefreshedAt: time.Date(2026, 3, 16, 14, 0, 0, 0, time.UTC),
		Entries: []CacheEntry{
			{Kind: "node", Name: "pg", File: "/opt/defs/pg.yaml", DocIndex: 0},
		},
	}

	err := WriteCache(cache, home)
	require.NoError(t, err)

	data, err := os.ReadFile(CachePath(home))
	require.NoError(t, err)

	var restored Cache
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)
	assert.Equal(t, "/opt/defs", restored.Path)
	assert.Len(t, restored.Entries, 1)
}

// --- Read cache ---

func TestReadCacheExists(t *testing.T) {
	home := t.TempDir()
	original := &Cache{Path: "/opt/defs", RefreshedAt: time.Now().UTC()}
	require.NoError(t, WriteCache(original, home))

	cache, err := ReadCache(home)
	require.NoError(t, err)
	require.NotNil(t, cache)
	assert.Equal(t, "/opt/defs", cache.Path)
}

func TestReadCacheNotExists(t *testing.T) {
	home := t.TempDir()

	cache, err := ReadCache(home)
	require.NoError(t, err)
	assert.Nil(t, cache)
}

func TestReadCacheCorruptJSON(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, ".modelr", "cache")
	os.MkdirAll(cacheDir, 0755)
	os.WriteFile(filepath.Join(cacheDir, "definitions.json"), []byte("{not json"), 0644)

	_, err := ReadCache(home)
	require.Error(t, err)
}

func TestReadCacheEmptyFile(t *testing.T) {
	home := t.TempDir()
	cacheDir := filepath.Join(home, ".modelr", "cache")
	os.MkdirAll(cacheDir, 0755)
	os.WriteFile(filepath.Join(cacheDir, "definitions.json"), []byte(""), 0644)

	_, err := ReadCache(home)
	require.Error(t, err)
}

// --- Staleness detection ---

func TestStalenessPathChanged(t *testing.T) {
	cache := &Cache{Path: "/a:/b"}

	result, err := CheckStaleness(cache, "/a:/b:/c")
	require.NoError(t, err)
	assert.True(t, result.Stale)
	assert.Contains(t, result.Reason, "$MODELR_PATH")
}

func TestStalenessFileModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte("test"), 0644)

	cache := &Cache{
		Path: dir,
		Files: []CacheFile{
			{Path: path, Mtime: time.Now().Add(-1 * time.Hour), Size: 4},
		},
	}

	result, err := CheckStaleness(cache, dir)
	require.NoError(t, err)
	assert.True(t, result.Stale)
	assert.Contains(t, result.Reason, "modified")
}

func TestStalenessFileDeleted(t *testing.T) {
	cache := &Cache{
		Path: "/some/dir",
		Files: []CacheFile{
			{Path: "/some/dir/deleted.yaml", Mtime: time.Now(), Size: 100},
		},
	}

	result, err := CheckStaleness(cache, "/some/dir")
	require.NoError(t, err)
	assert.True(t, result.Stale)
	assert.Contains(t, result.Reason, "no longer exists")
}

func TestStalenessNewFileAppeared(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.yaml")
	newPath := filepath.Join(dir, "new.yaml")
	os.WriteFile(existingPath, []byte("test"), 0644)
	os.WriteFile(newPath, []byte("test"), 0644)

	info, _ := os.Stat(existingPath)

	cache := &Cache{
		Path: dir,
		Files: []CacheFile{
			{Path: existingPath, Mtime: info.ModTime(), Size: info.Size()},
		},
	}

	result, err := CheckStaleness(cache, dir)
	require.NoError(t, err)
	assert.True(t, result.Stale)
	assert.Contains(t, result.Reason, "new file")
}

func TestStalenessFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte("test"), 0644)
	info, _ := os.Stat(path)

	cache := &Cache{
		Path: dir,
		Files: []CacheFile{
			{Path: path, Mtime: info.ModTime(), Size: info.Size()},
		},
	}

	result, err := CheckStaleness(cache, dir)
	require.NoError(t, err)
	assert.False(t, result.Stale)
}

func TestStalenessEmptyCache(t *testing.T) {
	cache := &Cache{Path: ""}

	result, err := CheckStaleness(cache, "")
	require.NoError(t, err)
	assert.False(t, result.Stale)
}

// --- Build cache ---

func TestBuildCacheEntries(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL
properties:
  max_connections:
    type: number
    unit: connections
    description: Max connections
---
kind: relationship
name: pg_check
description: PG check
resolve:
  cap: downstream.max_connections
checks:
  - name: cap_check
    expression: "cap >= 10"
    violation: "Too few"
`)

	cache, _, err := BuildCache(dir)
	require.NoError(t, err)
	assert.Len(t, cache.Entries, 2)

	// Entries are index-only
	for _, e := range cache.Entries {
		assert.NotEmpty(t, e.Kind)
		assert.NotEmpty(t, e.Name)
		assert.NotEmpty(t, e.File)
	}
}

func TestBuildCacheFileMetadata(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "test.yaml", `---
kind: node
name: test_node
description: Test
properties: {}
`)
	info, _ := os.Stat(filepath.Join(dir, "test.yaml"))

	cache, _, err := BuildCache(dir)
	require.NoError(t, err)
	require.Len(t, cache.Files, 1)
	assert.Equal(t, filepath.Join(dir, "test.yaml"), cache.Files[0].Path)
	assert.Equal(t, info.ModTime().UTC(), cache.Files[0].Mtime.UTC())
	assert.Equal(t, info.Size(), cache.Files[0].Size)
}

func TestBuildCachePath(t *testing.T) {
	dir := t.TempDir()

	cache, _, err := BuildCache(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, cache.Path)
}

func TestBuildCacheRefreshedAt(t *testing.T) {
	before := time.Now().UTC().Add(-1 * time.Second)

	cache, _, err := BuildCache("")
	require.NoError(t, err)
	assert.True(t, cache.RefreshedAt.After(before))
}

func TestBuildCacheShadowsEmbedded(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "server.yaml", `---
kind: node
name: server
description: Custom server
properties: {}
`)
	_, shadows, err := BuildCache(dir)
	require.NoError(t, err)

	var found bool
	for _, s := range shadows {
		if s.Name == "server" && s.Shadowed == "embedded" {
			found = true
		}
	}
	assert.True(t, found, "expected shadow event for path→embedded")
}

func TestBuildCacheShadowsWithinPath(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writeDefFile(t, dir1, "server.yaml", `---
kind: node
name: server
description: Dir1 server
properties: {}
`)
	writeDefFile(t, dir2, "server.yaml", `---
kind: node
name: server
description: Dir2 server
properties: {}
`)
	_, shadows, err := BuildCache(dir1 + ":" + dir2)
	require.NoError(t, err)

	var foundPathShadow bool
	for _, s := range shadows {
		if s.Name == "server" && s.Kind == "node" && s.Shadowed != "embedded" {
			foundPathShadow = true
		}
	}
	assert.True(t, foundPathShadow, "expected shadow within path dirs")
}

func TestBuildCacheEmptyPath(t *testing.T) {
	cache, _, err := BuildCache("")
	require.NoError(t, err)
	assert.Empty(t, cache.Entries)
	assert.Empty(t, cache.Files)
}

// --- DefinitionLoader: lazy + memoized ---

func TestDefinitionLoaderLoadNode(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL
properties:
  max_connections:
    type: number
    unit: connections
    description: Max connections
    default: 100
`)
	dl := NewDefinitionLoader()

	entry := CacheEntry{Kind: "node", Name: "postgres", File: path, DocIndex: 0}
	nd, rd, err := dl.LoadEntry(entry)
	require.NoError(t, err)
	require.NotNil(t, nd)
	assert.Nil(t, rd)
	assert.Equal(t, "postgres", nd.Name)
	assert.Contains(t, nd.Properties, "max_connections")
}

func TestDefinitionLoaderLoadRelationship(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "check.yaml", `---
kind: relationship
name: my_check
description: Custom check
resolve:
  rate: upstream.max_rps
checks:
  - name: rate_check
    expression: "rate <= 1000"
    violation: "Too high"
`)
	dl := NewDefinitionLoader()

	entry := CacheEntry{Kind: "relationship", Name: "my_check", File: path, DocIndex: 0}
	nd, rd, err := dl.LoadEntry(entry)
	require.NoError(t, err)
	assert.Nil(t, nd)
	require.NotNil(t, rd)
	assert.Equal(t, "my_check", rd.Name)
	assert.Len(t, rd.Checks, 1)
}

func TestDefinitionLoaderMemoizesFileParsing(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "defs.yaml", `---
kind: node
name: node_a
description: Node A
properties: {}
---
kind: relationship
name: rel_b
description: Rel B
resolve: {}
checks: []
`)
	dl := NewDefinitionLoader()

	// Load two entries from the same file
	entry1 := CacheEntry{Kind: "node", Name: "node_a", File: path, DocIndex: 0}
	entry2 := CacheEntry{Kind: "relationship", Name: "rel_b", File: path, DocIndex: 1}

	nd, _, err := dl.LoadEntry(entry1)
	require.NoError(t, err)
	require.NotNil(t, nd)

	_, rd, err := dl.LoadEntry(entry2)
	require.NoError(t, err)
	require.NotNil(t, rd)

	// Verify memoization: only one file in the internal map
	dl.mu.Lock()
	assert.Len(t, dl.files, 1, "file should be parsed only once")
	dl.mu.Unlock()
}

func TestDefinitionLoaderMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	path1 := writeDefFile(t, dir, "a.yaml", `---
kind: node
name: node_a
description: A
properties: {}
`)
	path2 := writeDefFile(t, dir, "b.yaml", `---
kind: node
name: node_b
description: B
properties: {}
`)
	dl := NewDefinitionLoader()

	nd1, _, err := dl.LoadEntry(CacheEntry{Kind: "node", Name: "node_a", File: path1, DocIndex: 0})
	require.NoError(t, err)
	assert.Equal(t, "node_a", nd1.Name)

	nd2, _, err := dl.LoadEntry(CacheEntry{Kind: "node", Name: "node_b", File: path2, DocIndex: 0})
	require.NoError(t, err)
	assert.Equal(t, "node_b", nd2.Name)

	dl.mu.Lock()
	assert.Len(t, dl.files, 2, "each file parsed once")
	dl.mu.Unlock()
}

func TestDefinitionLoaderMissingFile(t *testing.T) {
	dl := NewDefinitionLoader()
	entry := CacheEntry{Kind: "node", Name: "missing", File: "/nonexistent.yaml", DocIndex: 0}

	_, _, err := dl.LoadEntry(entry)
	require.Error(t, err)
}

// --- DefsFromCache: integration ---

func TestDefsFromCacheLoadsDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL
properties:
  max_connections:
    type: number
    unit: connections
    description: Max connections
---
kind: relationship
name: pg_check
description: PG check
resolve:
  cap: downstream.max_connections
checks:
  - name: cap_check
    expression: "cap >= 10"
    violation: "Too few"
`)

	cache, _, err := BuildCache(dir)
	require.NoError(t, err)

	nodes, rels, err := DefsFromCache(cache)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Len(t, rels, 1)
	assert.Equal(t, "postgres", nodes[0].Name)
	assert.Contains(t, nodes[0].Properties, "max_connections")
	assert.Equal(t, "pg_check", rels[0].Name)
	assert.Len(t, rels[0].Checks, 1)
}

func TestDefsFromCacheEmptyCache(t *testing.T) {
	cache := &Cache{Entries: nil}

	nodes, rels, err := DefsFromCache(cache)
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, rels)
}
