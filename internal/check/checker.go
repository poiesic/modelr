package check

import (
	"fmt"
	"strings"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
)

// Check evaluates all relationship constraints and returns findings and known unknowns.
func Check(m *model.SystemModel, registry *loader.Registry) (*model.CheckResult, error) {
	result := &model.CheckResult{}

	for _, rel := range m.Relationships {
		tmpl, ok := registry.LookupRelationship(rel.Template)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("unknown relationship template %q", rel.Template))
			continue
		}

		upstream := findComponent(m, rel.Upstream)
		downstream := findComponent(m, rel.Downstream)
		if upstream == nil || downstream == nil {
			continue
		}

		edge := findEdge(m, rel.Upstream, rel.Downstream)
		vars := resolveVariables(&tmpl, upstream, downstream, edge)

		// Skip if any variable resolved to a string (spec 4.4 step 3)
		if hasStringValue(vars) {
			continue
		}

		for _, chk := range tmpl.Checks {
			result, err := evaluateCheck(chk, vars, rel, tmpl, result)
			if err != nil {
				return result, err
			}
		}
	}

	if len(result.Findings) == 0 {
		result.Summary = "All relationship constraints satisfied."
	} else {
		result.Summary = fmt.Sprintf("Found %d relationship violation(s).", len(result.Findings))
	}

	return result, nil
}

func evaluateCheck(chk loader.CheckDef, vars map[string]any, rel model.Relationship, tmpl loader.RelationshipDef, result *model.CheckResult) (*model.CheckResult, error) {
	boolResult, err := EvaluateComparison(chk.Expression, vars)
	if err != nil {
		return result, fmt.Errorf("evaluating check %q in %q: %w", chk.Name, tmpl.Name, err)
	}

	if boolResult == nil {
		// Indeterminate — generate known unknown
		result.KnownUnknowns = append(result.KnownUnknowns, model.KnownUnknown{
			ID:          fmt.Sprintf("indeterminate-%s-%s-%s-%s", rel.Template, chk.Name, rel.Upstream, rel.Downstream),
			Category:    string(model.CategoryUnstatedConstraint),
			Description: fmt.Sprintf("check %q in relationship %q between %q and %q is indeterminate (null values in expression)", chk.Name, rel.Template, rel.Upstream, rel.Downstream),
			Impact:      "Cannot verify this constraint without the missing values",
		})
	} else if !*boolResult {
		// Violation
		formatted := FormatExpressionWithValues(chk.Expression, vars)
		result.Findings = append(result.Findings, model.Finding{
			Severity:     "error",
			Relationship: rel.Template,
			Upstream:     rel.Upstream,
			Downstream:   rel.Downstream,
			Description:  fmt.Sprintf("check %q failed: %s (with values: %s)", chk.Name, chk.Expression, formatted),
			Suggestion:   chk.Violation,
			Kind:         "arithmetic",
		})
	}

	return result, nil
}

func resolveVariables(tmpl *loader.RelationshipDef, upstream, downstream *model.Component, edge *model.Edge) map[string]any {
	vars := make(map[string]any)

	for varName, binding := range tmpl.Resolve {
		parts := strings.SplitN(binding, ".", 2)
		if len(parts) != 2 {
			vars[varName] = nil
			continue
		}
		scope, propName := parts[0], parts[1]

		switch scope {
		case "upstream":
			vars[varName] = getProperty(upstream.Properties, propName)
		case "downstream":
			vars[varName] = getProperty(downstream.Properties, propName)
		case "edge":
			if edge == nil {
				vars[varName] = nil
			} else {
				vars[varName] = getProperty(edge.Properties, propName)
			}
		default:
			vars[varName] = nil
		}
	}

	// Normalize numeric types (YAML may parse as int)
	for k, v := range vars {
		if v == nil {
			continue
		}
		switch v := v.(type) {
		case int:
			vars[k] = float64(v)
		case int64:
			vars[k] = float64(v)
		case int32:
			vars[k] = float64(v)
		case float32:
			vars[k] = float64(v)
		}
	}

	return vars
}

func getProperty(props map[string]any, name string) any {
	if props == nil {
		return nil
	}
	v, ok := props[name]
	if !ok {
		return nil
	}
	return v
}

func hasStringValue(vars map[string]any) bool {
	for _, v := range vars {
		if _, ok := v.(string); ok {
			return true
		}
	}
	return false
}

func findComponent(m *model.SystemModel, name string) *model.Component {
	for i := range m.Components {
		if m.Components[i].Name == name {
			return &m.Components[i]
		}
	}
	return nil
}

func findEdge(m *model.SystemModel, source, target string) *model.Edge {
	for i := range m.Edges {
		if m.Edges[i].Source == source && m.Edges[i].Target == target {
			return &m.Edges[i]
		}
	}
	return nil
}
