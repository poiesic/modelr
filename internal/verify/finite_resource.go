package verify

// NewFiniteResourceMachine creates a state machine for the finite_resource pattern.
// Two rules (RequestArrives, RequestCompletes), one invariant (Conservation).
func NewFiniteResourceMachine(roles *RoleMap) *Machine {
	return &Machine{
		Roles: roles,
		InitState: func(r *RoleMap) *State {
			s := &State{
				PerInstance: make([]InstanceState, r.MaxInstances),
				Shared:      SharedState{Values: map[string]int{"used_resources": 0}},
			}
			for i := range s.PerInstance {
				s.PerInstance[i] = InstanceState{Values: map[string]int{"active_requests": 0}}
			}
			return s
		},
		Rules: []Rule{
			{
				Name: "RequestArrives",
				CanFire: func(state *State, instance int) bool {
					return true // requests can always arrive
				},
				Apply: func(state *State, instance int) {
					state.PerInstance[instance].Values["active_requests"]++
					state.Shared.Values["used_resources"]++
				},
			},
			{
				Name: "RequestCompletes",
				CanFire: func(state *State, instance int) bool {
					return state.PerInstance[instance].Values["active_requests"] > 0
				},
				Apply: func(state *State, instance int) {
					state.PerInstance[instance].Values["active_requests"]--
					state.Shared.Values["used_resources"]--
				},
			},
		},
		Invariants: []Invariant{
			{
				Name:        "Conservation",
				Description: "Total used resources never exceed resource capacity",
				PerInstance: false,
				Check: func(state *State, instance int) bool {
					return state.Shared.Values["used_resources"] <= roles.ResourceCapacity
				},
			},
		},
	}
}
