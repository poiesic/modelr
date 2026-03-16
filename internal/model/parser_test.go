package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalModel = `
version: "0.2"
name: test-system
description: A test system
components:
  - name: api
    type: server
    description: API server
edges: []
`

func TestParseMinimalModel(t *testing.T) {
	result, err := Parse(strings.NewReader(minimalModel))
	require.NoError(t, err)
	require.NotNil(t, result.Model)

	m := result.Model
	assert.Equal(t, "0.2", m.Version)
	assert.Equal(t, "test-system", m.Name)
	assert.Equal(t, "A test system", m.Description)
	assert.Len(t, m.Components, 1)
	assert.Equal(t, "api", m.Components[0].Name)
	assert.Equal(t, "server", m.Components[0].Type)
	assert.Empty(t, m.Edges)
}

func TestParseComponentProperties(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: db
    type: datastore
    description: Database
    properties:
      max_connections: 100
      engine: postgres
edges: []
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	props := result.Model.Components[0].Properties
	assert.Equal(t, 100, props["max_connections"])
	assert.Equal(t, "postgres", props["engine"])
}

func TestParseNullProperty(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: db
    type: datastore
    description: Database
    properties:
      max_connections: null
edges: []
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	props := result.Model.Components[0].Properties
	assert.Contains(t, props, "max_connections")
	assert.Nil(t, props["max_connections"])
}

func TestParseEdges(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: db
    type: datastore
    description: DB
edges:
  - source: api
    target: db
    operation: read_write
    description: API reads/writes DB
    properties:
      avg_operation_ms: 5
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	require.Len(t, result.Model.Edges, 1)
	e := result.Model.Edges[0]
	assert.Equal(t, "api", e.Source)
	assert.Equal(t, "db", e.Target)
	assert.Equal(t, "read_write", e.Operation)
	assert.Equal(t, "API reads/writes DB", e.Description)
	assert.Equal(t, 5, e.Properties["avg_operation_ms"])
}

func TestParseRelationships(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: db
    type: datastore
    description: DB
edges: []
relationships:
  - template: capacity_chain
    upstream: api
    downstream: db
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	require.Len(t, result.Model.Relationships, 1)
	r := result.Model.Relationships[0]
	assert.Equal(t, "capacity_chain", r.Template)
	assert.Equal(t, "api", r.Upstream)
	assert.Equal(t, "db", r.Downstream)
}

func TestParseKnownUnknowns(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges: []
known_unknowns:
  - id: ku-1
    component: api
    category: unstated_constraint
    description: Max RPS unknown
    impact: Could overload under peak traffic
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	require.Len(t, result.Model.KnownUnknowns, 1)
	ku := result.Model.KnownUnknowns[0]
	assert.Equal(t, "ku-1", ku.ID)
	assert.Equal(t, "api", ku.Component)
	assert.Equal(t, "unstated_constraint", ku.Category)
	assert.Equal(t, "Max RPS unknown", ku.Description)
	assert.Equal(t, "Could overload under peak traffic", ku.Impact)
}

// --- Structural validation errors (Step 2.3) ---

func TestParseMissingVersion(t *testing.T) {
	input := `
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestParseMissingName(t *testing.T) {
	input := `
version: "0.2"
description: test
components:
  - name: api
    type: server
    description: API
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestParseMissingDescription(t *testing.T) {
	input := `
version: "0.2"
name: test
components:
  - name: api
    type: server
    description: API
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestParseMissingComponents(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "components")
}

func TestParseEmptyComponents(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components: []
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "components must not be empty")
}

func TestParseMissingEdges(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edges")
}

func TestParseMissingComponentName(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - type: server
    description: API
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component[0]: missing required field: name")
}

func TestParseMissingComponentType(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    description: API
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component[0]: missing required field: type")
}

func TestParseMissingComponentDescription(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "component[0]: missing required field: description")
}

func TestParseMissingEdgeSource(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges:
  - target: api
    operation: read
    description: test
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edge[0]: missing required field: source")
}

func TestParseMissingEdgeTarget(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges:
  - source: api
    operation: read
    description: test
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edge[0]: missing required field: target")
}

func TestParseMissingEdgeOperation(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: db
    type: datastore
    description: DB
edges:
  - source: api
    target: db
    description: test
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edge[0]: missing required field: operation")
}

func TestParseMissingEdgeDescription(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: db
    type: datastore
    description: DB
edges:
  - source: api
    target: db
    operation: read
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "edge[0]: missing required field: description")
}

func TestParseInvalidOperation(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: db
    type: datastore
    description: DB
edges:
  - source: api
    target: db
    operation: delete
    description: test
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid operation")
}

// --- Uniqueness and reference validation (Step 2.4) ---

func TestParseDuplicateComponentNames(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: api
    type: server
    description: Another API
edges: []
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate component name")
}

func TestParseEdgeUnknownSource(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges:
  - source: unknown
    target: api
    operation: read
    description: test
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source component")
}

func TestParseEdgeUnknownTarget(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges:
  - source: api
    target: unknown
    operation: read
    description: test
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown target component")
}

func TestParseRelationshipUnknownUpstream(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges: []
relationships:
  - template: capacity_chain
    upstream: unknown
    downstream: api
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown upstream component")
}

func TestParseRelationshipUnknownDownstream(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges: []
relationships:
  - template: capacity_chain
    upstream: api
    downstream: unknown
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown downstream component")
}

func TestParseRejectsPoolSize(t *testing.T) {
	input := `
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
  - name: db
    type: datastore
    description: DB
edges:
  - source: api
    target: db
    operation: read
    description: test
    properties:
      pool_size: 10
`
	_, err := Parse(strings.NewReader(input))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pool_size")
	assert.Contains(t, err.Error(), "min_pool_size")
	assert.Contains(t, err.Error(), "max_pool_size")
}

// --- Inline definitions (Step 2.5) ---

func TestParseInlineNodeDefinition(t *testing.T) {
	input := `---
kind: node
name: custom_server
description: Custom server type
properties:
  max_rps:
    type: number
    unit: req/s
    description: Max requests per second
---
version: "0.2"
name: test
description: test
components:
  - name: api
    type: custom_server
    description: API
edges: []
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, result.Model)
	require.Len(t, result.InlineNodes, 1)
	assert.Equal(t, "custom_server", result.InlineNodes[0].Name)
	assert.Contains(t, result.InlineNodes[0].Properties, "max_rps")
}

func TestParseInlineRelationshipDefinition(t *testing.T) {
	input := `---
kind: relationship
name: custom_check
description: Custom check
resolve:
  rate: upstream.max_rps
checks:
  - name: rate_check
    expression: "rate <= 1000"
    violation: "Rate too high"
---
version: "0.2"
name: test
description: test
components:
  - name: api
    type: server
    description: API
edges: []
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, result.InlineRels, 1)
	assert.Equal(t, "custom_check", result.InlineRels[0].Name)
}

func TestParseMultipleInlineDefinitions(t *testing.T) {
	input := `---
kind: node
name: type_a
description: Type A
properties: {}
---
kind: node
name: type_b
description: Type B
properties: {}
---
kind: relationship
name: rel_a
description: Rel A
resolve: {}
checks: []
---
version: "0.2"
name: test
description: test
components:
  - name: a
    type: type_a
    description: A
edges: []
`
	result, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, result.InlineNodes, 2)
	assert.Len(t, result.InlineRels, 1)
	require.NotNil(t, result.Model)
}
