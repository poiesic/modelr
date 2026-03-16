package check

import (
	"testing"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildDefaultRegistry(t *testing.T) *loader.Registry {
	t.Helper()
	reg, err := loader.BuildRegistry(loader.DefinitionSources{})
	require.NoError(t, err)
	return reg
}

// --- Step 6.1: Resolve variables ---

func TestResolveUpstreamVar(t *testing.T) {
	tmpl := &loader.RelationshipDef{
		Resolve: map[string]string{"rate": "upstream.max_connections"},
	}
	upstream := &model.Component{Properties: map[string]any{"max_connections": 100.0}}
	downstream := &model.Component{Properties: map[string]any{}}

	vars := resolveVariables(tmpl, upstream, downstream, nil)
	assert.Equal(t, 100.0, vars["rate"])
}

func TestResolveDownstreamVar(t *testing.T) {
	tmpl := &loader.RelationshipDef{
		Resolve: map[string]string{"cap": "downstream.max_connections"},
	}
	upstream := &model.Component{Properties: map[string]any{}}
	downstream := &model.Component{Properties: map[string]any{"max_connections": 200.0}}

	vars := resolveVariables(tmpl, upstream, downstream, nil)
	assert.Equal(t, 200.0, vars["cap"])
}

func TestResolveEdgeVar(t *testing.T) {
	tmpl := &loader.RelationshipDef{
		Resolve: map[string]string{"cost": "edge.avg_operation_ms"},
	}
	upstream := &model.Component{Properties: map[string]any{}}
	downstream := &model.Component{Properties: map[string]any{}}
	edge := &model.Edge{Properties: map[string]any{"avg_operation_ms": 5.0}}

	vars := resolveVariables(tmpl, upstream, downstream, edge)
	assert.Equal(t, 5.0, vars["cost"])
}

func TestResolveMissingProperty(t *testing.T) {
	tmpl := &loader.RelationshipDef{
		Resolve: map[string]string{"rate": "upstream.nonexistent"},
	}
	upstream := &model.Component{Properties: map[string]any{}}
	downstream := &model.Component{Properties: map[string]any{}}

	vars := resolveVariables(tmpl, upstream, downstream, nil)
	assert.Nil(t, vars["rate"])
}

func TestResolveStringProperty(t *testing.T) {
	tmpl := &loader.RelationshipDef{
		Resolve: map[string]string{"name": "upstream.engine"},
	}
	upstream := &model.Component{Properties: map[string]any{"engine": "postgres"}}
	downstream := &model.Component{Properties: map[string]any{}}

	vars := resolveVariables(tmpl, upstream, downstream, nil)
	assert.Equal(t, "postgres", vars["name"])
}

func TestResolveMissingEdge(t *testing.T) {
	tmpl := &loader.RelationshipDef{
		Resolve: map[string]string{"cost": "edge.avg_operation_ms"},
	}
	upstream := &model.Component{Properties: map[string]any{}}
	downstream := &model.Component{Properties: map[string]any{}}

	vars := resolveVariables(tmpl, upstream, downstream, nil)
	assert.Nil(t, vars["cost"])
}

// --- Step 6.2: Check execution ---

func TestCheckPassingRelationship(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 2.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 500.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestCheckFailingRelationship(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 1000.0, "max_instances": 50.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 100.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	require.NotEmpty(t, result.Findings)
	f := result.Findings[0]
	assert.Equal(t, "error", f.Severity)
	assert.Equal(t, "api", f.Upstream)
	assert.Equal(t, "db", f.Downstream)
	assert.Equal(t, "capacity_chain", f.Relationship)
	assert.Contains(t, f.Description, "throughput")
}

func TestCheckIndeterminateResult(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": nil, "max_instances": 1.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	assert.Empty(t, result.Findings)
	assert.NotEmpty(t, result.KnownUnknowns)
}

func TestCheckSkipsStringVariables(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": "unlimited", "max_instances": 1.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	assert.Empty(t, result.Findings)
	assert.Empty(t, result.KnownUnknowns)
}

func TestCheckMultipleRelationships(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 2.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 500.0, "max_instances": 1.0,
			}},
			{Name: "cache", Type: "datastore", Properties: map[string]any{
				"max_connections": 500.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10.0}},
			{Source: "api", Target: "cache", Properties: map[string]any{"avg_operation_ms": 5.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
			{Template: "capacity_chain", Upstream: "api", Downstream: "cache"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	// Both should pass
	assert.Empty(t, result.Findings)
	assert.Equal(t, "All relationship constraints satisfied.", result.Summary)
}

func TestCheckMultipleChecksPerTemplate(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_instances": 10.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections":  100.0,
				"max_ops_per_sec":  5000.0,
				"conn_establish_ms": 20.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{
				"min_pool_size":    5.0,
				"max_pool_size":    20.0,
				"avg_operation_ms": 50.0,
			}},
		},
		Relationships: []model.Relationship{
			{Template: "pooled_capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	// max_pool_size(20) * instances(10) = 200 > downstream_capacity(100) → violation
	// throughput: 20 * 10 * 1000 / 50 = 4000 <= 5000 → pass
	require.Len(t, result.Findings, 1)
	assert.Contains(t, result.Findings[0].Description, "connection_limit")
}

// --- Step 6.3: Finding descriptions ---

func TestFindingIncludesResolvedValues(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 1000.0, "max_instances": 50.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 100.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	require.NotEmpty(t, result.Findings)
	// Description should include substituted values
	assert.Contains(t, result.Findings[0].Description, "1000")
	assert.Contains(t, result.Findings[0].Description, "50")
}

func TestFindingSuggestion(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 1000.0, "max_instances": 50.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 100.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	require.NotEmpty(t, result.Findings)
	assert.Contains(t, result.Findings[0].Suggestion, "Upstream throughput may exceed downstream capacity")
}

// --- Step 6.4: Check result summary ---

func TestCheckResultSummaryClean(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 10.0, "max_instances": 1.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 500.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)
	assert.Equal(t, "All relationship constraints satisfied.", result.Summary)
}

func TestCheckResultSummaryViolations(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_instances": 100.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections":  50.0,
				"max_ops_per_sec":  100.0,
				"conn_establish_ms": 20.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{
				"min_pool_size":    5.0,
				"max_pool_size":    20.0,
				"avg_operation_ms": 10.0,
			}},
		},
		Relationships: []model.Relationship{
			{Template: "pooled_capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)
	// 20*100=2000 > 50 (connection_limit fail)
	// 20*100*1000/10=200000 > 100 (throughput fail)
	assert.Equal(t, "Found 2 relationship violation(s).", result.Summary)
}

func TestCheckResultMergesKnownUnknowns(t *testing.T) {
	m := &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": nil, "max_instances": 1.0,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 100.0, "max_instances": 1.0,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{"avg_operation_ms": 10.0}},
		},
		Relationships: []model.Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Check(m, reg)
	require.NoError(t, err)

	assert.NotEmpty(t, result.KnownUnknowns)
	assert.Contains(t, result.KnownUnknowns[0].Description, "indeterminate")
}
