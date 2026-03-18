package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
)

func runApp(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	return runAppWithEnv(t, envConfig{}, args...)
}

func runAppWithEnv(t *testing.T, env envConfig, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	app := buildAppWithEnv(&outBuf, &errBuf, env)
	fullArgs := append([]string{"modelr"}, args...)
	runErr := app.Run(context.Background(), fullArgs)
	return outBuf.String(), errBuf.String(), runErr
}

// Helper: write a YAML definition file in a temp directory
func writePathDef(t *testing.T, dir, filename, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644))
}

// --- CLI skeleton tests ---

func TestCLINoArgs(t *testing.T) {
	stdout, _, err := runApp(t)
	require.NoError(t, err)
	assert.Contains(t, stdout, "modelr")
}

func TestCLIUnknownCommand(t *testing.T) {
	t.Skip("urfave/cli v3 calls os.Exit for unknown commands")
}

// --- validate command ---

func TestValidateCommandSuccess(t *testing.T) {
	stdout, _, err := runApp(t, "validate", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "error")
}

func TestValidateCommandFileNotFound(t *testing.T) {
	_, _, err := runApp(t, "validate", "testdata/nonexistent.model.yaml")
	require.Error(t, err)
}

func TestValidateCommandInvalidModel(t *testing.T) {
	_, _, err := runApp(t, "validate", "testdata/invalid.model.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "components")
}

func TestValidateCommandVerbose(t *testing.T) {
	stdout, _, err := runApp(t, "validate", "--verbose", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Loading definitions")
	assert.Contains(t, stdout, "embedded")
}

// --- check command ---

func TestCheckCommandSuccess(t *testing.T) {
	outputPath := "testdata/clean.checked.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "check", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "All relationship constraints satisfied")
	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

func TestCheckCommandFindings(t *testing.T) {
	outputPath := "testdata/violation.checked.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "check", "testdata/violation.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "violation")
	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

func TestCheckCommandVerbose(t *testing.T) {
	outputPath := "testdata/clean.checked.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "check", "--verbose", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Loading definitions")
}

// --- .checked.yaml output ---

func TestCheckedYAMLStructure(t *testing.T) {
	outputPath := "testdata/clean.checked.yaml"
	defer os.Remove(outputPath)

	_, _, err := runApp(t, "check", "testdata/clean.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.Equal(t, "clean-system", output.Model)
	assert.NotEmpty(t, output.CheckedAt)
	assert.NotNil(t, output.KnownUnknowns)
	assert.NotNil(t, output.Findings)
	assert.NotNil(t, output.Assumptions)
	assert.NotNil(t, output.Warnings)
	assert.NotEmpty(t, output.Summary)
}

func TestCheckedYAMLTimestamp(t *testing.T) {
	outputPath := "testdata/clean.checked.yaml"
	defer os.Remove(outputPath)

	_, _, err := runApp(t, "check", "testdata/clean.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.Contains(t, output.CheckedAt, "T")
	assert.Contains(t, output.CheckedAt, "Z")
}

func TestCheckedYAMLFindings(t *testing.T) {
	outputPath := "testdata/violation.checked.yaml"
	defer os.Remove(outputPath)

	_, _, err := runApp(t, "check", "testdata/violation.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.NotEmpty(t, output.Findings)
	assert.Equal(t, "error", output.Findings[0].Severity)
}

func TestCheckedYAMLAssumptions(t *testing.T) {
	outputPath := "testdata/null-prop.checked.yaml"
	defer os.Remove(outputPath)

	_, _, err := runApp(t, "check", "testdata/null-prop.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.NotEmpty(t, output.Assumptions)
}

// --- End-to-end integration (iter 0) ---

func TestEndToEndCleanModel(t *testing.T) {
	outputPath := "testdata/clean.checked.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "check", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "All relationship constraints satisfied")

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)
	assert.Empty(t, output.Findings)
}

func TestEndToEndViolation(t *testing.T) {
	outputPath := "testdata/violation.checked.yaml"
	defer os.Remove(outputPath)

	_, _, err := runApp(t, "check", "testdata/violation.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)
	assert.NotEmpty(t, output.Findings)
}

func TestEndToEndWithInlineDefinitions(t *testing.T) {
	outputPath := "testdata/inline-def.checked.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "check", "testdata/inline-def.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Assumptions")
}

func TestEndToEndNullPropagation(t *testing.T) {
	outputPath := "testdata/null-prop.checked.yaml"
	defer os.Remove(outputPath)

	_, _, err := runApp(t, "check", "testdata/null-prop.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var output model.CheckedOutput
	err = yaml.Unmarshal(data, &output)
	require.NoError(t, err)

	assert.Empty(t, output.Findings, "null propagation should produce known unknowns, not findings")
	assert.NotEmpty(t, output.KnownUnknowns)
}

// --- Cache refresh command (Phase 4, Step 4.1) ---

func TestCacheRefreshCreatesFile(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}
	stdout, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Cache refreshed")

	_, statErr := os.Stat(loader.CachePath(home))
	assert.NoError(t, statErr)
}

func TestCacheRefreshRebuildDeletesExisting(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	env := envConfig{modelrPath: pathDir, homeDir: home}

	// First refresh
	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	c1, err := loader.ReadCache(home)
	require.NoError(t, err)

	// Second refresh with --rebuild
	_, _, err = runAppWithEnv(t, env, "cache", "refresh", "--rebuild")
	require.NoError(t, err)

	c2, err := loader.ReadCache(home)
	require.NoError(t, err)
	assert.True(t, c2.RefreshedAt.After(c1.RefreshedAt) || c2.RefreshedAt.Equal(c1.RefreshedAt))
}

func TestCacheRefreshAutoWhenStale(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	env := envConfig{modelrPath: pathDir, homeDir: home}

	// Build cache
	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	// Make stale by adding a new file
	writePathDef(t, pathDir, "new.yaml", `---
kind: node
name: newnode
description: New
properties: {}
`)

	stdout, _, err := runAppWithEnv(t, env, "cache", "refresh", "--auto")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Cache refreshed")
}

func TestCacheRefreshAutoWhenFresh(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "test.yaml", `---
kind: node
name: testnode
description: Test
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	// Build cache
	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	// Auto with fresh cache
	stdout, _, err := runAppWithEnv(t, env, "cache", "refresh", "--auto")
	require.NoError(t, err)
	assert.Contains(t, stdout, "current")
}

func TestCacheRefreshMutuallyExclusiveFlags(t *testing.T) {
	env := envConfig{homeDir: t.TempDir()}
	_, _, err := runAppWithEnv(t, env, "cache", "refresh", "--rebuild", "--auto")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestCacheRefreshEmptyPath(t *testing.T) {
	home := t.TempDir()
	env := envConfig{modelrPath: "", homeDir: home}

	stdout, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)
	assert.Contains(t, stdout, "0 definitions")
}

func TestCacheRefreshPrintsShadowing(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "server.yaml", `---
kind: node
name: server
description: Custom server
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	_, stderr, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)
	assert.Contains(t, stderr, "INF")
	assert.Contains(t, stderr, "server")
	assert.Contains(t, stderr, "shadowed")
}

// --- Cache status command (Phase 4, Step 4.2) ---

func TestCacheStatusNoCache(t *testing.T) {
	home := t.TempDir()
	env := envConfig{homeDir: home}

	stdout, _, err := runAppWithEnv(t, env, "cache", "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "No cache found")
}

func TestCacheStatusFresh(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "test.yaml", `---
kind: node
name: testnode
description: Test
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	stdout, _, err := runAppWithEnv(t, env, "cache", "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "current")
	assert.Contains(t, stdout, "Last refreshed")
}

func TestCacheStatusStale(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "test.yaml", `---
kind: node
name: testnode
description: Test
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	// Make stale by adding new file
	writePathDef(t, pathDir, "new.yaml", `---
kind: node
name: newnode
description: New
properties: {}
`)

	stdout, _, err := runAppWithEnv(t, env, "cache", "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "stale")
}

func TestCacheStatusEntryCount(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "defs.yaml", `---
kind: node
name: a
description: A
properties: {}
---
kind: node
name: b
description: B
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	stdout, _, err := runAppWithEnv(t, env, "cache", "status")
	require.NoError(t, err)
	assert.Contains(t, stdout, "2")
}

// --- Pipeline auto-cache (Phase 4, Step 4.3) ---

func TestPipelineAutoCacheOnFirstRun(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "pg.yaml", `---
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
	env := envConfig{modelrPath: pathDir, homeDir: home}

	// Run validate on a model that uses a path def type
	stdout, _, err := runAppWithEnv(t, env, "validate", "testdata/clean.model.yaml")
	require.NoError(t, err)
	// Auto-cache should have been built
	_, statErr := os.Stat(loader.CachePath(home))
	assert.NoError(t, statErr)
	_ = stdout
}

func TestPipelineStaleWarning(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "test.yaml", `---
kind: node
name: testnode
description: Test
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	// Build cache
	_, _, err := runAppWithEnv(t, env, "cache", "refresh")
	require.NoError(t, err)

	// Make stale
	writePathDef(t, pathDir, "new.yaml", `---
kind: node
name: newnode
description: New
properties: {}
`)

	_, stderr, err := runAppWithEnv(t, env, "validate", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stderr, "WRN")
	assert.Contains(t, stderr, "definition cache is stale")
}

func TestPipelineNoCacheNoPath(t *testing.T) {
	home := t.TempDir()
	env := envConfig{modelrPath: "", homeDir: home}

	stdout, stderr, err := runAppWithEnv(t, env, "validate", "testdata/clean.model.yaml")
	require.NoError(t, err)
	// No auto-cache, no warning
	assert.NotContains(t, stderr, "warning")
	assert.NotContains(t, stderr, "info")
	_ = stdout
}

// --- Definitions command (Phase 5) ---

func TestDefinitionsCommandEmbeddedOnly(t *testing.T) {
	env := envConfig{modelrPath: "", homeDir: t.TempDir()}

	stdout, _, err := runAppWithEnv(t, env, "definitions")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Node Definitions:")
	assert.Contains(t, stdout, "server")
	assert.Contains(t, stdout, "datastore")
	assert.Contains(t, stdout, "queue")
	assert.Contains(t, stdout, "(embedded)")
	assert.Contains(t, stdout, "Relationship Definitions:")
	assert.Contains(t, stdout, "capacity_chain")
	assert.Contains(t, stdout, "pooled_capacity_chain")
}

func TestDefinitionsCommandWithPath(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL database
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	stdout, _, err := runAppWithEnv(t, env, "definitions")
	require.NoError(t, err)
	assert.Contains(t, stdout, "postgres")
	assert.Contains(t, stdout, pathDir)
}

func TestDefinitionsCommandShadowing(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "server.yaml", `---
kind: node
name: server
description: Custom server from path
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	stdout, _, err := runAppWithEnv(t, env, "definitions")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Custom server from path")
	assert.Contains(t, stdout, pathDir)
}

func TestDefinitionsCommandVerbose(t *testing.T) {
	env := envConfig{modelrPath: "", homeDir: t.TempDir()}

	stdout, _, err := runAppWithEnv(t, env, "definitions", "--verbose")
	require.NoError(t, err)
	assert.Contains(t, stdout, "max_connections")
	assert.Contains(t, stdout, "number")
	assert.Contains(t, stdout, "connections")
}

// --- Integration: check/validate with path defs (Phase 6) ---

func TestCheckWithPathDefinitions(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL
properties:
  max_connections:
    type: number
    unit: connections
    description: Max connections
  conn_establish_ms:
    type: number
    unit: ms
    description: Connection establishment time
    default: 25
`)
	// Write a model that uses the postgres type
	modelDir := t.TempDir()
	modelPath := filepath.Join(modelDir, "test.model.yaml")
	os.WriteFile(modelPath, []byte(`
version: "0.2"
name: pg-test
description: Test with postgres
components:
  - name: db
    type: postgres
    description: Primary DB
    properties:
      max_connections: 200
edges: []
`), 0644)

	env := envConfig{modelrPath: pathDir, homeDir: home}

	outputPath := filepath.Join(modelDir, "test.checked.yaml")
	defer os.Remove(outputPath)

	stdout, _, err := runAppWithEnv(t, env, "check", modelPath)
	require.NoError(t, err)

	// Should have an assumption for conn_establish_ms from the path def
	assert.Contains(t, stdout, "conn_establish_ms")
	assert.Contains(t, stdout, "25")
}

// --- Visualize command ---

func TestVisualizeCommandWritesDOT(t *testing.T) {
	dotPath := "testdata/clean.dot"
	defer os.Remove(dotPath)
	defer os.Remove("testdata/clean.svg")

	_, _, err := runApp(t, "visualize", "testdata/clean.model.yaml")
	require.NoError(t, err)

	_, statErr := os.Stat(dotPath)
	assert.NoError(t, statErr)
}

func TestVisualizeCommandContentValid(t *testing.T) {
	dotPath := "testdata/clean.dot"
	defer os.Remove(dotPath)
	defer os.Remove("testdata/clean.svg")

	_, _, err := runApp(t, "visualize", "testdata/clean.model.yaml")
	require.NoError(t, err)

	data, err := os.ReadFile(dotPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "digraph")
	assert.Contains(t, content, "api")
	assert.Contains(t, content, "db")
}

func TestVisualizeCommandVerbose(t *testing.T) {
	dotPath := "testdata/clean.dot"
	defer os.Remove(dotPath)
	defer os.Remove("testdata/clean.svg")

	stdout, _, err := runApp(t, "visualize", "--verbose", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Loading definitions")
}

func TestVerboseShowsDefinitionSources(t *testing.T) {
	home := t.TempDir()
	pathDir := t.TempDir()
	writePathDef(t, pathDir, "pg.yaml", `---
kind: node
name: postgres
description: PostgreSQL
properties: {}
`)
	env := envConfig{modelrPath: pathDir, homeDir: home}

	outputPath := "testdata/clean.checked.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runAppWithEnv(t, env, "check", "--verbose", "testdata/clean.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Loading definitions:")
	assert.Contains(t, stdout, "node/server")
	assert.Contains(t, stdout, "embedded")
	assert.Contains(t, stdout, "node/postgres")
	assert.Contains(t, stdout, pathDir)
}

// --- Init command ---

func TestInitCommandCreatesFiles(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	stdout, _, err := runApp(t, "init")
	require.NoError(t, err)
	assert.Contains(t, stdout, "created")
	assert.Contains(t, stdout, "SKILL.md")
	assert.Contains(t, stdout, "mcp.json")

	_, statErr := os.Stat(filepath.Join(dir, ".claude", "skills", "modelr-model", "SKILL.md"))
	assert.NoError(t, statErr)
	_, statErr = os.Stat(filepath.Join(dir, ".claude", "mcp.json"))
	assert.NoError(t, statErr)
}

func TestInitCommandSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// First run
	_, _, err := runApp(t, "init")
	require.NoError(t, err)

	// Second run
	stdout, _, err := runApp(t, "init")
	require.NoError(t, err)
	assert.Contains(t, stdout, "skipped")
	assert.Contains(t, stdout, "already exist")
}

func TestInitCommandOutput(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	stdout, _, err := runApp(t, "init")
	require.NoError(t, err)
	assert.Contains(t, stdout, "modelr-model")
	assert.Contains(t, stdout, "modelr-outage-report")
	assert.Contains(t, stdout, "modelr-verify")
	assert.Contains(t, stdout, "mcp.json")
	assert.Contains(t, stdout, "initialized")
}

// --- End-to-end: init creates valid content ---

func TestEndToEndInitCreatesValidSkills(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, _, err := runApp(t, "init")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "skills", "modelr-model", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "list_definitions")
	assert.Contains(t, string(data), "check_model")
}

func TestEndToEndInitMCPConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	_, _, err := runApp(t, "init")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".claude", "mcp.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "modelr")
	assert.Contains(t, string(data), "mcp")
}

// --- Verify command ---

func TestVerifyCommandPassingModel(t *testing.T) {
	outputPath := "testdata/verify-pass.verified.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "verify", "testdata/verify-pass.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Accepted")

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

func TestVerifyCommandFailingModel(t *testing.T) {
	outputPath := "testdata/verify-fail.verified.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "verify", "testdata/verify-fail.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Rejected")

	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

func TestVerifyCommandNoPatterns(t *testing.T) {
	// Use a model with no relationships at all
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "test.model.yaml")
	os.WriteFile(modelPath, []byte(`
version: "0.2"
name: no-patterns
description: No relationships
components:
  - name: api
    type: server
    description: API
edges: []
`), 0644)

	stdout, _, err := runAppWithEnv(t, envConfig{homeDir: t.TempDir()}, "verify", modelPath)
	require.NoError(t, err)
	assert.Contains(t, stdout, "No behavioral patterns")
}

func TestVerifyCommandFlags(t *testing.T) {
	outputPath := "testdata/verify-pass.verified.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "verify", "--failure-rate", "0.01", "--confidence", "0.95", "testdata/verify-pass.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Accepted")
}

func TestVerifyCommandVerbose(t *testing.T) {
	outputPath := "testdata/verify-pass.verified.yaml"
	defer os.Remove(outputPath)

	stdout, _, err := runApp(t, "verify", "--verbose", "testdata/verify-pass.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Loading definitions")
}

func TestVerifyVerboseShrinkOutput(t *testing.T) {
	outputPath := "testdata/verify-fail.verified.yaml"
	defer os.Remove(outputPath)

	_, stderr, err := runApp(t, "verify", "--verbose", "testdata/verify-fail.model.yaml")
	require.NoError(t, err)
	assert.Contains(t, stderr, "INF")
	assert.Contains(t, stderr, "shrink progress")
	assert.Contains(t, stderr, "delete_chunks")
}

// --- End-to-end: check then verify ---

func TestEndToEndCheckThenVerify(t *testing.T) {
	checkedPath := "testdata/verify-pass.checked.yaml"
	verifiedPath := "testdata/verify-pass.verified.yaml"
	defer os.Remove(checkedPath)
	defer os.Remove(verifiedPath)

	_, _, err := runApp(t, "check", "testdata/verify-pass.model.yaml")
	require.NoError(t, err)

	_, _, err = runApp(t, "verify", "testdata/verify-pass.model.yaml")
	require.NoError(t, err)

	_, err = os.Stat(checkedPath)
	assert.NoError(t, err)
	_, err = os.Stat(verifiedPath)
	assert.NoError(t, err)
}
