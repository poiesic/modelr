package loader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPropertySchemaHasExpectedFields(t *testing.T) {
	ps := PropertySchema{
		Type:        "number",
		Unit:        "connections",
		Description: "Maximum concurrent connections",
		Default:     100,
	}
	assert.Equal(t, "number", ps.Type)
	assert.Equal(t, "connections", ps.Unit)
	assert.Equal(t, "Maximum concurrent connections", ps.Description)
	assert.Equal(t, 100, ps.Default)
}

func TestLoadEmbeddedDefinitions(t *testing.T) {
	nodes, rels, err := LoadEmbedded()
	require.NoError(t, err)
	assert.Len(t, nodes, 3)
	assert.Len(t, rels, 2)

	nodeNames := make([]string, len(nodes))
	for i, n := range nodes {
		nodeNames[i] = n.Name
	}
	assert.Contains(t, nodeNames, "server")
	assert.Contains(t, nodeNames, "datastore")
	assert.Contains(t, nodeNames, "queue")

	relNames := make([]string, len(rels))
	for i, r := range rels {
		relNames[i] = r.Name
	}
	assert.Contains(t, relNames, "capacity_chain")
	assert.Contains(t, relNames, "pooled_capacity_chain")
}

func findNode(nodes []NodeDef, name string) *NodeDef {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}
	return nil
}

func findRel(rels []RelationshipDef, name string) *RelationshipDef {
	for i := range rels {
		if rels[i].Name == name {
			return &rels[i]
		}
	}
	return nil
}

func TestServerNodeDefProperties(t *testing.T) {
	nodes, _, err := LoadEmbedded()
	require.NoError(t, err)

	server := findNode(nodes, "server")
	require.NotNil(t, server)

	assert.Contains(t, server.Properties, "max_connections")
	assert.Nil(t, server.Properties["max_connections"].Default)

	assert.Contains(t, server.Properties, "min_instances")
	assert.Equal(t, 1, server.Properties["min_instances"].Default)

	assert.Contains(t, server.Properties, "max_instances")
	assert.Equal(t, 1, server.Properties["max_instances"].Default)
}

func TestDatastoreNodeDefProperties(t *testing.T) {
	nodes, _, err := LoadEmbedded()
	require.NoError(t, err)

	ds := findNode(nodes, "datastore")
	require.NotNil(t, ds)

	assert.Contains(t, ds.Properties, "max_connections")
	assert.Nil(t, ds.Properties["max_connections"].Default)

	assert.Contains(t, ds.Properties, "max_ops_per_sec")
	assert.Nil(t, ds.Properties["max_ops_per_sec"].Default)

	assert.Contains(t, ds.Properties, "conn_establish_ms")
	assert.Equal(t, 20, ds.Properties["conn_establish_ms"].Default)
}

func TestQueueNodeDefProperties(t *testing.T) {
	nodes, _, err := LoadEmbedded()
	require.NoError(t, err)

	q := findNode(nodes, "queue")
	require.NotNil(t, q)

	assert.Contains(t, q.Properties, "max_depth")
	assert.Nil(t, q.Properties["max_depth"].Default)

	assert.Contains(t, q.Properties, "max_throughput")
	assert.Nil(t, q.Properties["max_throughput"].Default)
}

func TestCapacityChainResolveBindings(t *testing.T) {
	_, rels, err := LoadEmbedded()
	require.NoError(t, err)

	cc := findRel(rels, "capacity_chain")
	require.NotNil(t, cc)

	expected := map[string]string{
		"upstream_rate":        "upstream.max_connections",
		"instances":            "upstream.max_instances",
		"operation_cost":       "edge.avg_operation_ms",
		"downstream_capacity":  "downstream.max_connections",
		"downstream_instances": "downstream.max_instances",
	}
	assert.Equal(t, expected, cc.Resolve)
}

func TestPooledCapacityChainResolveBindings(t *testing.T) {
	_, rels, err := LoadEmbedded()
	require.NoError(t, err)

	pcc := findRel(rels, "pooled_capacity_chain")
	require.NotNil(t, pcc)

	expected := map[string]string{
		"min_pool_size":       "edge.min_pool_size",
		"max_pool_size":       "edge.max_pool_size",
		"instances":           "upstream.max_instances",
		"operation_cost":      "edge.avg_operation_ms",
		"downstream_capacity": "downstream.max_connections",
		"downstream_max_ops":  "downstream.max_ops_per_sec",
		"conn_establish_ms":   "downstream.conn_establish_ms",
	}
	assert.Equal(t, expected, pcc.Resolve)
}

