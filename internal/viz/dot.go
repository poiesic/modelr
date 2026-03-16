package viz

import (
	"fmt"
	"strings"

	"github.com/poiesic/modelr/internal/model"
)

// DOTInput holds all data needed to generate a DOT visualization.
type DOTInput struct {
	Model            *model.SystemModel
	CheckResult      *model.CheckResult
	ValidationResult *model.ValidationResult
}

// GenerateDOT produces Graphviz DOT source from a checked model.
func GenerateDOT(input *DOTInput) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("digraph %q {\n", input.Model.Name))
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  fontname=\"Helvetica\";\n")
	b.WriteString("  node [fontname=\"Helvetica\", style=filled];\n")
	b.WriteString("  edge [fontname=\"Helvetica\"];\n\n")

	// Collect known unknowns by component
	kuComponents := make(map[string]bool)
	allKU := mergeKnownUnknowns(input)
	for _, ku := range allKU {
		if ku.Component != "" {
			kuComponents[ku.Component] = true
		}
	}

	// Nodes
	for _, c := range input.Model.Components {
		shape, fillColor := nodeStyle(c.Type)
		borderAttrs := ""
		if kuComponents[c.Name] {
			borderAttrs = fmt.Sprintf(", penwidth=2, color=%q", "#E6A23C")
		}

		label := buildNodeLabel(c)
		b.WriteString(fmt.Sprintf("  %q [shape=%s, fillcolor=%q, label=%s%s];\n",
			c.Name, shape, fillColor, label, borderAttrs))
	}

	b.WriteString("\n")

	// Edges
	findingPairs := buildFindingIndex(input.CheckResult)
	kuPairs := buildKUIndex(allKU)

	for _, e := range input.Model.Edges {
		pair := e.Source + "->" + e.Target
		style, color, penwidth := edgeStyle(findingPairs[pair], kuPairs[pair])
		label := buildEdgeLabel(e, findingPairs[pair])

		b.WriteString(fmt.Sprintf("  %q -> %q [label=%s, color=%q, penwidth=%.1f, style=%s];\n",
			e.Source, e.Target, label, color, penwidth, style))
	}

	b.WriteString("}\n")
	return b.String()
}

func nodeStyle(componentType string) (shape, fillColor string) {
	switch componentType {
	case "server":
		return "box", "#4A90D9"
	case "datastore":
		return "cylinder", "#50B83C"
	case "queue":
		return "parallelogram", "#E6A23C"
	default:
		return "box", "#999999"
	}
}

func buildNodeLabel(c model.Component) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("<b>%s</b>", c.Name))
	lines = append(lines, fmt.Sprintf("<i>%s</i>", c.Type))

	for k, v := range c.Properties {
		if v != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", k, v))
		}
	}

	return fmt.Sprintf("<%s>", strings.Join(lines, "<br/>"))
}

func buildEdgeLabel(e model.Edge, hasFinding bool) string {
	parts := []string{e.Operation}
	for k, v := range e.Properties {
		if v != nil {
			parts = append(parts, fmt.Sprintf("%s: %v", k, v))
		}
	}
	if hasFinding {
		parts = append(parts, "⚠")
	}
	return fmt.Sprintf("%q", strings.Join(parts, "\\n"))
}

func edgeStyle(hasFinding, hasKU bool) (style, color string, penwidth float64) {
	if hasFinding {
		return "solid", "red", 2.5
	}
	if hasKU {
		return "dashed", "orange", 1.5
	}
	return "solid", "black", 1.0
}

func buildFindingIndex(cr *model.CheckResult) map[string]bool {
	m := make(map[string]bool)
	if cr == nil {
		return m
	}
	for _, f := range cr.Findings {
		m[f.Upstream+"->"+f.Downstream] = true
	}
	return m
}

func buildKUIndex(kus []model.KnownUnknown) map[string]bool {
	m := make(map[string]bool)
	for _, ku := range kus {
		if ku.Edge != "" {
			m[ku.Edge] = true
		}
	}
	return m
}

func mergeKnownUnknowns(input *DOTInput) []model.KnownUnknown {
	var all []model.KnownUnknown
	if input.ValidationResult != nil {
		all = append(all, input.ValidationResult.KnownUnknowns...)
	}
	if input.CheckResult != nil {
		all = append(all, input.CheckResult.KnownUnknowns...)
	}
	return all
}
