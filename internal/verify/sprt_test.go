package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func defaultSPRTConfig() SPRTConfig {
	return SPRTConfig{TargetFailureRate: 0.001, Confidence: 0.99}
}

func TestSPRTAcceptAfterAllPass(t *testing.T) {
	sprt := NewSPRT(defaultSPRTConfig())
	var decision SPRTDecision
	for i := 0; i < 10000; i++ {
		decision = sprt.Update(false)
		if decision != SPRTContinue {
			break
		}
	}
	assert.Equal(t, SPRTAccept, decision)
}

func TestSPRTRejectAfterFailures(t *testing.T) {
	sprt := NewSPRT(defaultSPRTConfig())
	var decision SPRTDecision
	for i := 0; i < 1000; i++ {
		// Fail every 4th simulation (~25% failure rate, way above 0.1% target)
		decision = sprt.Update(i%4 == 0)
		if decision != SPRTContinue {
			break
		}
	}
	assert.Equal(t, SPRTReject, decision)
	assert.Less(t, sprt.Simulations(), 50, "should reject quickly with high failure rate")
}

func TestSPRTContinue(t *testing.T) {
	sprt := NewSPRT(defaultSPRTConfig())
	// Just a few observations — not enough evidence yet
	decision := sprt.Update(false)
	assert.Equal(t, SPRTContinue, decision)
}

func TestSPRTDefaultParams(t *testing.T) {
	sprt := NewSPRT(defaultSPRTConfig())
	assert.NotZero(t, sprt.lnA)
	assert.NotZero(t, sprt.lnB)
	assert.Greater(t, sprt.lnA, sprt.lnB, "upper threshold > lower threshold")
}

func TestSPRTCustomParams(t *testing.T) {
	custom := NewSPRT(SPRTConfig{TargetFailureRate: 0.01, Confidence: 0.95})
	def := NewSPRT(defaultSPRTConfig())
	// Different configs → different thresholds
	assert.NotEqual(t, custom.lnA, def.lnA)
}

func TestSPRTEstimatedFailureRate(t *testing.T) {
	sprt := NewSPRT(defaultSPRTConfig())
	sprt.Update(true)
	sprt.Update(false)
	sprt.Update(false)
	sprt.Update(true)
	assert.InDelta(t, 0.5, sprt.EstimatedFailureRate(), 0.01)
}
