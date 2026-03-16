package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func finiteResourceRoles(instances, capacity int) *RoleMap {
	return &RoleMap{
		Pattern:          PatternFiniteResource,
		MaxInstances:        instances,
		ResourceCapacity: capacity,
		OperationTime:    5,
	}
}

func TestFiniteResourceInitState(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(3, 10))
	state := m.InitState(m.Roles)
	assert.Len(t, state.PerInstance, 3)
	assert.Equal(t, 0, state.PerInstance[0].Values["active_requests"])
	assert.Equal(t, 0, state.Shared.Values["used_resources"])
}

func TestFiniteResourceRequestArrives(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 10))
	state := m.InitState(m.Roles)

	assert.True(t, m.Rules[0].CanFire(state, 0))
	m.Rules[0].Apply(state, 0)
	assert.Equal(t, 1, state.PerInstance[0].Values["active_requests"])
	assert.Equal(t, 1, state.Shared.Values["used_resources"])
}

func TestFiniteResourceRequestCompletes(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 10))
	state := m.InitState(m.Roles)

	// Arrive first
	m.Rules[0].Apply(state, 0)

	// Complete
	assert.True(t, m.Rules[1].CanFire(state, 0))
	m.Rules[1].Apply(state, 0)
	assert.Equal(t, 0, state.PerInstance[0].Values["active_requests"])
	assert.Equal(t, 0, state.Shared.Values["used_resources"])
}

func TestFiniteResourceRequestCompletesGuard(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 10))
	state := m.InitState(m.Roles)

	assert.False(t, m.Rules[1].CanFire(state, 0)) // no active requests
}

func TestFiniteResourceConservationPass(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 10))
	state := m.InitState(m.Roles)
	state.Shared.Values["used_resources"] = 9

	assert.True(t, m.Invariants[0].Check(state, -1))
}

func TestFiniteResourceConservationFail(t *testing.T) {
	m := NewFiniteResourceMachine(finiteResourceRoles(2, 10))
	state := m.InitState(m.Roles)
	state.Shared.Values["used_resources"] = 11

	assert.False(t, m.Invariants[0].Check(state, -1))
}

func TestFiniteResourceConcurrentExhaustion(t *testing.T) {
	// 3 instances, capacity 2 — if all arrive, used_resources = 3 > 2
	m := NewFiniteResourceMachine(finiteResourceRoles(3, 2))
	state := m.InitState(m.Roles)

	m.Rules[0].Apply(state, 0) // instance 0 arrives
	m.Rules[0].Apply(state, 1) // instance 1 arrives
	m.Rules[0].Apply(state, 2) // instance 2 arrives

	require.Equal(t, 3, state.Shared.Values["used_resources"])
	assert.False(t, m.Invariants[0].Check(state, -1)) // conservation violated
}
