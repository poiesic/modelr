package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoleMapHasRequiredRoles(t *testing.T) {
	rm := &RoleMap{
		Pattern:          PatternFiniteResource,
		MaxInstances:        10,
		ResourceCapacity: 100,
		OperationTime:    5,
	}
	assert.True(t, rm.Valid())
	assert.Empty(t, rm.Missing())
}

func TestRoleMapMissingRole(t *testing.T) {
	rm := &RoleMap{
		Pattern:          PatternFiniteResource,
		ResourceCapacity: 100,
	}
	assert.False(t, rm.Valid())
	assert.Contains(t, rm.Missing(), "instances")
}

func TestRoleMapPooledRequiresExtra(t *testing.T) {
	rm := &RoleMap{
		Pattern:          PatternFinitePooledResource,
		MaxInstances:        10,
		ResourceCapacity: 100,
	}
	assert.False(t, rm.Valid())
	missing := rm.Missing()
	assert.Contains(t, missing, "pool_capacity")
	assert.Contains(t, missing, "acquire_time")
}

func TestRoleMapPooledComplete(t *testing.T) {
	rm := &RoleMap{
		Pattern:          PatternFinitePooledResource,
		MaxInstances:        10,
		ResourceCapacity: 100,
		PoolCapacity:     20,
		AcquireTime:      20,
	}
	assert.True(t, rm.Valid())
}
