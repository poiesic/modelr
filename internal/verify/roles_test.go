package verify

import (
	"testing"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func capacityChainTemplate() loader.RelationshipDef {
	return loader.RelationshipDef{
		Name:    "capacity_chain",
		Pattern: PatternFiniteResource,
		Resolve: map[string]string{
			"upstream_rate":        "upstream.max_connections",
			"instances":            "upstream.max_instances",
			"operation_cost":       "edge.avg_operation_ms",
			"downstream_capacity":  "downstream.max_connections",
			"downstream_instances": "downstream.max_instances",
		},
	}
}

func pooledTemplate() loader.RelationshipDef {
	return loader.RelationshipDef{
		Name:    "pooled_capacity_chain",
		Pattern: PatternFinitePooledResource,
		Resolve: map[string]string{
			"max_pool_size":       "edge.max_pool_size",
			"instances":           "upstream.max_instances",
			"operation_cost":      "edge.avg_operation_ms",
			"downstream_capacity": "downstream.max_connections",
			"downstream_max_ops":  "downstream.max_ops_per_sec",
			"conn_establish_ms":   "downstream.conn_establish_ms",
		},
	}
}

func testModel() *model.SystemModel {
	return &model.SystemModel{
		Components: []model.Component{
			{Name: "api", Type: "server", Properties: map[string]any{
				"max_connections": 100, "max_instances": 10,
			}},
			{Name: "db", Type: "datastore", Properties: map[string]any{
				"max_connections": 200, "max_ops_per_sec": 5000, "conn_establish_ms": 20,
			}},
		},
		Edges: []model.Edge{
			{Source: "api", Target: "db", Properties: map[string]any{
				"avg_operation_ms": 5, "max_pool_size": 20,
			}},
		},
	}
}

func TestInferRolesFiniteResource(t *testing.T) {
	tmpl := capacityChainTemplate()
	rel := model.Relationship{Template: "capacity_chain", Upstream: "api", Downstream: "db"}

	rm, err := InferRoles(tmpl, rel, testModel())
	require.NoError(t, err)
	assert.Equal(t, 10, rm.MaxInstances)
	assert.Equal(t, 200, rm.ResourceCapacity)
	assert.Equal(t, 5, rm.OperationTime)
}

func TestInferRolesFinitePooledResource(t *testing.T) {
	tmpl := pooledTemplate()
	rel := model.Relationship{Template: "pooled_capacity_chain", Upstream: "api", Downstream: "db"}

	rm, err := InferRoles(tmpl, rel, testModel())
	require.NoError(t, err)
	assert.Equal(t, 10, rm.MaxInstances)
	assert.Equal(t, 200, rm.ResourceCapacity)
	assert.Equal(t, 20, rm.PoolCapacity)
	assert.Equal(t, 20, rm.AcquireTime)
}

func TestInferRolesUnknownPattern(t *testing.T) {
	tmpl := loader.RelationshipDef{Name: "test", Pattern: "unknown"}
	rel := model.Relationship{Upstream: "api", Downstream: "db"}

	_, err := InferRoles(tmpl, rel, testModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pattern")
}

func TestInferRolesMissingBinding(t *testing.T) {
	tmpl := loader.RelationshipDef{
		Name:    "broken",
		Pattern: PatternFiniteResource,
		Resolve: map[string]string{
			"downstream_capacity": "downstream.max_connections",
			// missing instances and operation_time
		},
	}
	rel := model.Relationship{Upstream: "api", Downstream: "db"}

	_, err := InferRoles(tmpl, rel, testModel())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instances")
}

func TestInferRolesFromCustomTemplate(t *testing.T) {
	tmpl := loader.RelationshipDef{
		Name:    "custom_check",
		Pattern: PatternFiniteResource,
		Resolve: map[string]string{
			"instances":           "upstream.max_instances",
			"downstream_capacity": "downstream.max_connections",
			"op_time":             "edge.avg_operation_ms",
		},
	}
	rel := model.Relationship{Upstream: "api", Downstream: "db"}

	rm, err := InferRoles(tmpl, rel, testModel())
	require.NoError(t, err)
	assert.Equal(t, 10, rm.MaxInstances)
	assert.Equal(t, 200, rm.ResourceCapacity)
	assert.Equal(t, 5, rm.OperationTime)
}
