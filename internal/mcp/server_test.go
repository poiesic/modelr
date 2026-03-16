package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEnv(t *testing.T) EnvConfig {
	t.Helper()
	return EnvConfig{
		ModelrPath: "",
		HomeDir:    t.TempDir(),
	}
}

func writeModel(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

type testClient struct {
	t      *testing.T
	client *client.Client
}

func newTestClient(t *testing.T, env EnvConfig) *testClient {
	t.Helper()
	s := NewServer(env)
	c, err := client.NewInProcessClient(s)
	require.NoError(t, err)
	require.NoError(t, c.Start(context.Background()))
	_, err = c.Initialize(context.Background(), mcplib.InitializeRequest{})
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	return &testClient{t: t, client: c}
}

func (tc *testClient) callTool(name string, args map[string]any) string {
	tc.t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	result, err := tc.client.CallTool(context.Background(), req)
	require.NoError(tc.t, err)
	require.NotNil(tc.t, result)
	require.NotEmpty(tc.t, result.Content)

	for _, c := range result.Content {
		if textContent, ok := c.(mcplib.TextContent); ok {
			return textContent.Text
		}
	}
	tc.t.Fatal("no text content in result")
	return ""
}

const cleanModel = `
version: "0.2"
name: test-system
description: A test system
components:
  - name: api
    type: server
    description: API server
    properties:
      max_connections: 100
      max_instances: 2
  - name: db
    type: datastore
    description: Database
    properties:
      max_connections: 500
edges:
  - source: api
    target: db
    operation: read_write
    description: API to DB
    properties:
      avg_operation_ms: 10
relationships:
  - template: capacity_chain
    upstream: api
    downstream: db
`

const violationModel = `
version: "0.2"
name: oversubscribed
description: Oversubscribed system
components:
  - name: api
    type: server
    description: API server
    properties:
      max_connections: 1000
      max_instances: 50
  - name: db
    type: datastore
    description: Database
    properties:
      max_connections: 100
edges:
  - source: api
    target: db
    operation: read_write
    description: API to DB
    properties:
      avg_operation_ms: 100
relationships:
  - template: capacity_chain
    upstream: api
    downstream: db
`

// --- Server setup ---

func TestNewServerHasTools(t *testing.T) {
	tc := newTestClient(t, testEnv(t))
	require.NotNil(t, tc)
}

// --- list_definitions ---

func TestListDefinitionsReturnsEmbedded(t *testing.T) {
	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("list_definitions", nil)

	assert.Contains(t, text, "server")
	assert.Contains(t, text, "datastore")
	assert.Contains(t, text, "queue")
	assert.Contains(t, text, "capacity_chain")
	assert.Contains(t, text, "pooled_capacity_chain")
}

func TestListDefinitionsStructure(t *testing.T) {
	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("list_definitions", nil)

	assert.Contains(t, text, "Node Definitions:")
	assert.Contains(t, text, "Relationship Definitions:")
}

// --- check_model ---

func TestCheckModelValid(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("check_model", map[string]any{"path": modelPath})

	assert.Contains(t, text, "All relationship constraints satisfied")
	_, statErr := os.Stat(filepath.Join(dir, "test.checked.yaml"))
	assert.NoError(t, statErr)
}

func TestCheckModelFindings(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", violationModel)

	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("check_model", map[string]any{"path": modelPath})

	assert.Contains(t, text, "Findings:")
}

func TestCheckModelBadPath(t *testing.T) {
	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("check_model", map[string]any{"path": "/nonexistent.model.yaml"})
	assert.Contains(t, text, "opening")
}

// --- validate_model ---

func TestValidateModelValid(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("validate_model", map[string]any{"path": modelPath})

	assert.NotEmpty(t, text)
}

func TestValidateModelBadPath(t *testing.T) {
	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("validate_model", map[string]any{"path": "/nonexistent.model.yaml"})
	assert.Contains(t, text, "opening")
}

// --- visualize_model ---

func TestVisualizeModelReturnsDOT(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("visualize_model", map[string]any{"path": modelPath})

	assert.Contains(t, text, "digraph")
}

func TestVisualizeModelWritesFile(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	tc.callTool("visualize_model", map[string]any{"path": modelPath})

	_, statErr := os.Stat(filepath.Join(dir, "test.dot"))
	assert.NoError(t, statErr)
}

// --- verify_model ---

func TestVerifyModelPassing(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("verify_model", map[string]any{"path": modelPath})
	assert.Contains(t, text, "Accepted")
}

func TestVerifyModelFailing(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", violationModel)

	tc := newTestClient(t, testEnv(t))
	text := tc.callTool("verify_model", map[string]any{"path": modelPath})
	assert.Contains(t, text, "Rejected")
}

func TestVerifyModelWritesFile(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	tc.callTool("verify_model", map[string]any{"path": modelPath})

	_, statErr := os.Stat(filepath.Join(dir, "test.verified.yaml"))
	assert.NoError(t, statErr)
}

// --- Integration ---

func TestMCPCheckThenVisualize(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	tc.callTool("check_model", map[string]any{"path": modelPath})
	tc.callTool("visualize_model", map[string]any{"path": modelPath})

	_, statErr := os.Stat(filepath.Join(dir, "test.checked.yaml"))
	assert.NoError(t, statErr)
	_, statErr = os.Stat(filepath.Join(dir, "test.dot"))
	assert.NoError(t, statErr)
}

func TestMCPListThenCheck(t *testing.T) {
	dir := t.TempDir()
	modelPath := writeModel(t, dir, "test.model.yaml", cleanModel)

	tc := newTestClient(t, testEnv(t))
	listText := tc.callTool("list_definitions", nil)
	assert.Contains(t, listText, "server")

	checkText := tc.callTool("check_model", map[string]any{"path": modelPath})
	assert.Contains(t, checkText, "constraints satisfied")
}
