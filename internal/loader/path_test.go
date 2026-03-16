package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Step 1.1: ParseModelrPath ---

func TestParseModelrPathEmpty(t *testing.T) {
	result := ParseModelrPath("")
	assert.Empty(t, result)
}

func TestParseModelrPathUnset(t *testing.T) {
	result := ParseModelrPath("")
	assert.Empty(t, result)
}

func TestParseModelrPathSingle(t *testing.T) {
	result := ParseModelrPath("/opt/defs")
	assert.Equal(t, []string{"/opt/defs"}, result)
}

func TestParseModelrPathMultiple(t *testing.T) {
	result := ParseModelrPath("/opt/defs:/home/user/defs:/proj/defs")
	assert.Equal(t, []string{"/opt/defs", "/home/user/defs", "/proj/defs"}, result)
}

func TestParseModelrPathSkipsEmptySegments(t *testing.T) {
	result := ParseModelrPath("/opt/defs::/home/user/defs")
	assert.Equal(t, []string{"/opt/defs", "/home/user/defs"}, result)
}

// --- Step 1.2: ScanDirectory ---

func TestScanDirectoryFindsYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("---\nkind: node\nname: a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.yaml"), []byte("---\nkind: node\nname: b\n"), 0644)

	paths, err := ScanDirectory(dir)
	require.NoError(t, err)
	assert.Len(t, paths, 2)
	assert.Equal(t, filepath.Join(dir, "a.yaml"), paths[0])
	assert.Equal(t, filepath.Join(dir, "b.yaml"), paths[1])
}

func TestScanDirectoryIgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "config.yml"), []byte("x: 1"), 0644)

	paths, err := ScanDirectory(dir)
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestScanDirectoryDoesNotRecurse(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0755)
	os.WriteFile(filepath.Join(sub, "nested.yaml"), []byte("---\nkind: node\nname: nested\n"), 0644)

	paths, err := ScanDirectory(dir)
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestScanDirectoryNotExist(t *testing.T) {
	_, err := ScanDirectory("/nonexistent/dir/that/does/not/exist")
	require.Error(t, err)
}

func TestScanDirectoryEmpty(t *testing.T) {
	dir := t.TempDir()

	paths, err := ScanDirectory(dir)
	require.NoError(t, err)
	assert.Empty(t, paths)
}

// --- Step 1.3: ParseDefinitionFile ---

func writeDefFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestParseDefinitionFileNodeOnly(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "server.yaml", `---
kind: node
name: custom_server
description: A custom server
properties:
  max_rps:
    type: number
    unit: req/s
    description: Max requests per second
`)
	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Empty(t, rels)
	assert.Equal(t, "custom_server", nodes[0].Name)
}

func TestParseDefinitionFileRelOnly(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "check.yaml", `---
kind: relationship
name: custom_check
description: A custom check
resolve:
  rate: upstream.max_rps
checks:
  - name: rate_check
    expression: "rate <= 1000"
    violation: "Rate too high"
`)
	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Len(t, rels, 1)
	assert.Equal(t, "custom_check", rels[0].Name)
}

func TestParseDefinitionFileMixed(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "postgres.yaml", `---
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
    violation: "Too few connections"
`)
	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Len(t, rels, 1)
}

func TestParseDefinitionFileMultipleDocs(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "types.yaml", `---
kind: node
name: type_a
description: A
properties: {}
---
kind: node
name: type_b
description: B
properties: {}
---
kind: node
name: type_c
description: C
properties: {}
`)
	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	assert.Len(t, nodes, 3)
	assert.Empty(t, rels)
}

func TestParseDefinitionFileSkipsUnknownKind(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "misc.yaml", `---
kind: other
name: something
description: Unknown kind
`)
	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, rels)
}

func TestParseDefinitionFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "empty.yaml", "")

	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, rels)
}

func TestParseDefinitionFileInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "bad.yaml", "{{not yaml at all")

	_, _, err := ParseDefinitionFile(path)
	require.Error(t, err)
}

func TestParseDefinitionFileSetsSourceField(t *testing.T) {
	dir := t.TempDir()
	path := writeDefFile(t, dir, "defs.yaml", `---
kind: node
name: mynode
description: Test node
properties: {}
---
kind: relationship
name: myrel
description: Test rel
resolve: {}
checks: []
`)
	nodes, rels, err := ParseDefinitionFile(path)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Len(t, rels, 1)
	assert.Equal(t, path, nodes[0].Source)
	assert.Equal(t, path, rels[0].Source)
}

// --- Step 1.4: LoadFromPath ---

func TestLoadFromPathEmpty(t *testing.T) {
	nodes, rels, warnings, err := LoadFromPath(nil)
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, rels)
	assert.Empty(t, warnings)
}

func TestLoadFromPathSingleDir(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "a.yaml", `---
kind: node
name: node_a
description: A
properties: {}
`)
	writeDefFile(t, dir, "b.yaml", `---
kind: relationship
name: rel_b
description: B
resolve: {}
checks: []
`)
	nodes, rels, warnings, err := LoadFromPath([]string{dir})
	require.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Len(t, rels, 1)
	assert.Empty(t, warnings)
}

func TestLoadFromPathMultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writeDefFile(t, dir1, "a.yaml", `---
kind: node
name: from_dir1
description: Dir1
properties: {}
`)
	writeDefFile(t, dir2, "b.yaml", `---
kind: node
name: from_dir2
description: Dir2
properties: {}
`)
	nodes, _, _, err := LoadFromPath([]string{dir1, dir2})
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	// dir1 entries come first
	assert.Equal(t, "from_dir1", nodes[0].Name)
	assert.Equal(t, "from_dir2", nodes[1].Name)
}

func TestLoadFromPathMissingDir(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "a.yaml", `---
kind: node
name: node_a
description: A
properties: {}
`)
	nodes, _, warnings, err := LoadFromPath([]string{"/nonexistent/path", dir})
	require.NoError(t, err)
	assert.Len(t, nodes, 1) // still got defs from the valid dir
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "/nonexistent/path")
}

func TestLoadFromPathFileOrder(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "z.yaml", `---
kind: node
name: z_node
description: Z
properties: {}
`)
	writeDefFile(t, dir, "a.yaml", `---
kind: node
name: a_node
description: A
properties: {}
`)
	nodes, _, _, err := LoadFromPath([]string{dir})
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	// a.yaml processed before z.yaml (alphabetical)
	assert.Equal(t, "a_node", nodes[0].Name)
	assert.Equal(t, "z_node", nodes[1].Name)
}

func TestLoadFromPathDuplicateAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeDefFile(t, dir, "a.yaml", `---
kind: node
name: server
description: From a.yaml
properties: {}
`)
	writeDefFile(t, dir, "b.yaml", `---
kind: node
name: server
description: From b.yaml
properties: {}
`)
	nodes, _, _, err := LoadFromPath([]string{dir})
	require.NoError(t, err)
	// Both returned — deduplication is done by the registry, not LoadFromPath
	assert.Len(t, nodes, 2)
	assert.Equal(t, "server", nodes[0].Name)
	assert.Equal(t, "server", nodes[1].Name)
}
