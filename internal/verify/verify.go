package verify

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/poiesic/modelr/internal/loader"
	"github.com/poiesic/modelr/internal/model"
)

// Verify runs behavioral verification on all patterned relationships in the model.
func Verify(
	m *model.SystemModel,
	registry *loader.Registry,
	validation *model.ValidationResult,
	config VerifyConfig,
) (*VerificationResult, error) {
	composed, warnings, err := Compose(m, registry)
	if err != nil {
		return nil, err
	}

	if len(composed.Machines) == 0 {
		return &VerificationResult{
			Summary: "No behavioral patterns to verify.",
		}, nil
	}

	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	simConfig := SimulationConfig{MaxSteps: config.MaxStepsPerSim}
	if simConfig.MaxSteps <= 0 {
		simConfig.MaxSteps = 1000
	}

	var verifications []Verification

	for _, machine := range composed.Machines {
		v := verifyMachine(machine, rng, config, simConfig, validation)
		verifications = append(verifications, v)
	}

	// Build summary
	passed := 0
	failed := 0
	for _, v := range verifications {
		if v.Result == "pass" {
			passed++
		} else {
			failed++
		}
	}

	var summary string
	if failed > 0 {
		summary = fmt.Sprintf("Rejected: %d of %d verifications failed.", failed, len(verifications))
	} else {
		summary = fmt.Sprintf("Accepted: all %d verifications passed.", len(verifications))
	}

	_ = warnings // warnings already emitted by Compose

	return &VerificationResult{
		Verifications: verifications,
		Summary:       summary,
	}, nil
}

func verifyMachine(machine *Machine, rng *rand.Rand, config VerifyConfig, simConfig SimulationConfig, validation *model.ValidationResult) Verification {
	sprt := NewSPRT(SPRTConfig{
		TargetFailureRate: config.TargetFailureRate,
		Confidence:        config.Confidence,
	})

	maxSims := config.MaxSimulations
	if maxSims <= 0 {
		maxSims = 10000
	}

	var failingBytes []byte
	var failedInvariant string

	for i := 0; i < maxSims; i++ {
		bs := NewBytestream(rng.Int63())
		outcome := Simulate(machine, bs, simConfig)

		decision := sprt.Update(outcome.Violated)

		if outcome.Violated && failingBytes == nil {
			failingBytes = bs.Bytes()
			failedInvariant = outcome.Invariant
		}

		if decision == SPRTAccept || decision == SPRTReject {
			break
		}
	}

	v := Verification{
		Upstream:         machine.Upstream,
		Downstream:       machine.Downstream,
		Pattern:          machine.Roles.Pattern,
		Roles: VerificationRoles{
			MinInstances:     machine.Roles.EffectiveMinInstances(),
			MaxInstances:     machine.Roles.MaxInstances,
			Instances:        machine.Roles.MaxInstances, // overridden by shrunk value for failures
			ResourceCapacity: machine.Roles.ResourceCapacity,
			PoolCapacity:     machine.Roles.PoolCapacity,
			AcquireTime:      machine.Roles.AcquireTime,
			OperationTime:    machine.Roles.OperationTime,
		},
		Simulations:      sprt.Simulations(),
		Failures:         sprt.Failures(),
		Confidence:       config.Confidence,
		FailureRateBound: config.TargetFailureRate,
	}

	// Collect assumptions relevant to this relationship
	if validation != nil {
		for _, a := range validation.Assumptions {
			if a.Component == machine.Upstream || a.Component == machine.Downstream {
				v.Assumptions = append(v.Assumptions, a)
			}
		}
	}

	if sprt.Failures() > 0 && failingBytes != nil {
		v.Result = "fail"
		v.ViolatedInvariant = failedInvariant

		// Shrink
		shrinkCfg := ShrinkConfig{MaxAttempts: config.ShrinkAttempts}
		if config.OnShrinkProgress != nil {
			up, down := machine.Upstream, machine.Downstream
			shrinkCfg.OnProgress = func(p ShrinkProgress) {
				config.OnShrinkProgress(up, down, p)
			}
		}
		shrunk := Shrink(machine, failingBytes, simConfig, shrinkCfg)
		replayBs := FromBytes(shrunk)
		replayOutcome := Simulate(machine, replayBs, simConfig)
		if replayOutcome.Violated {
			steps := replayOutcome.Steps[:replayOutcome.FailedAt+1]
			v.MinimalFailure = steps
			v.Roles.Instances = replayOutcome.InstanceCount

			// Extract trigger: the final step that violated the invariant
			last := steps[len(steps)-1]
			resourceKey := sharedResourceKey(machine.Roles.Pattern)
			trigger := &Trigger{
				Rule:        last.Rule,
				Instance:    last.Instance,
				Resource:    resourceKey,
				Limit:       machine.Roles.ResourceCapacity,
				ValueAfter:  last.State[resourceKey],
			}
			if len(steps) >= 2 {
				trigger.ValueBefore = steps[len(steps)-2].State[resourceKey]
			}
			v.Trigger = trigger
		}
	} else {
		v.Result = "pass"
	}

	return v
}

func sharedResourceKey(pattern string) string {
	switch pattern {
	case PatternFinitePooledResource:
		return "used_connections"
	default:
		return "used_resources"
	}
}
