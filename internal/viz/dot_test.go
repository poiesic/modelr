package viz

import (
	"strings"
	"testing"

	"github.com/poiesic/modelr/internal/model"
	"github.com/stretchr/testify/assert"
)

func minimalInput() *DOTInput {
	return &DOTInput{
		Model: &model.SystemModel{
			Name: "test-system",
			Components: []model.Component{
				{Name: "api", Type: "server", Description: "API", Properties: map[string]any{}},
			},
			Edges: []model.Edge{},
		},
		CheckResult:      &model.CheckResult{},
		ValidationResult: &model.ValidationResult{},
	}
}

// --- Step 1.1: Basic DOT structure ---

func TestDOTMinimalModel(t *testing.T) {
	dot := GenerateDOT(minimalInput())
	assert.Contains(t, dot, "digraph")
	assert.Contains(t, dot, "api")
	assert.Contains(t, dot, "rankdir=LR")
}

func TestDOTNodeHasLabel(t *testing.T) {
	dot := GenerateDOT(minimalInput())
	assert.Contains(t, dot, "<b>api</b>")
	assert.Contains(t, dot, "<i>server</i>")
}

func TestDOTOutputIsParseable(t *testing.T) {
	dot := GenerateDOT(minimalInput())
	assert.True(t, strings.HasPrefix(dot, "digraph"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(dot), "}"))
}

// --- Step 1.2: Node styling by type ---

func TestDOTServerNodeStyle(t *testing.T) {
	dot := GenerateDOT(minimalInput())
	assert.Contains(t, dot, "shape=box")
	assert.Contains(t, dot, `fillcolor="#4A90D9"`)
}

func TestDOTDatastoreNodeStyle(t *testing.T) {
	input := minimalInput()
	input.Model.Components = []model.Component{
		{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, "shape=cylinder")
	assert.Contains(t, dot, `fillcolor="#50B83C"`)
}

func TestDOTQueueNodeStyle(t *testing.T) {
	input := minimalInput()
	input.Model.Components = []model.Component{
		{Name: "q", Type: "queue", Description: "Queue", Properties: map[string]any{}},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, "shape=parallelogram")
	assert.Contains(t, dot, `fillcolor="#E6A23C"`)
}

func TestDOTUnknownTypeNodeStyle(t *testing.T) {
	input := minimalInput()
	input.Model.Components = []model.Component{
		{Name: "x", Type: "custom", Description: "Custom", Properties: map[string]any{}},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, "shape=box")
	assert.Contains(t, dot, `fillcolor="#999999"`)
}

func TestDOTNodeHasProperties(t *testing.T) {
	input := minimalInput()
	input.Model.Components[0].Properties = map[string]any{
		"max_connections": 100,
		"null_prop":       nil,
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, "max_connections: 100")
	assert.NotContains(t, dot, "null_prop")
}

// --- Step 1.3: Edge rendering ---

func TestDOTEdgeBasic(t *testing.T) {
	input := minimalInput()
	input.Model.Components = append(input.Model.Components,
		model.Component{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}})
	input.Model.Edges = []model.Edge{
		{Source: "api", Target: "db", Operation: "read_write", Description: "API to DB", Properties: map[string]any{}},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, `"api" -> "db"`)
	assert.Contains(t, dot, "read_write")
}

func TestDOTEdgeProperties(t *testing.T) {
	input := minimalInput()
	input.Model.Components = append(input.Model.Components,
		model.Component{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}})
	input.Model.Edges = []model.Edge{
		{Source: "api", Target: "db", Operation: "read", Description: "test", Properties: map[string]any{
			"avg_operation_ms": 10,
		}},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, "avg_operation_ms: 10")
}

func TestDOTEdgeClean(t *testing.T) {
	input := minimalInput()
	input.Model.Components = append(input.Model.Components,
		model.Component{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}})
	input.Model.Edges = []model.Edge{
		{Source: "api", Target: "db", Operation: "read", Description: "test", Properties: map[string]any{}},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, `color="black"`)
	assert.Contains(t, dot, "penwidth=1.0")
	assert.Contains(t, dot, "style=solid")
}

func TestDOTEdgeFinding(t *testing.T) {
	input := minimalInput()
	input.Model.Components = append(input.Model.Components,
		model.Component{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}})
	input.Model.Edges = []model.Edge{
		{Source: "api", Target: "db", Operation: "read", Description: "test", Properties: map[string]any{}},
	}
	input.CheckResult = &model.CheckResult{
		Findings: []model.Finding{
			{Upstream: "api", Downstream: "db", Severity: "error", Description: "overload"},
		},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, `color="red"`)
	assert.Contains(t, dot, "penwidth=2.5")
	assert.Contains(t, dot, "⚠")
}

func TestDOTEdgeKnownUnknown(t *testing.T) {
	input := minimalInput()
	input.Model.Components = append(input.Model.Components,
		model.Component{Name: "db", Type: "datastore", Description: "DB", Properties: map[string]any{}})
	input.Model.Edges = []model.Edge{
		{Source: "api", Target: "db", Operation: "read", Description: "test", Properties: map[string]any{}},
	}
	input.CheckResult = &model.CheckResult{
		KnownUnknowns: []model.KnownUnknown{
			{ID: "ku-1", Edge: "api->db", Category: "unstated_constraint", Description: "unknown"},
		},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, `color="orange"`)
	assert.Contains(t, dot, "penwidth=1.5")
	assert.Contains(t, dot, "style=dashed")
}

// --- Step 1.4: Component borders for known unknowns ---

func TestDOTComponentWithKnownUnknown(t *testing.T) {
	input := minimalInput()
	input.ValidationResult = &model.ValidationResult{
		KnownUnknowns: []model.KnownUnknown{
			{ID: "ku-1", Component: "api", Category: "unstated_constraint", Description: "missing prop"},
		},
	}
	dot := GenerateDOT(input)
	assert.Contains(t, dot, "penwidth=2")
	assert.Contains(t, dot, `color="#E6A23C"`)
}

func TestDOTComponentClean(t *testing.T) {
	dot := GenerateDOT(minimalInput())
	// No orange border on a clean component
	assert.NotContains(t, dot, `color="#E6A23C"`)
}
