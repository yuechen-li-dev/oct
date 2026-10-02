package octrandom

import "errors"

// Key is the 64-bit Philox key held by an Oct Random.Stream (its _Key field,
// reinterpreted as unsigned). Every value is a valid key.
type Key uint64

// Counter word 3 separates the three uses of the generator so that a draw, a
// Child derivation and a Fork derivation can never share a counter.
const (
	domainDraw  uint32 = 0
	domainChild uint32 = 1
	domainFork  uint32 = 2
)

// ErrNegativeIndex is returned by every operation that takes an index.
var ErrNegativeIndex = errors.New("random: index must be >= 0")

// words runs Philox for one counter and packs the four 32-bit outputs into
// the two 64-bit words the specification names w0 and w1.
func words(k Key, n uint64, block uint32, domain uint32) (w0, w1 uint64) {
	out := Philox4x32(
		[4]uint32{uint32(n), uint32(n >> 32), block, domain},
		[2]uint32{uint32(k), uint32(k >> 32)},
	)
	return uint64(out[0]) | uint64(out[1])<<32, uint64(out[2]) | uint64(out[3])<<32
}

// drawWords returns the two 64-bit words of block `block` for the draw at
// index i. The caller has already rejected a negative index.
func drawWords(k Key, i int64, block uint32) (w0, w1 uint64) {
	return words(k, uint64(i), block, domainDraw)
}

// Seeded returns the stream key for a user seed. The seed is used directly:
// Philox is designed for structured keys, so no pre-mixing is needed.
func Seeded(seed int64) Key {
	return Key(uint64(seed))
}

// Fork derives an independent substream named by label.
func Fork(k Key, label string) Key {
	w0, _ := words(k, FNV1a64(label), 0, domainFork)
	return Key(w0)
}

// Child derives the substream for index i, for callers that need several
// draws, or a data-dependent number of draws, at one step.
func Child(k Key, i int64) (Key, error) {
	if i < 0 {
		return 0, ErrNegativeIndex
	}
	w0, _ := words(k, uint64(i), 0, domainChild)
	return Key(w0), nil
}
