package verify

// NewFinitePooledResourceMachine creates a state machine for the finite_pooled_resource pattern.
// Four rules, two invariants.
func NewFinitePooledResourceMachine(roles *RoleMap) *Machine {
	return &Machine{
		Roles: roles,
		InitState: func(r *RoleMap) *State {
			s := &State{
				PerInstance: make([]InstanceState, r.MaxInstances),
				Shared:      SharedState{Values: map[string]int{"used_connections": 0}},
			}
			for i := range s.PerInstance {
				s.PerInstance[i] = InstanceState{Values: map[string]int{
					"pool":      0,
					"in_flight": 0,
					"pending":   0,
				}}
			}
			return s
		},
		Rules: []Rule{
			{
				Name: "RequestArrives",
				CanFire: func(state *State, instance int) bool {
					return true
				},
				Apply: func(state *State, instance int) {
					state.PerInstance[instance].Values["pending"]++
				},
			},
			{
				Name: "StartGrowth",
				CanFire: func(state *State, instance int) bool {
					inst := state.PerInstance[instance].Values
					return inst["pending"] > 0 &&
						inst["pool"]+inst["in_flight"] < roles.PoolCapacity
				},
				Apply: func(state *State, instance int) {
					state.PerInstance[instance].Values["pending"]--
					state.PerInstance[instance].Values["in_flight"]++
					state.Shared.Values["used_connections"]++
				},
			},
			{
				Name: "GrowComplete",
				CanFire: func(state *State, instance int) bool {
					return state.PerInstance[instance].Values["in_flight"] > 0
				},
				Apply: func(state *State, instance int) {
					state.PerInstance[instance].Values["in_flight"]--
					state.PerInstance[instance].Values["pool"]++
				},
			},
			{
				Name: "RequestCompletes",
				CanFire: func(state *State, instance int) bool {
					return state.PerInstance[instance].Values["pool"] > 0
				},
				Apply: func(state *State, instance int) {
					state.PerInstance[instance].Values["pool"]--
					state.Shared.Values["used_connections"]--
				},
			},
		},
		Invariants: []Invariant{
			{
				Name:        "Conservation",
				Description: "Total used connections never exceed resource capacity",
				PerInstance: false,
				Check: func(state *State, instance int) bool {
					return state.Shared.Values["used_connections"] <= roles.ResourceCapacity
				},
			},
			{
				Name:        "PoolBounded",
				Description: "Per-instance pool never exceeds configured max",
				PerInstance: true,
				Check: func(state *State, instance int) bool {
					inst := state.PerInstance[instance].Values
					return inst["pool"]+inst["in_flight"] <= roles.PoolCapacity
				},
			},
		},
	}
}