func TestPooledCapacityChainChecks(t *testing.T) {
	_, rels, err := LoadEmbedded()
	require.NoError(t, err)

	pcc := findRel(rels, "pooled_capacity_chain")
	require.NotNil(t, pcc)

	require.Len(t, pcc.Checks, 2)
	assert.Equal(t, "connection_limit", pcc.Checks[0].Name)
	assert.Equal(t, "max_pool_size * instances <= downstream_capacity", pcc.Checks[0].Expression)
	assert.Equal(t, "throughput", pcc.Checks[1].Name)
	assert.Equal(t, "max_pool_size * instances * 1000 / operation_cost <= downstream_max_ops", pcc.Checks[1].Expression)
}

// --- Registry tests (Step 3.1) ---

func TestRegistryLookupNode(t *testing.T) {
	reg := NewRegistry()
	reg.AddNode(NodeDef{Name: "server", Description: "A server"})

	def, ok := reg.LookupNode("server")
	assert.True(t, ok)
	assert.Equal(t, "server", def.Name)
}

func TestRegistryLookupRelationship(t *testing.T) {
	reg := NewRegistry()
	reg.AddRelationship(RelationshipDef{Name: "capacity_chain", Description: "Capacity check"})

	def, ok := reg.LookupRelationship("capacity_chain")
	assert.True(t, ok)
	assert.Equal(t, "capacity_chain", def.Name)
}

func TestRegistryNodeNotFound(t *testing.T) {
	reg := NewRegistry()
	_, ok := reg.LookupNode("missing")
	assert.False(t, ok)
}

func TestRegistryFirstWins(t *testing.T) {
	reg := NewRegistry()
	assert.True(t, reg.AddNode(NodeDef{Name: "server", Description: "first"}))
	assert.False(t, reg.AddNode(NodeDef{Name: "server", Description: "second"}))

	def, ok := reg.LookupNode("server")
	require.True(t, ok)
	assert.Equal(t, "first", def.Description)
}

// --- BuildRegistry tests (Step 3.2) ---

func TestBuildRegistryEmbeddedOnly(t *testing.T) {
	reg, err := BuildRegistry(DefinitionSources{})
	require.NoError(t, err)

	_, ok := reg.LookupNode("server")
	assert.True(t, ok)
	_, ok = reg.LookupNode("datastore")
	assert.True(t, ok)
	_, ok = reg.LookupNode("queue")
	assert.True(t, ok)

	_, ok = reg.LookupRelationship("capacity_chain")
	assert.True(t, ok)
	_, ok = reg.LookupRelationship("pooled_capacity_chain")
	assert.True(t, ok)
}

func TestBuildRegistryInlineShadowsEmbedded(t *testing.T) {
	inlineServer := NodeDef{
		Name:        "server",
		Description: "Custom server",
		Properties:  map[string]PropertySchema{"custom_prop": {Type: "number"}},
	}
	reg, err := BuildRegistry(DefinitionSources{InlineNodes: []NodeDef{inlineServer}})
	require.NoError(t, err)

	def, ok := reg.LookupNode("server")
	require.True(t, ok)
	assert.Equal(t, "Custom server", def.Description)
	assert.Contains(t, def.Properties, "custom_prop")
}

func TestBuildRegistryInlineRelShadowsEmbedded(t *testing.T) {
	inlineRel := RelationshipDef{
		Name:        "capacity_chain",
		Description: "Custom capacity chain",
	}
	reg, err := BuildRegistry(DefinitionSources{InlineRels: []RelationshipDef{inlineRel}})
	require.NoError(t, err)

	def, ok := reg.LookupRelationship("capacity_chain")
	require.True(t, ok)
	assert.Equal(t, "Custom capacity chain", def.Description)
}

// --- Shadow event tests (Step 2.2) ---

func TestRegistryShadowOnDuplicate(t *testing.T) {
	reg := NewRegistry()
	reg.AddNode(NodeDef{Name: "server", Source: "inline"})
	reg.AddNode(NodeDef{Name: "server", Source: "embedded"})

	shadows := reg.Shadows()
	require.Len(t, shadows, 1)
	assert.Equal(t, "node", shadows[0].Kind)
	assert.Equal(t, "server", shadows[0].Name)
	assert.Equal(t, "inline", shadows[0].Winner)
	assert.Equal(t, "embedded", shadows[0].Shadowed)
}

func TestRegistryNoShadowForNewDef(t *testing.T) {
	reg := NewRegistry()
	reg.AddNode(NodeDef{Name: "server", Source: "embedded"})
	reg.AddNode(NodeDef{Name: "datastore", Source: "embedded"})

	assert.Empty(t, reg.Shadows())
}

func TestRegistryRelShadow(t *testing.T) {
	reg := NewRegistry()
	reg.AddRelationship(RelationshipDef{Name: "capacity_chain", Source: "path"})
	reg.AddRelationship(RelationshipDef{Name: "capacity_chain", Source: "embedded"})

	shadows := reg.Shadows()
	require.Len(t, shadows, 1)
	assert.Equal(t, "relationship", shadows[0].Kind)
	assert.Equal(t, "path", shadows[0].Winner)
	assert.Equal(t, "embedded", shadows[0].Shadowed)
}

