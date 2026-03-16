package verify

import "math"

// SPRTConfig configures the sequential probability ratio test.
type SPRTConfig struct {
	TargetFailureRate float64 // p0: maximum acceptable failure probability
	Confidence        float64 // 1-alpha = 1-beta
}

// SPRTDecision is the outcome of an SPRT evaluation.
type SPRTDecision int

const (
	SPRTContinue SPRTDecision = iota
	SPRTAccept                // H0: safe
	SPRTReject                // H1: flawed
)

// SPRT implements Wald's sequential probability ratio test.
type SPRT struct {
	p0          float64 // target failure rate
	p1          float64 // alternative (10x p0)
	lnA         float64 // upper threshold (reject H0)
	lnB         float64 // lower threshold (accept H0)
	lnLR        float64 // cumulative log-likelihood ratio
	simulations int
	failures    int
}

// NewSPRT creates a new SPRT calculator.
func NewSPRT(config SPRTConfig) *SPRT {
	p0 := config.TargetFailureRate
	p1 := p0 * 10 // alternative hypothesis: failure rate is 10x the target
	if p1 > 0.5 {
		p1 = 0.5
	}

	alpha := 1 - config.Confidence // type I error
	beta := alpha                  // type II error (symmetric)

	lnA := math.Log((1 - beta) / alpha)
	lnB := math.Log(beta / (1 - alpha))

	return &SPRT{
		p0:  p0,
		p1:  p1,
		lnA: lnA,
		lnB: lnB,
	}
}

// Update records a simulation result and returns the current decision.
func (s *SPRT) Update(failed bool) SPRTDecision {
	s.simulations++
	if failed {
		s.failures++
		s.lnLR += math.Log(s.p1 / s.p0)
	} else {
		s.lnLR += math.Log((1 - s.p1) / (1 - s.p0))
	}

	if s.lnLR >= s.lnA {
		return SPRTReject
	}
	if s.lnLR <= s.lnB {
		return SPRTAccept
	}
	return SPRTContinue
}

// Simulations returns the total number of simulations run.
func (s *SPRT) Simulations() int { return s.simulations }

// Failures returns the number of failed simulations.
func (s *SPRT) Failures() int { return s.failures }

// EstimatedFailureRate returns the observed failure rate.
func (s *SPRT) EstimatedFailureRate() float64 {
	if s.simulations == 0 {
		return 0
	}
	return float64(s.failures) / float64(s.simulations)
}
