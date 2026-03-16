package verify

import (
	"fmt"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
)

// ComposedSimulation holds multiple machines that may share state.
type ComposedSimulation struct {
	Machines []*Machine
}

// Compose builds machines from all patterned relationships in the model.
// Relationships sharing a downstream component share the same resource counter.
func Compose(m *model.SystemModel, registry *loader.Registry) (*ComposedSimulation, []string, error) {
	var machines []*Machine
	var warnings []string

	// Track shared state by downstream component name
	sharedStates := make(map[string]*SharedState)

	for _, rel := range m.Relationships {
		tmpl, ok := registry.LookupRelationship(rel.Template)
		if !ok {
			continue
		}

		if tmpl.Pattern == "" {
			continue // no behavioral pattern
		}

		if tmpl.Pattern != PatternFiniteResource && tmpl.Pattern != PatternFinitePooledResource {
			warnings = append(warnings, fmt.Sprintf("unknown pattern %q on template %q, skipping", tmpl.Pattern, tmpl.Name))
			continue
		}

		roles, err := InferRoles(tmpl, rel, m)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("cannot verify %s→%s: %v", rel.Upstream, rel.Downstream, err))
			continue
		}

		var machine *Machine
		switch tmpl.Pattern {
		case PatternFiniteResource:
			machine = NewFiniteResourceMachine(roles)
		case PatternFinitePooledResource:
			machine = NewFinitePooledResourceMachine(roles)
		}

		machine.Upstream = rel.Upstream
		machine.Downstream = rel.Downstream

		// Share state for same downstream
		if shared, ok := sharedStates[rel.Downstream]; ok {
			// Replace the machine's init to use the shared state
			origInit := machine.InitState
			machine.InitState = func(r *RoleMap) *State {
				state := origInit(r)
				state.Shared = *shared
				return state
			}
		} else {
			// First machine for this downstream — it creates the shared state
			origInit := machine.InitState
			machine.InitState = func(r *RoleMap) *State {
				state := origInit(r)
				sharedStates[rel.Downstream] = &state.Shared
				return state
			}
		}

		machines = append(machines, machine)
	}

	return &ComposedSimulation{Machines: machines}, warnings, nil
}
