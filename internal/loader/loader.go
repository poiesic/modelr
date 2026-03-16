package loader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/poiesic/modelr/embed"
	"gopkg.in/yaml.v3"
)

// LoadEmbedded parses the embedded definitions YAML and returns node and relationship definitions.
// All definitions have Source set to "embedded".
func LoadEmbedded() ([]NodeDef, []RelationshipDef, error) {
	nodes, rels, err := ParseDefinitions(embed.Definitions)
	if err != nil {
		return nil, nil, err
	}
	for i := range nodes {
		nodes[i].Source = "embedded"
	}
	for i := range rels {
		rels[i].Source = "embedded"
	}
	return nodes, rels, nil
}

// ParseDefinitions parses a multi-document YAML byte slice into node and relationship definitions.
func ParseDefinitions(data []byte) ([]NodeDef, []RelationshipDef, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	var nodes []NodeDef
	var rels []RelationshipDef

	for {
		var raw map[string]any
		err := decoder.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("parsing definitions: %w", err)
		}

		kind, _ := raw["kind"].(string)
		switch kind {
		case "node":
			var node NodeDef
			if err := remarshal(raw, &node); err != nil {
				return nil, nil, fmt.Errorf("parsing node definition: %w", err)
			}
			nodes = append(nodes, node)
		case "relationship":
			var rel RelationshipDef
			if err := remarshal(raw, &rel); err != nil {
				return nil, nil, fmt.Errorf("parsing relationship definition: %w", err)
			}
			rels = append(rels, rel)
		default:
			// Skip unknown kinds
		}
	}

	return nodes, rels, nil
}

// Registry holds indexed definitions for lookup by name.
type Registry struct {
	nodes   map[string]NodeDef
	rels    map[string]RelationshipDef
	shadows []ShadowEvent
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]NodeDef),
		rels:  make(map[string]RelationshipDef),
	}
}

// AddNode registers a node definition. Returns false if a definition with the same name already exists.
// When a duplicate is skipped, a ShadowEvent is recorded.
func (r *Registry) AddNode(def NodeDef) bool {
	if existing, exists := r.nodes[def.Name]; exists {
		r.shadows = append(r.shadows, ShadowEvent{
			Kind:     "node",
			Name:     def.Name,
			Winner:   existing.Source,
			Shadowed: def.Source,
		})
		return false
	}
	r.nodes[def.Name] = def
	return true
}

// AddRelationship registers a relationship definition. Returns false if already exists.
// When a duplicate is skipped, a ShadowEvent is recorded.
func (r *Registry) AddRelationship(def RelationshipDef) bool {
	if existing, exists := r.rels[def.Name]; exists {
		r.shadows = append(r.shadows, ShadowEvent{
			Kind:     "relationship",
			Name:     def.Name,
			Winner:   existing.Source,
			Shadowed: def.Source,
		})
		return false
	}
	r.rels[def.Name] = def
	return true
}

// Shadows returns all shadow events recorded during registry building.
func (r *Registry) Shadows() []ShadowEvent {
	return r.shadows
}

// LookupNode returns the node definition with the given name.
func (r *Registry) LookupNode(name string) (NodeDef, bool) {
	def, ok := r.nodes[name]
	return def, ok
}

// LookupRelationship returns the relationship definition with the given name.
func (r *Registry) LookupRelationship(name string) (RelationshipDef, bool) {
	def, ok := r.rels[name]
	return def, ok
}

// DefinitionSource describes a single definition and where it came from.
type DefinitionSource struct {
	Kind   string // "node" or "relationship"
	Name   string
	Origin string // "embedded", "inline", or file path
}

// Sources returns all definitions in the registry with their sources, sorted by kind then name.
func (r *Registry) Sources() []DefinitionSource {
	var sources []DefinitionSource
	for name, def := range r.nodes {
		sources = append(sources, DefinitionSource{Kind: "node", Name: name, Origin: def.Source})
	}
	for name, def := range r.rels {
		sources = append(sources, DefinitionSource{Kind: "relationship", Name: name, Origin: def.Source})
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Kind != sources[j].Kind {
			return sources[i].Kind < sources[j].Kind
		}
		return sources[i].Name < sources[j].Name
	})
	return sources
}

// BuildRegistry creates a registry from all definition sources.
// Resolution order: inline (highest priority) → path → embedded (lowest).
func BuildRegistry(sources DefinitionSources) (*Registry, error) {
	reg := NewRegistry()

	// Inline definitions first (highest priority)
	for _, n := range sources.InlineNodes {
		if n.Source == "" {
			n.Source = "inline"
		}
		reg.AddNode(n)
	}
	for _, r := range sources.InlineRels {
		if r.Source == "" {
			r.Source = "inline"
		}
		reg.AddRelationship(r)
	}

	// Path definitions (middle priority)
	for _, n := range sources.PathNodes {
		reg.AddNode(n)
	}
	for _, r := range sources.PathRels {
		reg.AddRelationship(r)
	}

	// Embedded defaults (lowest priority)
	nodes, rels, err := LoadEmbedded()
	if err != nil {
		return nil, fmt.Errorf("loading embedded definitions: %w", err)
	}
	for _, n := range nodes {
		reg.AddNode(n)
	}
	for _, r := range rels {
		reg.AddRelationship(r)
	}

	return reg, nil
}

// remarshal marshals the raw map back to YAML and then decodes it into the target struct.
func remarshal(raw map[string]any, target any) error {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, target)
}
