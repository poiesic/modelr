package verify

import (
	"testing"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildRegistry(t *testing.T) *loader.Registry {
	t.Helper()
	reg, err := loader.BuildRegistry(loader.DefinitionSources{})
	require.NoError(t, err)
	return reg
}

func TestComposeSingleRelationship(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{"max_connections": 100, "max_instances": 5}},
			{Name: "db", Type: "datastore", Properties: map[string]any{"max_connections": 200}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	comp, warnings, err := Compose(m, buildRegistry(t))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Len(t, comp.Machines, 1)
}

func TestComposeTwoRelationshipsDifferentDownstream(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{"max_connections": 100, "max_instances": 5}},
			{Name: "db", Type: "datastore", Properties: map[string]any{"max_connections": 200}},
			{Name: "cache", Type: "datastore", Properties: map[string]any{"max_connections": 300}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10}},
			{Source: "api", Target: "cache", Properties: map[string]any{"avg_operation_ms": 5}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
			{Template: "capacity_chain", Upstream: "api", Downstream: "cache"},
		},
	}
	comp, _, err := Compose(m, buildRegistry(t))
	require.NoError(t, err)
	assert.Len(t, comp.Machines, 2)
}

func TestComposeSkipsNoPattern(t *testing.T) {
	// Create a custom template without pattern
	reg, err := loader.BuildRegistry(loader.DefinitionSources{
		InlineRels: []loader.RelationshipDef{
			{Name: "no_pattern", Resolve: map[string]string{}, Checks: nil},
		},
	})
	require.NoError(t, err)

	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "a", Type: "server", Properties: map[string]any{"max_instances": 1}},
			{Name: "b", Type: "server", Properties: map[string]any{}},
		},
		Relationships: []model.Relationship{
			{Template: "no_pattern", Upstream: "a", Downstream: "b"},
		},
	}
	comp, _, err := Compose(m, reg)
	require.NoError(t, err)
	assert.Empty(t, comp.Machines)
}

func TestComposeUnknownPatternWarning(t *testing.T) {
	reg, err := loader.BuildRegistry(loader.DefinitionSources{
		InlineRels: []loader.RelationshipDef{
			{Name: "weird", Pattern: "weird_pattern", Resolve: map[string]string{}},
		},
	})
	require.NoError(t, err)

	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "a", Type: "server", Properties: map[string]any{}},
			{Name: "b", Type: "server", Properties: map[string]any{}},
		},
		Relationships: []model.Relationship{
			{Template: "weird", Upstream: "a", Downstream: "b"},
		},
	}
	comp, warnings, err := Compose(m, reg)
	require.NoError(t, err)
	assert.Empty(t, comp.Machines)
	assert.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "unknown pattern")
}
