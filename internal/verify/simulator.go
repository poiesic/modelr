package verify

// StepWidth is the fixed number of bytes consumed per simulation loop iteration.
// Every iteration consumes exactly StepWidth bytes from the bytestream, regardless
// of whether a rule fires. This enables step-aligned shrinking.
const StepWidth = 2

// PrefixWidth is the number of bytes consumed before the simulation loop.
// Currently: 1 byte for instance count selection + 1 byte padding for alignment.
const PrefixWidth = StepWidth

// SimulationConfig controls a single simulation run.
type SimulationConfig struct {
	MaxSteps int
}

// SimulationOutcome holds the result of one simulation.
type SimulationOutcome struct {
	Violated      bool
	Invariant     string // which invariant failed
	Steps         []Step // full trace up to failure
	FailedAt      int    // step index where violation occurred (-1 if pass)
	InstanceCount int    // actual instance count used in this simulation
}

// Simulate runs a single simulation of the given machine driven by the bytestream.
func Simulate(machine *Machine, bs *Bytestream, config SimulationConfig) *SimulationOutcome {
	maxSteps := config.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 1000
	}

	// Draw instance count from prefix bytes (PrefixWidth = StepWidth for alignment)
	minInst := machine.Roles.EffectiveMinInstances()
	maxInst := machine.Roles.MaxInstances
	b := bs.DrawByte()
	_ = bs.DrawByte() // pad to PrefixWidth
	instanceCount := int(b)%(maxInst-minInst+1) + minInst

	// Initialize state with the drawn instance count
	roles := *machine.Roles
	roles.MaxInstances = instanceCount
	state := machine.InitState(&roles)

	var steps []Step

	for step := 0; step < maxSteps; step++ {
		// Consume exactly StepWidth bytes unconditionally.
		// Byte 0: instance selection. Byte 1: rule selection.
		b0 := bs.DrawByte()
		b1 := bs.DrawByte()

		instance := int(b0) % instanceCount

		// Collect fireable rules
		var fireable []int
		for i, rule := range machine.Rules {
			if rule.CanFire(state, instance) {
				fireable = append(fireable, i)
			}
		}

		if len(fireable) == 0 {
			// No rules can fire — b1 is discarded, skip step
			continue
		}

		// Select a rule from the fireable set
		ruleIdx := fireable[int(b1)%len(fireable)]
		rule := machine.Rules[ruleIdx]

		// Apply the rule
		rule.Apply(state, instance)

		steps = append(steps, Step{
			Rule:     rule.Name,
			Instance: instance,
			State:    state.Snapshot(instance),
		})

		// Check invariants
		for _, inv := range machine.Invariants {
			if inv.PerInstance {
				for i := 0; i < len(state.PerInstance); i++ {
					if !inv.Check(state, i) {
						return &SimulationOutcome{
							Violated:      true,
							Invariant:     inv.Name,
							Steps:         steps,
							FailedAt:      len(steps) - 1,
							InstanceCount: instanceCount,
						}
					}
				}
			} else {
				if !inv.Check(state, -1) {
					return &SimulationOutcome{
						Violated:      true,
						Invariant:     inv.Name,
						Steps:         steps,
						FailedAt:      len(steps) - 1,
						InstanceCount: instanceCount,
					}
				}
			}
		}
	}

	return &SimulationOutcome{
		Violated:      false,
		Steps:         steps,
		FailedAt:      -1,
		InstanceCount: instanceCount,
	}
}
