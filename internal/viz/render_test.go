package viz

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphvizAvailable(t *testing.T) {
	// Just verify it returns a bool without panicking
	_ = GraphvizAvailable()
}

func TestRenderDOTToSVG(t *testing.T) {
	if !GraphvizAvailable() {
		t.Skip("graphviz not installed")
	}
	dot := `digraph "test" { "a" -> "b"; }`
	svg, err := RenderDOT(dot, "svg")
	require.NoError(t, err)
	assert.Contains(t, string(svg), "<svg")
}

func TestRenderDOTToSVGNoGraphviz(t *testing.T) {
	if GraphvizAvailable() {
		t.Skip("graphviz is installed, cannot test missing-graphviz path")
	}
	_, err := RenderDOT(`digraph "test" {}`, "svg")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "graphviz")
}
