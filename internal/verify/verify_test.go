package verify

import (
	"testing"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func passingModel() *model.SystemModel {
	return &model.SystemModel{
		Name: "passing",
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 10, "max_instances": 3,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 10000,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
}

func failingModel() *model.SystemModel {
	return &model.SystemModel{
		Name: "failing",
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 100, "max_instances": 50,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 50,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
}

func TestVerifyPassingModel(t *testing.T) {
	reg := buildRegistry(t)
	config := DefaultVerifyConfig()
	config.Seed = 42

	result, err := Verify(passingModel(), reg, &model.ValidationResult{}, config)
	require.NoError(t, err)
	require.Len(t, result.Verifications, 1)
	assert.Equal(t, "pass", result.Verifications[0].Result)
	assert.Equal(t, 0, result.Verifications[0].Failures)
	assert.Contains(t, result.Summary, "Accepted")
}

func TestVerifyFailingModel(t *testing.T) {
	reg := buildRegistry(t)
	config := DefaultVerifyConfig()
	config.Seed = 42

	result, err := Verify(failingModel(), reg, &model.ValidationResult{}, config)
	require.NoError(t, err)
	require.Len(t, result.Verifications, 1)
	assert.Equal(t, "fail", result.Verifications[0].Result)
	assert.Greater(t, result.Verifications[0].Failures, 0)
	assert.NotEmpty(t, result.Verifications[0].MinimalFailure)
	assert.Contains(t, result.Summary, "Rejected")
}

func TestVerifyNoPatterns(t *testing.T) {
	m := &model.SystemModel{
		Name: "no-patterns",
		Components: []model.Component{
			{Name: "a", Type: "server", Properties: map[string]any{}},
		},
	}
	reg := buildRegistry(t)

	result, err := Verify(m, reg, &model.ValidationResult{}, DefaultVerifyConfig())
	require.NoError(t, err)
	assert.Empty(t, result.Verifications)
	assert.Contains(t, result.Summary, "No behavioral patterns")
}

func TestVerifyMixedRelationships(t *testing.T) {
	reg, err := loader.BuildRegistry(loader.DefinitionSources{
		InlineRels: []loader.RelationshipDef{
			{Name: "no_pattern", Resolve: map[string]string{}, Checks: nil},
		},
	})
	require.NoError(t, err)

	m := &model.SystemModel{
		Name: "mixed",
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 10, "max_instances": 3,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 10000,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
			{Template: "no_pattern", Upstream: "api", Downstream: "db"},
		},
	}

	config := DefaultVerifyConfig()
	config.Seed = 42

	result, err := Verify(m, reg, &model.ValidationResult{}, config)
	require.NoError(t, err)
	assert.Len(t, result.Verifications, 1) // only the patterned one
}

func TestVerifyDeterministic(t *testing.T) {
	reg := buildRegistry(t)
	config := DefaultVerifyConfig()
	config.Seed = 42

	r1, err := Verify(failingModel(), reg, &model.ValidationResult{}, config)
	require.NoError(t, err)

	config.Seed = 42
	r2, err := Verify(failingModel(), reg, &model.ValidationResult{}, config)
	require.NoError(t, err)

	assert.Equal(t, r1.Verifications[0].Result, r2.Verifications[0].Result)
	assert.Equal(t, r1.Verifications[0].Simulations, r2.Verifications[0].Simulations)
}
