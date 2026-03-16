package model

// UncertaintyCategory represents the category of a known unknown.
type UncertaintyCategory string

const (
	CategoryUnstatedConstraint UncertaintyCategory = "unstated_constraint"
	CategoryUnresolvedTradeoff UncertaintyCategory = "unresolved_tradeoff"
	CategoryUndefinedBoundary  UncertaintyCategory = "undefined_boundary"
	CategoryAssumedContext     UncertaintyCategory = "assumed_context"
	CategoryDeferredDecision   UncertaintyCategory = "deferred_decision"
	CategoryUnknownUnknown     UncertaintyCategory = "unknown_unknown"
)

var validCategories = map[UncertaintyCategory]bool{
	CategoryUnstatedConstraint: true,
	CategoryUnresolvedTradeoff: true,
	CategoryUndefinedBoundary:  true,
	CategoryAssumedContext:     true,
	CategoryDeferredDecision:   true,
	CategoryUnknownUnknown:     true,
}

// IsValidUncertaintyCategory returns true if the category is one of the six defined categories.
func IsValidUncertaintyCategory(c string) bool {
	return validCategories[UncertaintyCategory(c)]
}

// Operation represents an edge operation type.
type Operation string

const (
	OpRead      Operation = "read"
	OpWrite     Operation = "write"
	OpReadWrite Operation = "read_write"
)

var validOperations = map[Operation]bool{
	OpRead:      true,
	OpWrite:     true,
	OpReadWrite: true,
}

// IsValidOperation returns true if the operation is read, write, or read_write.
func IsValidOperation(op string) bool {
	return validOperations[Operation(op)]
}

// SystemModel is the top-level model parsed from a .model.yaml file.
type SystemModel struct {
	Version       string         `yaml:"version"`
	Name          string         `yaml:"name"`
	Description   string         `yaml:"description"`
	Components    []Component    `yaml:"components"`
	Edges         []Edge         `yaml:"edges"`
	Relationships []Relationship `yaml:"relationships"`
	KnownUnknowns []KnownUnknown `yaml:"known_unknowns"`
}

// Component represents a system component.
type Component struct {
	Name        string         `yaml:"name"`
	Type        string         `yaml:"type"`
	Description string         `yaml:"description"`
	Properties  map[string]any `yaml:"properties"`
}

// Edge represents a connection between two components.
type Edge struct {
	Source      string         `yaml:"source"`
	Target      string         `yaml:"target"`
	Operation   string         `yaml:"operation"`
	Description string         `yaml:"description"`
	Properties  map[string]any `yaml:"properties"`
}

// Relationship references a relationship template between two components.
type Relationship struct {
	Template   string `yaml:"template"`
	Upstream   string `yaml:"upstream"`
	Downstream string `yaml:"downstream"`
}

// KnownUnknown represents an explicitly documented uncertainty.
type KnownUnknown struct {
	ID          string `yaml:"id"`
	Component   string `yaml:"component,omitempty"`
	Edge        string `yaml:"edge,omitempty"`
	Category    string `yaml:"category"`
	Description string `yaml:"description"`
	Impact      string `yaml:"impact"`
}

// Assumption records that a property value was filled from a default.
type Assumption struct {
	Property  string `yaml:"property"`
	Component string `yaml:"component,omitempty"`
	Edge      string `yaml:"edge,omitempty"`
	Value     any    `yaml:"value"`
	Source    string `yaml:"source"`
}

// Finding represents a constraint violation.
type Finding struct {
	Severity     string `yaml:"severity"`
	Relationship string `yaml:"relationship"`
	Upstream     string `yaml:"upstream"`
	Downstream   string `yaml:"downstream"`
	Description  string `yaml:"description"`
	Suggestion   string `yaml:"suggestion"`
	Kind         string `yaml:"kind,omitempty"`
}

// CheckResult contains the output of constraint checking.
type CheckResult struct {
	Findings      []Finding      `yaml:"findings"`
	KnownUnknowns []KnownUnknown `yaml:"known_unknowns"`
	Assumptions   []Assumption   `yaml:"assumptions"`
	Warnings      []string       `yaml:"warnings"`
	Summary       string         `yaml:"summary"`
}
