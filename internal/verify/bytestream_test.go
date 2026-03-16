package verify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBytestreamDeterministic(t *testing.T) {
	bs1 := NewBytestream(42)
	bs2 := NewBytestream(42)
	for i := 0; i < 100; i++ {
		assert.Equal(t, bs1.DrawByte(), bs2.DrawByte(), "byte %d differs", i)
	}
}

func TestBytestreamDifferentSeeds(t *testing.T) {
	bs1 := NewBytestream(42)
	bs2 := NewBytestream(99)
	same := 0
	for i := 0; i < 100; i++ {
		if bs1.DrawByte() == bs2.DrawByte() {
			same++
		}
	}
	assert.Less(t, same, 50, "different seeds should produce mostly different bytes")
}

func TestBytestreamDrawByte(t *testing.T) {
	bs := NewBytestream(42)
	b := bs.DrawByte()
	assert.GreaterOrEqual(t, int(b), 0)
	assert.LessOrEqual(t, int(b), 255)
}

func TestBytestreamDrawInt(t *testing.T) {
	bs := NewBytestream(42)
	for i := 0; i < 100; i++ {
		v := bs.DrawInt(10)
		assert.GreaterOrEqual(t, v, 0)
		assert.Less(t, v, 10)
	}
}

func TestBytestreamRecordable(t *testing.T) {
	bs := NewBytestream(42)
	bs.DrawByte()
	bs.DrawByte()
	bs.DrawByte()
	assert.Len(t, bs.Bytes(), 3)
}

func TestBytestreamReplay(t *testing.T) {
	// Record
	bs1 := NewBytestream(42)
	for i := 0; i < 10; i++ {
		bs1.DrawByte()
	}
	recorded := bs1.Bytes()

	// Replay
	bs2 := FromBytes(recorded)
	for i := 0; i < 10; i++ {
		assert.Equal(t, recorded[i], bs2.DrawByte(), "replay byte %d differs", i)
	}
}
