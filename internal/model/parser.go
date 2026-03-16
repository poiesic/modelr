package model

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/poiesic/modelr/internal/loader"
	"gopkg.in/yaml.v3"
)

// ParseResult holds the model and any inline definitions found in the YAML file.
type ParseResult struct {
	Model      *SystemModel
	InlineNodes []loader.NodeDef
	InlineRels  []loader.RelationshipDef
}

// Parse reads a multi-document YAML stream and returns the model and inline definitions.
func Parse(reader io.Reader) (*ParseResult, error) {
	decoder := yaml.NewDecoder(reader)

	result := &ParseResult{}

	for {
		var raw map[string]any
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("YAML decode error: %w", err)
		}

		kind, _ := raw["kind"].(string)
		switch kind {
		case "node":
			var node loader.NodeDef
			if err := remarshal(raw, &node); err != nil {
				return nil, fmt.Errorf("parsing inline node definition: %w", err)
			}
			result.InlineNodes = append(result.InlineNodes, node)
		case "relationship":
			var rel loader.RelationshipDef
			if err := remarshal(raw, &rel); err != nil {
				return nil, fmt.Errorf("parsing inline relationship definition: %w", err)
			}
			result.InlineRels = append(result.InlineRels, rel)
		default:
			// Assume this is the model document
			if result.Model != nil {
				return nil, fmt.Errorf("multiple model documents found in file")
			}
			var model SystemModel
			if err := remarshal(raw, &model); err != nil {
				return nil, fmt.Errorf("parsing model: %w", err)
			}
			result.Model = &model
		}
	}

	if result.Model == nil {
		return nil, fmt.Errorf("no model document found in file")
	}

	if err := validateModel(result.Model); err != nil {
		return nil, err
	}

	return result, nil
}

func validateModel(m *SystemModel) error {
	var errs []string

	if m.Version == "" {
		errs = append(errs, "missing required field: version")
	}
	if m.Name == "" {
		errs = append(errs, "missing required field: name")
	}
	if m.Description == "" {
		errs = append(errs, "missing required field: description")
	}
	if m.Components == nil {
		errs = append(errs, "missing required field: components")
	} else if len(m.Components) == 0 {
		errs = append(errs, "components must not be empty")
	}
	if m.Edges == nil {
		errs = append(errs, "missing required field: edges")
	}

	// Validate individual components
	for i, c := range m.Components {
		if c.Name == "" {
			errs = append(errs, fmt.Sprintf("component[%d]: missing required field: name", i))
		}
		if c.Type == "" {
			errs = append(errs, fmt.Sprintf("component[%d]: missing required field: type", i))
		}
		if c.Description == "" {
			errs = append(errs, fmt.Sprintf("component[%d]: missing required field: description", i))
		}
	}

	// Validate individual edges
	for i, e := range m.Edges {
		if e.Source == "" {
			errs = append(errs, fmt.Sprintf("edge[%d]: missing required field: source", i))
		}
		if e.Target == "" {
			errs = append(errs, fmt.Sprintf("edge[%d]: missing required field: target", i))
		}
		if e.Operation == "" {
			errs = append(errs, fmt.Sprintf("edge[%d]: missing required field: operation", i))
		} else if !IsValidOperation(e.Operation) {
			errs = append(errs, fmt.Sprintf("edge[%d]: invalid operation %q (must be read, write, or read_write)", i, e.Operation))
		}
		if e.Description == "" {
			errs = append(errs, fmt.Sprintf("edge[%d]: missing required field: description", i))
		}
	}

	// Check duplicate component names
	names := make(map[string]bool)
	for _, c := range m.Components {
		if c.Name == "" {
			continue
		}
		if names[c.Name] {
			errs = append(errs, fmt.Sprintf("duplicate component name: %q", c.Name))
		}
		names[c.Name] = true
	}

	// Validate edge references and reject pool_size
	for i, e := range m.Edges {
		if e.Source != "" && !names[e.Source] {
			errs = append(errs, fmt.Sprintf("edge[%d]: unknown source component %q", i, e.Source))
		}
		if e.Target != "" && !names[e.Target] {
			errs = append(errs, fmt.Sprintf("edge[%d]: unknown target component %q", i, e.Target))
		}
		if _, ok := e.Properties["pool_size"]; ok {
			errs = append(errs, fmt.Sprintf("edge[%d]: pool_size is not supported; use min_pool_size and max_pool_size instead", i))
		}
	}

	// Validate relationship references
	for i, r := range m.Relationships {
		if r.Upstream != "" && !names[r.Upstream] {
			errs = append(errs, fmt.Sprintf("relationship[%d]: unknown upstream component %q", i, r.Upstream))
		}
		if r.Downstream != "" && !names[r.Downstream] {
			errs = append(errs, fmt.Sprintf("relationship[%d]: unknown downstream component %q", i, r.Downstream))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

func remarshal(raw map[string]any, target any) error {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}
