package verify

import "math/rand"

// Bytestream provides deterministic random byte generation for simulations.
// All randomness in a simulation is driven by the bytestream, making simulations
// reproducible from a recorded byte sequence.
type Bytestream struct {
	rng   *rand.Rand
	drawn []byte
	// When replaying from recorded bytes, use those instead of rng
	replay []byte
	pos    int
}

// NewBytestream creates a new bytestream seeded with the given value.
func NewBytestream(seed int64) *Bytestream {
	return &Bytestream{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// FromBytes creates a bytestream that replays from recorded bytes.
func FromBytes(data []byte) *Bytestream {
	return &Bytestream{
		replay: data,
	}
}

// DrawByte returns a random byte, recording it for later replay.
func (b *Bytestream) DrawByte() byte {
	if b.replay != nil && b.pos < len(b.replay) {
		val := b.replay[b.pos]
		b.pos++
		b.drawn = append(b.drawn, val)
		return val
	}
	if b.rng != nil {
		val := byte(b.rng.Intn(256))
		b.drawn = append(b.drawn, val)
		return val
	}
	// Exhausted replay with no rng fallback — return 0
	b.drawn = append(b.drawn, 0)
	return 0
}

// DrawInt returns a random integer in [0, n) using rejection sampling.
func (b *Bytestream) DrawInt(n int) int {
	if n <= 0 {
		return 0
	}
	if n == 1 {
		b.DrawByte() // consume a byte to keep stream advancing
		return 0
	}
	// Use one or two bytes depending on n
	if n <= 256 {
		return int(b.DrawByte()) % n
	}
	hi := int(b.DrawByte())
	lo := int(b.DrawByte())
	return (hi*256 + lo) % n
}

// Bytes returns all bytes drawn so far.
func (b *Bytestream) Bytes() []byte {
	result := make([]byte, len(b.drawn))
	copy(result, b.drawn)
	return result
}
