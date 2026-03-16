package model

import (
	"testing"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildDefaultRegistry(t *testing.T) *loader.Registry {
	t.Helper()
	reg, err := loader.BuildRegistry(loader.DefinitionSources{})
	require.NoError(t, err)
	return reg
}

// --- Step 4.1: Fill defaults ---

func TestValidateFillsDefaultMinInstances(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{
				"max_connections": 100,
			}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	assert.Equal(t, 1, m.Components[0].Properties["min_instances"])

	var found bool
	for _, a := range result.Assumptions {
		if a.Component == "api" && a.Property == "min_instances" {
			assert.Equal(t, 1, a.Value)
			assert.Equal(t, "server type schema", a.Source)
			found = true
		}
	}
	assert.True(t, found, "expected assumption for min_instances")
}

func TestValidateFillsDefaultMaxInstances(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{
				"max_connections": 100,
			}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	assert.Equal(t, 1, m.Components[0].Properties["max_instances"])

	var found bool
	for _, a := range result.Assumptions {
		if a.Component == "api" && a.Property == "max_instances" {
			found = true
		}
	}
	assert.True(t, found, "expected assumption for max_instances")
}

func TestValidateFillsDefaultConnEstablishMs(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{
				"max_connections": 100,
			}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	assert.Equal(t, 20, m.Components[0].Properties["conn_establish_ms"])

	var found bool
	for _, a := range result.Assumptions {
		if a.Component == "db" && a.Property == "conn_establish_ms" {
			assert.Equal(t, 20, a.Value)
			assert.Equal(t, "datastore type schema", a.Source)
			found = true
		}
	}
	assert.True(t, found, "expected assumption for conn_establish_ms")
}

func TestValidateExplicitValueNotOverridden(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{
				"min_instances": 4,
			}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	assert.Equal(t, 4, m.Components[0].Properties["min_instances"])

	for _, a := range result.Assumptions {
		if a.Component == "api" && a.Property == "min_instances" {
			t.Fatal("should not have assumption for explicitly set min_instances")
		}
	}
}

// --- Step 4.2: Known unknowns for missing properties ---

func TestValidateMissingPropertyNoDefault(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	// max_connections has no default in server schema
	assert.Nil(t, m.Components[0].Properties["max_connections"])

	var found bool
	for _, ku := range result.KnownUnknowns {
		if ku.Component == "api" && ku.Category == "unstated_constraint" {
			if assert.Contains(t, ku.Description, "max_connections") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected known unknown for max_connections")
}

func TestValidateKnownUnknownHasComponentRef(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	for _, ku := range result.KnownUnknowns {
		if ku.Component == "api" {
			assert.NotEmpty(t, ku.Component)
			return
		}
	}
	t.Fatal("expected known unknown referencing component api")
}

func TestValidateKnownUnknownDescription(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	var found bool
	for _, ku := range result.KnownUnknowns {
		if ku.Component == "api" {
			assert.Contains(t, ku.Description, "max_connections")
			found = true
		}
	}
	assert.True(t, found)
}

// --- Step 4.3: Unknown component types ---

func TestValidateUnknownTypeWarning(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "x", Type: "custom_thing", Description: "Custom", Properties: map[string]any{"foo": 42}},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "custom_thing")
}

func TestValidateUnknownTypePropertiesUntouched(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "x", Type: "custom_thing", Description: "Custom", Properties: map[string]any{"foo": 42}},
		},
	}
	reg := buildDefaultRegistry(t)
	_, err := Validate(m, reg)
	require.NoError(t, err)

	assert.Equal(t, 42, m.Components[0].Properties["foo"])
	assert.Len(t, m.Components[0].Properties, 1)
}

// --- Step 4.4: Relationship binding validation ---

func TestValidateRelationshipBindings(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{
				"max_connections": 100,
				"max_instances":   2,
			}},
			{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{
				"max_connections": 200,
				"max_instances":   1,
			}},
		},
		Edges: []Edge{
			{Source: "api", Target: "db", Operation: "read", Description: "test", Properties: map[string]any{
				"avg_operation_ms": 10,
			}},
		},
		Relationships: []Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	// Should not have warnings about missing edges
	for _, w := range result.Warnings {
		assert.NotContains(t, w, "no edge found")
	}
	// Note: capacity_chain binds downstream.max_instances, which is not in the datastore schema.
	// This is a valid warning per spec section 7.7.
}

func TestValidateRelationshipMissingEdge(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
			{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}},
		},
		Edges:         []Edge{},
		Relationships: []Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	var found bool
	for _, w := range result.Warnings {
		if assert.Condition(t, func() bool { return true }) {
			if contains(w, "no edge found") {
				found = true
			}
		}
	}
	assert.True(t, found, "expected warning about missing edge")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidateRelationshipMissingProperty(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
			{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}},
		},
		Edges: []Edge{
			{Source: "api", Target: "db", Operation: "read", Description: "test", Properties: map[string]any{}},
		},
		Relationships: []Relationship{
			{Template: "capacity_chain", Upstream: "api", Downstream: "db"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	// max_connections missing on both → known unknowns generated
	var foundKU bool
	for _, ku := range result.KnownUnknowns {
		if ku.Component == "api" || ku.Component == "db" {
			foundKU = true
		}
	}
	assert.True(t, foundKU, "expected known unknowns for missing properties")
}

func TestValidateRelationshipMismatchedSchema(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
			{Name: "q", Type: "queue", Description: "Queue", Properties: map[string]any{}},
		},
		Edges: []Edge{
			{Source: "api", Target: "q", Operation: "write", Description: "test", Properties: map[string]any{}},
		},
		Relationships: []Relationship{
			{Template: "pooled_capacity_chain", Upstream: "api", Downstream: "q"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	// queue doesn't have conn_establish_ms → warning about property not in schema
	var found bool
	for _, w := range result.Warnings {
		if contains(w, "conn_establish_ms") && contains(w, "not defined in") {
			found = true
		}
	}
	assert.True(t, found, "expected warning about conn_establish_ms not in queue schema")
}

// --- Step 4.5: Merge known unknowns ---

func TestValidateMergesModelKnownUnknowns(t *testing.T) {
	m := &SystemModel{
		Components: []Component{
			{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
		},
		KnownUnknowns: []KnownUnknown{
			{ID: "explicit-ku", Category: "assumed_context", Description: "Explicit KU", Impact: "Some impact"},
		},
	}
	reg := buildDefaultRegistry(t)
	result, err := Validate(m, reg)
	require.NoError(t, err)

	// Should have both the explicit KU and validator-generated ones
	var foundExplicit bool
	var foundGenerated bool
	for _, ku := range result.KnownUnknowns {
		if ku.ID == "explicit-ku" {
			foundExplicit = true
		}
		if ku.Component == "api" {
			foundGenerated = true
		}
	}
	assert.True(t, foundExplicit, "expected explicit known unknown to be preserved")
	assert.True(t, foundGenerated, "expected generated known unknowns")
}
