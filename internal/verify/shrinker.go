package verify

// ShrinkProgress reports the shrinker's state at a point in time.
type ShrinkProgress struct {
	Phase       string // "delete_chunks", "zero_chunks", "reduce_bytes"
	Attempt     int
	MaxAttempts int
	BestLength  int
	BestSteps   int
	Improved    bool
}

// ShrinkConfig controls shrinking behavior.
type ShrinkConfig struct {
	MaxAttempts int
	OnProgress  func(ShrinkProgress) // nil = silent
}

// Shrink attempts to find a shorter bytestream that still produces a failing simulation.
func Shrink(machine *Machine, failingBytes []byte, simConfig SimulationConfig, shrinkConfig ShrinkConfig) []byte {
	maxAttempts := shrinkConfig.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1000
	}

	best := make([]byte, len(failingBytes))
	copy(best, failingBytes)
	attempts := 0

	// Replay initial bytes to get starting step count
	initOutcome := tryCandidate(machine, best, simConfig)
	bestSteps := initOutcome.FailedAt + 1

	emit := func(phase string, improved bool) {
		if shrinkConfig.OnProgress != nil {
			shrinkConfig.OnProgress(ShrinkProgress{
				Phase:       phase,
				Attempt:     attempts,
				MaxAttempts: maxAttempts,
				BestLength:  len(best),
				BestSteps:   bestSteps,
				Improved:    improved,
			})
		}
	}

	// Repeat all three phases until a full pass produces no improvements
	for attempts < maxAttempts {
		prevLen := len(best)
		prevSteps := bestSteps

		// Phase 1: Delete step-aligned chunks — skip prefix, try removing progressively smaller chunks
		emit("delete_chunks", false)
		for chunkSize := alignDown(len(best)/2, StepWidth); chunkSize >= StepWidth && attempts < maxAttempts; chunkSize = alignDown(chunkSize/2, StepWidth) {
			for start := PrefixWidth; start+chunkSize <= len(best) && attempts < maxAttempts; start += StepWidth {
				candidate := deleteChunk(best, start, chunkSize)
				attempts++
				outcome := tryCandidate(machine, candidate, simConfig)
				if outcome.Violated {
					best = candidate
					bestSteps = outcome.FailedAt + 1
					emit("delete_chunks", true)
					// Don't advance start — try deleting at the same position again
				}
			}
		}

		// Phase 2: Zero step-aligned chunks — skip prefix, try zeroing progressively smaller chunks
		emit("zero_chunks", false)
		for chunkSize := alignDown(len(best)/2, StepWidth); chunkSize >= StepWidth && attempts < maxAttempts; chunkSize = alignDown(chunkSize/2, StepWidth) {
			for start := PrefixWidth; start+chunkSize <= len(best) && attempts < maxAttempts; start += StepWidth {
				candidate := zeroChunk(best, start, chunkSize)
				attempts++
				outcome := tryCandidate(machine, candidate, simConfig)
				if outcome.Violated {
					best = candidate
					bestSteps = outcome.FailedAt + 1
					emit("zero_chunks", true)
				}
			}
		}

		// Phase 3: Reduce individual bytes — binary search toward zero
		emit("reduce_bytes", false)
		for i := 0; i < len(best) && attempts < maxAttempts; i++ {
			if best[i] == 0 {
				continue
			}
			lo, hi := byte(0), best[i]
			for lo < hi && attempts < maxAttempts {
				mid := lo + (hi-lo)/2
				candidate := make([]byte, len(best))
				copy(candidate, best)
				candidate[i] = mid
				attempts++
				outcome := tryCandidate(machine, candidate, simConfig)
				if outcome.Violated {
					best = candidate
					bestSteps = outcome.FailedAt + 1
					hi = mid
					emit("reduce_bytes", true)
				} else {
					lo = mid + 1
				}
			}
		}

		// No improvement this pass — converged
		if len(best) == prevLen && bestSteps == prevSteps {
			break
		}
	}

	return best
}

func tryCandidate(machine *Machine, data []byte, config SimulationConfig) *SimulationOutcome {
	bs := FromBytes(data)
	return Simulate(machine, bs, config)
}

func deleteChunk(data []byte, start, length int) []byte {
	result := make([]byte, 0, len(data)-length)
	result = append(result, data[:start]...)
	result = append(result, data[start+length:]...)
	return result
}

func zeroChunk(data []byte, start, length int) []byte {
	result := make([]byte, len(data))
	copy(result, data)
	for i := start; i < start+length && i < len(result); i++ {
		result[i] = 0
	}
	return result
}

func alignDown(n, align int) int {
	if align <= 0 {
		return n
	}
	return (n / align) * align
}
