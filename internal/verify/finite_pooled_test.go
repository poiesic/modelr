package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func pooledRoles(instances, poolCap, resCap int) *RoleMap {
	return &RoleMap{
		Pattern:          PatternFinitePooledResource,
		MaxInstances:        instances,
		ResourceCapacity: resCap,
		PoolCapacity:     poolCap,
		AcquireTime:      20,
		OperationTime:    5,
	}
}

func TestPooledInitState(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	assert.Len(t, state.PerInstance, 2)
	assert.Equal(t, 0, state.PerInstance[0].Values["pool"])
	assert.Equal(t, 0, state.PerInstance[0].Values["in_flight"])
	assert.Equal(t, 0, state.PerInstance[0].Values["pending"])
	assert.Equal(t, 0, state.Shared.Values["used_connections"])
}

func TestPooledRequestArrives(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	m.Rules[0].Apply(state, 0) // RequestArrives
	assert.Equal(t, 1, state.PerInstance[0].Values["pending"])
}

func TestPooledStartGrowthGuard(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)

	// No pending → can't fire
	assert.False(t, m.Rules[1].CanFire(state, 0))

	// Add pending
	state.PerInstance[0].Values["pending"] = 1
	assert.True(t, m.Rules[1].CanFire(state, 0))
}

func TestPooledStartGrowthBlocked(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["pending"] = 0
	assert.False(t, m.Rules[1].CanFire(state, 0))
}

func TestPooledStartGrowthAtCapacity(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 2, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["pending"] = 1
	state.PerInstance[0].Values["pool"] = 1
	state.PerInstance[0].Values["in_flight"] = 1
	// pool + in_flight == pool_capacity (2) → blocked
	assert.False(t, m.Rules[1].CanFire(state, 0))
}

func TestPooledStartGrowthEffect(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["pending"] = 1
	m.Rules[1].Apply(state, 0) // StartGrowth
	assert.Equal(t, 0, state.PerInstance[0].Values["pending"])
	assert.Equal(t, 1, state.PerInstance[0].Values["in_flight"])
	assert.Equal(t, 1, state.Shared.Values["used_connections"])
}

func TestPooledGrowComplete(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["in_flight"] = 1

	assert.True(t, m.Rules[2].CanFire(state, 0))
	m.Rules[2].Apply(state, 0) // GrowComplete
	assert.Equal(t, 0, state.PerInstance[0].Values["in_flight"])
	assert.Equal(t, 1, state.PerInstance[0].Values["pool"])
}

func TestPooledRequestCompletes(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["pool"] = 1
	state.Shared.Values["used_connections"] = 1

	assert.True(t, m.Rules[3].CanFire(state, 0))
	m.Rules[3].Apply(state, 0) // RequestCompletes
	assert.Equal(t, 0, state.PerInstance[0].Values["pool"])
	assert.Equal(t, 0, state.Shared.Values["used_connections"])
}

func TestPooledConservationPass(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	state.Shared.Values["used_connections"] = 99
	assert.True(t, m.Invariants[0].Check(state, -1))
}

func TestPooledConservationFail(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 10, 100))
	state := m.InitState(m.Roles)
	state.Shared.Values["used_connections"] = 101
	assert.False(t, m.Invariants[0].Check(state, -1))
}

func TestPooledPoolBoundedPass(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 5, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["pool"] = 3
	state.PerInstance[0].Values["in_flight"] = 2
	assert.True(t, m.Invariants[1].Check(state, 0))
}

func TestPooledPoolBoundedFail(t *testing.T) {
	m := NewFinitePooledResourceMachine(pooledRoles(2, 5, 100))
	state := m.InitState(m.Roles)
	state.PerInstance[0].Values["pool"] = 3
	state.PerInstance[0].Values["in_flight"] = 3
	assert.False(t, m.Invariants[1].Check(state, 0))
}
