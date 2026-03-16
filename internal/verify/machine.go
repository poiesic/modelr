package verify

// InstanceState holds per-instance state variables.
type InstanceState struct {
	Values map[string]int
}

// SharedState holds state shared across all instances (e.g., downstream resource counters).
type SharedState struct {
	Values map[string]int
}

// State holds the complete simulation state.
type State struct {
	PerInstance []InstanceState
	Shared      SharedState
}

// Clone creates a deep copy of the state.
func (s *State) Clone() *State {
	clone := &State{
		PerInstance: make([]InstanceState, len(s.PerInstance)),
		Shared:      SharedState{Values: make(map[string]int)},
	}
	for i, inst := range s.PerInstance {
		clone.PerInstance[i] = InstanceState{Values: make(map[string]int)}
		for k, v := range inst.Values {
			clone.PerInstance[i].Values[k] = v
		}
	}
	for k, v := range s.Shared.Values {
		clone.Shared.Values[k] = v
	}
	return clone
}

// Snapshot returns a flat map of all state for a given instance (for trace output).
func (s *State) Snapshot(instance int) map[string]int {
	snap := make(map[string]int)
	if instance >= 0 && instance < len(s.PerInstance) {
		for k, v := range s.PerInstance[instance].Values {
			snap[k] = v
		}
	}
	for k, v := range s.Shared.Values {
		snap[k] = v
	}
	return snap
}


// Rule defines a state transition with an optional guard.
type Rule struct {
	Name    string
	CanFire func(state *State, instance int) bool
	Apply   func(state *State, instance int)
}

// Invariant defines a property that must hold after every step.
type Invariant struct {
	Name        string
	Description string
	Check       func(state *State, instance int) bool // instance used for per-instance invariants
	PerInstance bool                                   // true = check per instance, false = global
}

// Machine defines a behavioral pattern's state machine.
type Machine struct {
	Roles      *RoleMap
	Rules      []Rule
	Invariants []Invariant
	InitState  func(roles *RoleMap) *State

	// Metadata for output
	Upstream   string
	Downstream string
}
