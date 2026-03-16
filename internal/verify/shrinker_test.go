package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShrinkReducesLength(t *testing.T) {
	// 10 instances, capacity 2 — very easy to violate
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 500}

	outcome := Simulate(m, bs, simCfg)
	require.True(t, outcome.Violated)
	original := bs.Bytes()

	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 500})
	assert.Less(t, len(shrunk), len(original))
}

func TestShrinkPreservesFailure(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 500}

	Simulate(m, bs, simCfg)
	original := bs.Bytes()

	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 500})

	// Verify the shrunk version still fails
	replayBs := FromBytes(shrunk)
	outcome := Simulate(m, replayBs, simCfg)
	assert.True(t, outcome.Violated)
}

func TestShrinkMinimal(t *testing.T) {
	// 2 instances, capacity 1 — minimal failure is just 2 RequestArrives
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 1))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 200}

	outcome := Simulate(m, bs, simCfg)
	require.True(t, outcome.Violated)
	original := bs.Bytes()

	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 1000})

	// Shrunk should be quite short
	assert.Less(t, len(shrunk), len(original))

	// And still fails
	replayBs := FromBytes(shrunk)
	replayOutcome := Simulate(m, replayBs, simCfg)
	assert.True(t, replayOutcome.Violated)
}

func TestShrinkMaxAttempts(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 100}

	Simulate(m, bs, simCfg)
	original := bs.Bytes()

	// Very low max attempts — should still terminate
	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 5})
	assert.NotNil(t, shrunk)
}

func TestShrinkProgressCallback(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 500}

	outcome := Simulate(m, bs, simCfg)
	require.True(t, outcome.Violated)
	original := bs.Bytes()

	var progress []ShrinkProgress
	shrinkCfg := ShrinkConfig{
		MaxAttempts: 500,
		OnProgress: func(p ShrinkProgress) {
			progress = append(progress, p)
		},
	}

	Shrink(m, original, simCfg, shrinkCfg)

	// Should have at least 3 phase-entry events
	require.GreaterOrEqual(t, len(progress), 3)

	// First event is phase entry for delete_chunks
	assert.Equal(t, "delete_chunks", progress[0].Phase)
	assert.False(t, progress[0].Improved)

	// All events have correct MaxAttempts
	for _, p := range progress {
		assert.Equal(t, 500, p.MaxAttempts)
	}

	// Phase entries appear in order within each pass
	phases := []string{}
	for _, p := range progress {
		if !p.Improved && (len(phases) == 0 || phases[len(phases)-1] != p.Phase) {
			phases = append(phases, p.Phase)
		}
	}
	// At least one full pass of all three phases
	require.GreaterOrEqual(t, len(phases), 3)
	assert.Equal(t, "delete_chunks", phases[0])
	assert.Equal(t, "zero_chunks", phases[1])
	assert.Equal(t, "reduce_bytes", phases[2])

	// Improvements should have positive BestSteps
	for _, p := range progress {
		if p.Improved {
			assert.Greater(t, p.BestSteps, 0)
			assert.Greater(t, p.BestLength, 0)
		}
	}
}

func TestShrinkEffectiveWithManyInstances(t *testing.T) {
	// 50 instances, pool 10, capacity 50 — mirrors chat-server ws->postgres
	m := NewFinitePooledResourceMachine(pooledRoles(50, 10, 50))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 500}

	outcome := Simulate(m, bs, simCfg)
	require.True(t, outcome.Violated)
	original := bs.Bytes()
	originalSteps := outcome.FailedAt + 1

	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 1000})

	// Shrunk bytestream should be shorter
	assert.Less(t, len(shrunk), len(original))

	// Verify shrinking actually reduced steps, not just bytes
	replayBs := FromBytes(shrunk)
	replayOutcome := Simulate(m, replayBs, simCfg)
	require.True(t, replayOutcome.Violated)
	shrunkSteps := replayOutcome.FailedAt + 1
	assert.Less(t, shrunkSteps, originalSteps,
		"shrinker should reduce step count, not just byte count")
}

func TestShrunkBytesStepAligned(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 200}

	outcome := Simulate(m, bs, simCfg)
	require.True(t, outcome.Violated)
	original := bs.Bytes()

	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 500})

	assert.Equal(t, 0, len(shrunk)%StepWidth,
		"shrunk bytes should be step-aligned (multiple of %d)", StepWidth)
}

func TestShrinkNilCallbackSilent(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	simCfg := SimulationConfig{MaxSteps: 500}

	Simulate(m, bs, simCfg)
	original := bs.Bytes()

	// nil OnProgress should not panic
	shrunk := Shrink(m, original, simCfg, ShrinkConfig{MaxAttempts: 500, OnProgress: nil})
	assert.Less(t, len(shrunk), len(original))
}
