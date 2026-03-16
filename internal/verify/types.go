package verify

import "github.com/poiesic/modelr/internal/model"

// Pattern names
const (
	PatternFiniteResource       = "finite_resource"
	PatternFinitePooledResource = "finite_pooled_resource"
)

// RoleMap holds resolved numeric values for pattern roles.
type RoleMap struct {
	Pattern          string
	MinInstances     int // 0 means use 1
	MaxInstances     int
	ResourceCapacity int
	OperationTime    int // milliseconds
	PoolCapacity     int // finite_pooled_resource only
	AcquireTime      int // finite_pooled_resource only
}

// EffectiveMinInstances returns MinInstances, defaulting to 1 if unset.
func (r *RoleMap) EffectiveMinInstances() int {
	if r.MinInstances <= 0 {
		return 1
	}
	return r.MinInstances
}

// Valid returns true if all required roles for the pattern are populated.
func (r *RoleMap) Valid() bool {
	return len(r.Missing()) == 0
}

// Missing returns the names of required roles that are not set (zero value).
func (r *RoleMap) Missing() []string {
	var missing []string
	if r.MaxInstances <= 0 {
		missing = append(missing, "instances")
	}
	if r.ResourceCapacity <= 0 {
		missing = append(missing, "resource_capacity")
	}
	if r.Pattern == PatternFinitePooledResource {
		if r.PoolCapacity <= 0 {
			missing = append(missing, "pool_capacity")
		}
		if r.AcquireTime <= 0 {
			missing = append(missing, "acquire_time")
		}
	}
	return missing
}

// Step records one event in a simulation trace.
type Step struct {
	Rule     string         `yaml:"rule"`
	Instance int            `yaml:"instance"`
	State    map[string]int `yaml:"state,omitempty"`
}

// VerificationRoles captures the resolved role mapping for output context.
type VerificationRoles struct {
	MinInstances     int `yaml:"min_instances"`
	MaxInstances     int `yaml:"max_instances"`
	Instances        int `yaml:"instances"`         // actual count used in minimal failure
	ResourceCapacity int `yaml:"resource_capacity"`
	PoolCapacity     int `yaml:"pool_capacity,omitempty"`
	AcquireTime      int `yaml:"acquire_time,omitempty"`
	OperationTime    int `yaml:"operation_time,omitempty"`
}

// Trigger captures the single event that violated an invariant.
type Trigger struct {
	Rule          string `yaml:"rule"`
	Instance      int    `yaml:"instance"`
	Resource      string `yaml:"resource"`
	ValueBefore   int    `yaml:"value_before"`
	Limit         int    `yaml:"limit"`
	ValueAfter    int    `yaml:"value_after"`
}

// Verification holds the result of verifying one relationship.
type Verification struct {
	Upstream          string             `yaml:"upstream"`
	Downstream        string             `yaml:"downstream"`
	Pattern           string             `yaml:"pattern"`
	Roles             VerificationRoles  `yaml:"roles"`
	Result            string             `yaml:"result"` // "pass" or "fail"
	Simulations       int                `yaml:"simulations"`
	Failures          int                `yaml:"failures"`
	Confidence        float64            `yaml:"confidence"`
	FailureRateBound  float64            `yaml:"failure_rate_bound"`
	Trigger           *Trigger           `yaml:"trigger,omitempty"`
	MinimalFailure    []Step             `yaml:"minimal_failure,omitempty"`
	ViolatedInvariant string             `yaml:"violated_invariant,omitempty"`
	Assumptions       []model.Assumption `yaml:"assumptions,omitempty"`
}

// VerificationResult holds all verification outcomes for a model.
type VerificationResult struct {
	Verifications []Verification
	Summary       string
}

// VerifyConfig controls verification behavior.
type VerifyConfig struct {
	TargetFailureRate float64
	Confidence        float64
	MaxStepsPerSim    int
	ShrinkAttempts    int
	MaxSimulations    int
	Seed              int64 // 0 = random
	OnShrinkProgress  func(upstream, downstream string, p ShrinkProgress) // nil = silent
}

// DefaultVerifyConfig returns sensible defaults per spec section 8.7.
func DefaultVerifyConfig() VerifyConfig {
	return VerifyConfig{
		TargetFailureRate: 0.001,
		Confidence:        0.99,
		MaxStepsPerSim:    1000,
		ShrinkAttempts:    2000,
		MaxSimulations:    10000,
	}
}
