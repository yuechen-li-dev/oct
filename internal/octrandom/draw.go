package octrandom

import (
	"errors"
	"math"
)

// Precondition failures. These are programmer errors: the interpreter reports
// them as non-recoverable runtime errors and generated programs panic with
// the same text.
var (
	ErrFloatRange    = errors.New("random: Between requires lo <= hi")
	ErrFloatSpan     = errors.New("random: Between requires finite lo, hi and hi - lo")
	ErrIntRange      = errors.New("random: IntBetween requires lo <= hi")
	ErrNormalStddev  = errors.New("random: Normal requires stddev >= 0")
	ErrNormalNotReal = errors.New("random: Normal requires finite mean and stddev")
)

// unitScale is 2^-53. Multiplying a 53-bit integer by it is exact.
const unitScale = 1.0 / (1 << 53)

func unitFromWord(w uint64) float64 {
	return float64(w>>11) * unitScale
}

// Unit returns the uniform draw in [0, 1) at index i.
func Unit(k Key, i int64) (float64, error) {
	if i < 0 {
		return 0, ErrNegativeIndex
	}
	w0, _ := drawWords(k, i, 0)
	return unitFromWord(w0), nil
}

// Between returns the uniform draw in [lo, hi) at index i. When lo == hi it
// returns lo.
func Between(k Key, i int64, lo, hi float64) (float64, error) {
	if i < 0 {
		return 0, ErrNegativeIndex
	}
	if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		return 0, ErrFloatSpan
	}
	if lo > hi {
		return 0, ErrFloatRange
	}
	span := hi - lo
	if math.IsInf(span, 0) {
		return 0, ErrFloatSpan
	}
	if lo == hi {
		return lo, nil
	}
	w0, _ := drawWords(k, i, 0)
	return betweenFromUnit(unitFromWord(w0), lo, span, hi), nil
}

func betweenFromUnit(u, lo, span, hi float64) float64 {
	r := lo + float64(span*u)
	if r >= hi {
		// Rounding can land exactly on hi; keep the interval half-open.
		r = math.Nextafter(hi, lo)
	}
	return r
}

// IntBetween returns the unbiased uniform integer draw in [lo, hi] at index i.
func IntBetween(k Key, i int64, lo, hi int64) (int64, error) {
	if i < 0 {
		return 0, ErrNegativeIndex
	}
	if lo > hi {
		return 0, ErrIntRange
	}
	span := uint64(hi) - uint64(lo) + 1
	if span == 0 {
		// The full Int range: every 64-bit word is a valid result.
		w0, _ := drawWords(k, i, 0)
		return int64(w0), nil
	}
	// rem is 2^64 mod span. Words at or above 2^64 - rem would bias the
	// result, so they are rejected and the next word is tried.
	rem := (-span) % span
	for block := uint32(0); ; block++ {
		w0, w1 := drawWords(k, i, block)
		if acceptWord(w0, rem) {
			return int64(uint64(lo) + w0%span), nil
		}
		if acceptWord(w1, rem) {
			return int64(uint64(lo) + w1%span), nil
		}
	}
}

func acceptWord(w, rem uint64) bool {
	return rem == 0 || w < -rem
}

// Normal returns the Gaussian draw at index i by the Box-Muller transform of
// the two words of block 0.
func Normal(k Key, i int64, mean, stddev float64) (float64, error) {
	if i < 0 {
		return 0, ErrNegativeIndex
	}
	if math.IsNaN(mean) || math.IsNaN(stddev) || math.IsInf(mean, 0) || math.IsInf(stddev, 0) {
		return 0, ErrNormalNotReal
	}
	if stddev < 0 {
		return 0, ErrNormalStddev
	}
	w0, w1 := drawWords(k, i, 0)
	return normalFromWords(w0, w1, mean, stddev), nil
}

func normalFromWords(w0, w1 uint64, mean, stddev float64) float64 {
	// u1 is in (0, 1], so its logarithm is finite and non-positive.
	u1 := float64((w0>>11)+1) * unitScale
	u2 := unitFromWord(w1)
	radius := math.Sqrt(float64(-2 * math.Log(u1)))
	z := float64(radius * math.Cos(float64(2*math.Pi*u2)))
	return mean + float64(stddev*z)
}
