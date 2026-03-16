package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimulatePassingModel(t *testing.T) {
	// Generous capacity — 3 instances, capacity 1000
	m := NewFiniteResourceMachine(finiteResourceRoles(3, 1000))
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 500})

	assert.False(t, outcome.Violated)
	assert.Equal(t, -1, outcome.FailedAt)
}

func TestSimulateFailingModel(t *testing.T) {
	// Very tight — 10 instances, capacity 2
	m := NewFiniteResourceMachine(finiteResourceRoles(10, 2))
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 500})

	require.True(t, outcome.Violated)
	assert.Equal(t, "Conservation", outcome.Invariant)
	assert.Greater(t, outcome.FailedAt, -1)
}

func TestSimulateDeterministic(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(5, 3))

	outcome1 := Simulate(m, NewBytestream(42), SimulationConfig{MaxSteps: 200})
	outcome2 := Simulate(m, NewBytestream(42), SimulationConfig{MaxSteps: 200})

	assert.Equal(t, outcome1.Violated, outcome2.Violated)
	assert.Equal(t, len(outcome1.Steps), len(outcome2.Steps))
	if outcome1.Violated {
		assert.Equal(t, outcome1.FailedAt, outcome2.FailedAt)
	}
}

func TestSimulateRespectsGuards(t *testing.T) {
	// All steps should be valid (no rule fires without its guard being true)
	m := NewFinitePooledResourceMachine(pooledRoles(3, 5, 100))
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 200})

	// If it passes, all steps respected guards (no panic, no negative values)
	_ = outcome
}

func TestSimulateMaxSteps(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 1000))
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 50})

	assert.False(t, outcome.Violated)
	assert.LessOrEqual(t, len(outcome.Steps), 50)
}

func TestSimulateFixedBytesPerStep(t *testing.T) {
	// Generous capacity — should run all maxSteps without violation
	m := NewFiniteResourceMachine(finiteResourceRoles(5, 1000))
	maxSteps := 100
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: maxSteps})

	require.False(t, outcome.Violated)
	assert.Equal(t, PrefixWidth+maxSteps*StepWidth, len(bs.Bytes()))
}

func TestSimulateFixedBytesPerStepManyInstances(t *testing.T) {
	// 500 instances — previously would have used 2-byte DrawInt
	m := NewFiniteResourceMachine(finiteResourceRoles(500, 100000))
	maxSteps := 100
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: maxSteps})

	require.False(t, outcome.Violated)
	assert.Equal(t, PrefixWidth+maxSteps*StepWidth, len(bs.Bytes()))
}

func TestSimulateVaryingInstanceCount(t *testing.T) {
	roles := finiteResourceRoles(50, 1000)
	roles.MinInstances = 1
	m := NewFiniteResourceMachine(roles)

	// Different seeds should produce different instance counts
	counts := map[int]bool{}
	for seed := int64(0); seed < 100; seed++ {
		bs := NewBytestream(seed)
		outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 10})
		assert.GreaterOrEqual(t, outcome.InstanceCount, 1)
		assert.LessOrEqual(t, outcome.InstanceCount, 50)
		counts[outcome.InstanceCount] = true
	}
	// With 100 seeds and range [1,50], we should see multiple distinct counts
	assert.Greater(t, len(counts), 1)
}

func TestSimulateInstanceCountRecorded(t *testing.T) {
	roles := finiteResourceRoles(10, 1000)
	roles.MinInstances = 5
	m := NewFiniteResourceMachine(roles)

	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 10})

	assert.GreaterOrEqual(t, outcome.InstanceCount, 5)
	assert.LessOrEqual(t, outcome.InstanceCount, 10)
}

func TestSimulatePooledFailing(t *testing.T) {
	// 10 instances, pool 10 each, only 50 downstream connections
	// 10 * 10 = 100 potential connections > 50 capacity
	m := NewFinitePooledResourceMachine(pooledRoles(10, 10, 50))
	bs := NewBytestream(42)
	outcome := Simulate(m, bs, SimulationConfig{MaxSteps: 1000})

	require.True(t, outcome.Violated)
	assert.Equal(t, "Conservation", outcome.Invariant)
}
