package model

import (
	"fmt"
	"strings"

	"github.com/poiesic/modelr/internal/loader"
)

// ValidationResult holds the output of model validation.
type ValidationResult struct {
	Model         *SystemModel
	Assumptions   []Assumption
	KnownUnknowns []KnownUnknown
	Warnings      []string
}

// Validate fills in defaults, generates assumptions and known unknowns, and validates relationship bindings.
func Validate(model *SystemModel, registry *loader.Registry) (*ValidationResult, error) {
	result := &ValidationResult{
		Model: model,
	}

	// Merge model-level known unknowns
	result.KnownUnknowns = append(result.KnownUnknowns, model.KnownUnknowns...)

	// Validate each component against its type schema
	for i := range model.Components {
		c := &model.Components[i]
		schema, ok := registry.LookupNode(c.Type)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("unknown component type %q for component %q", c.Type, c.Name))
			continue
		}

		if c.Properties == nil {
			c.Properties = make(map[string]any)
		}

		for propName, propSchema := range schema.Properties {
			if _, exists := c.Properties[propName]; exists {
				continue
			}

			if propSchema.Default != nil {
				c.Properties[propName] = propSchema.Default
				result.Assumptions = append(result.Assumptions, Assumption{
					Property:  propName,
					Component: c.Name,
					Value:     propSchema.Default,
					Source:    fmt.Sprintf("%s type schema", c.Type),
				})
			} else {
				c.Properties[propName] = nil
				result.KnownUnknowns = append(result.KnownUnknowns, KnownUnknown{
					ID:          fmt.Sprintf("missing-%s-%s", c.Name, propName),
					Component:   c.Name,
					Category:    string(CategoryUnstatedConstraint),
					Description: fmt.Sprintf("property %q not specified for component %q", propName, c.Name),
					Impact:      fmt.Sprintf("Cannot evaluate constraints that depend on %s.%s", c.Name, propName),
				})
			}
		}
	}

	// Validate relationship bindings
	for _, rel := range model.Relationships {
		tmpl, ok := registry.LookupRelationship(rel.Template)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("unknown relationship template %q", rel.Template))
			continue
		}

		upstream := findComponent(model, rel.Upstream)
		downstream := findComponent(model, rel.Downstream)
		edge := findEdge(model, rel.Upstream, rel.Downstream)

		// Check that bindings reference valid properties
		for varName, binding := range tmpl.Resolve {
			parts := strings.SplitN(binding, ".", 2)
			if len(parts) != 2 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("relationship %q: invalid binding %q for variable %q", rel.Template, binding, varName))
				continue
			}
			scope, propName := parts[0], parts[1]

			switch scope {
			case "upstream":
				if upstream != nil {
					validateBindingProperty(result, upstream, propName, rel.Template, registry)
				}
			case "downstream":
				if downstream != nil {
					validateBindingProperty(result, downstream, propName, rel.Template, registry)
				}
			case "edge":
				if edge == nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("relationship %q: no edge found between %q and %q", rel.Template, rel.Upstream, rel.Downstream))
				}
			}
		}
	}

	return result, nil
}

func validateBindingProperty(result *ValidationResult, comp *Component, propName string, relTemplate string, registry *loader.Registry) {
	// Check if the property exists in the component's type schema
	schema, ok := registry.LookupNode(comp.Type)
	if !ok {
		return // Unknown type, already warned
	}
	if _, exists := schema.Properties[propName]; !exists {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"relationship %q: property %q not defined in %s type schema for component %q",
			relTemplate, propName, comp.Type, comp.Name,
		))
	}
}

func findComponent(model *SystemModel, name string) *Component {
	for i := range model.Components {
		if model.Components[i].Name == name {
			return &model.Components[i]
		}
	}
	return nil
}

func findEdge(model *SystemModel, source, target string) *Edge {
	for i := range model.Edges {
		if model.Edges[i].Source == source && model.Edges[i].Target == target {
			return &model.Edges[i]
		}
	}
	return nil
}