func TestRegistryMultipleShadows(t *testing.T) {
	reg := NewRegistry()
	reg.AddNode(NodeDef{Name: "server", Source: "inline"})
	reg.AddNode(NodeDef{Name: "server", Source: "path"})
	reg.AddNode(NodeDef{Name: "server", Source: "embedded"})

	assert.Len(t, reg.Shadows(), 2)
}

// --- BuildRegistry extended tests (Step 2.3) ---

func TestBuildRegistryWithPathDefs(t *testing.T) {
	reg, err := BuildRegistry(DefinitionSources{
		InlineNodes: []NodeDef{{Name: "custom", Description: "custom", Source: "inline"}},
		PathNodes:   []NodeDef{{Name: "server", Description: "path server", Source: "/path/server.yaml"}},
	})
	require.NoError(t, err)

	// Path "server" wins over embedded
	def, ok := reg.LookupNode("server")
	require.True(t, ok)
	assert.Equal(t, "path server", def.Description)

	// Inline "custom" is present
	_, ok = reg.LookupNode("custom")
	assert.True(t, ok)

	// Embedded non-shadowed defs present
	_, ok = reg.LookupNode("datastore")
	assert.True(t, ok)
	_, ok = reg.LookupNode("queue")
	assert.True(t, ok)
}

func TestBuildRegistryPathShadowsEmbedded(t *testing.T) {
	reg, err := BuildRegistry(DefinitionSources{
		PathNodes: []NodeDef{{Name: "server", Description: "path server", Source: "/defs/server.yaml"}},
	})
	require.NoError(t, err)

	shadows := reg.Shadows()
	var found bool
	for _, s := range shadows {
		if s.Name == "server" && s.Winner == "/defs/server.yaml" && s.Shadowed == "embedded" {
			found = true
		}
	}
	assert.True(t, found, "expected shadow event for path→embedded")
}

func TestBuildRegistryInlineShadowsPath(t *testing.T) {
	reg, err := BuildRegistry(DefinitionSources{
		InlineNodes: []NodeDef{{Name: "server", Description: "inline"}},
		PathNodes:   []NodeDef{{Name: "server", Description: "path", Source: "/defs/server.yaml"}},
	})
	require.NoError(t, err)

	def, _ := reg.LookupNode("server")
	assert.Equal(t, "inline", def.Description)

	// Shadow: inline wins over path, and inline wins over embedded
	shadows := reg.Shadows()
	assert.GreaterOrEqual(t, len(shadows), 1)
}

func TestBuildRegistrySourcesCorrect(t *testing.T) {
	reg, err := BuildRegistry(DefinitionSources{
		InlineNodes: []NodeDef{{Name: "custom", Description: "inline custom"}},
		PathNodes:   []NodeDef{{Name: "pg", Description: "path pg", Source: "/defs/pg.yaml"}},
	})
	require.NoError(t, err)

	sources := reg.Sources()
	sourceMap := make(map[string]string)
	for _, s := range sources {
		sourceMap[s.Kind+"/"+s.Name] = s.Origin
	}
	assert.Equal(t, "inline", sourceMap["node/custom"])
	assert.Equal(t, "/defs/pg.yaml", sourceMap["node/pg"])
	assert.Equal(t, "embedded", sourceMap["node/server"])
}

func TestBuildRegistryEmptyPath(t *testing.T) {
	reg, err := BuildRegistry(DefinitionSources{
		InlineNodes: []NodeDef{{Name: "custom", Description: "inline"}},
	})
	require.NoError(t, err)

	// Same as iter 0 — inline + embedded
	_, ok := reg.LookupNode("custom")
	assert.True(t, ok)
	_, ok = reg.LookupNode("server")
	assert.True(t, ok)
}

// --- Source field tests (Step 2.1) ---

func TestNodeDefSourceNotInYAML(t *testing.T) {
	// yaml:"-" means Source won't appear in YAML output
	node := NodeDef{Name: "test", Source: "embedded"}
	data, err := yaml.Marshal(node)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "source")
	assert.NotContains(t, string(data), "embedded")
}

func TestNodeDefSourcePreservedInRegistry(t *testing.T) {
	reg := NewRegistry()
	reg.AddNode(NodeDef{Name: "server", Source: "embedded"})

	def, ok := reg.LookupNode("server")
	require.True(t, ok)
	assert.Equal(t, "embedded", def.Source)
}

func TestEmbeddedDefsHaveSourceSet(t *testing.T) {
	nodes, rels, err := LoadEmbedded()
	require.NoError(t, err)

	for _, n := range nodes {
		assert.Equal(t, "embedded", n.Source, "node %s missing source", n.Name)
	}
	for _, r := range rels {
		assert.Equal(t, "embedded", r.Source, "rel %s missing source", r.Name)
	}
}
