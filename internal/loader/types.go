package loader

// NodeDef defines a component type schema.
type NodeDef struct {
	Kind        string                    `yaml:"kind"`
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	Properties  map[string]PropertySchema `yaml:"properties"`
	Source      string                    `yaml:"-" json:"source,omitempty"`
}

// PropertySchema describes a single property within a type schema.
type PropertySchema struct {
	Type        string `yaml:"type"`
	Unit        string `yaml:"unit"`
	Description string `yaml:"description"`
	Default     any    `yaml:"default"`
}

// CheckDef defines a single check within a relationship template.
type CheckDef struct {
	Name       string `yaml:"name"`
	Expression string `yaml:"expression"`
	Violation  string `yaml:"violation"`
}

// RelationshipDef defines a relationship template with resolve bindings and checks.
type RelationshipDef struct {
	Kind        string            `yaml:"kind"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Pattern     string            `yaml:"pattern,omitempty"`
	Resolve     map[string]string `yaml:"resolve"`
	Checks      []CheckDef        `yaml:"checks"`
	Source      string            `yaml:"-" json:"source,omitempty"`
}

// ShadowEvent records when one definition shadows another during resolution.
type ShadowEvent struct {
	Kind     string // "node" or "relationship"
	Name     string
	Winner   string // source of the def that won
	Shadowed string // source of the def that was skipped
}

// DefinitionSources groups all definition sources for BuildRegistry.
type DefinitionSources struct {
	InlineNodes []NodeDef
	InlineRels  []RelationshipDef
	PathNodes   []NodeDef
	PathRels    []RelationshipDef
}
