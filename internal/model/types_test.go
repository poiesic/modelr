package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUncertaintyCategoryIsValid(t *testing.T) {
	valid := []string{
		"unstated_constraint",
		"unresolved_tradeoff",
		"undefined_boundary",
		"assumed_context",
		"deferred_decision",
		"unknown_unknown",
	}
	for _, c := range valid {
		assert.True(t, IsValidUncertaintyCategory(c), "expected %q to be valid", c)
	}

	assert.False(t, IsValidUncertaintyCategory("bogus"))
	assert.False(t, IsValidUncertaintyCategory(""))
}

func TestOperationIsValid(t *testing.T) {
	valid := []string{"read", "write", "read_write"}
	for _, op := range valid {
		assert.True(t, IsValidOperation(op), "expected %q to be valid", op)
	}

	assert.False(t, IsValidOperation("delete"))
	assert.False(t, IsValidOperation(""))
	assert.False(t, IsValidOperation("readwrite"))
}
