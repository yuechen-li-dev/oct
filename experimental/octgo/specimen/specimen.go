// Package specimen is an ordinary Go package used to dogfood OCTGO-M0.
// It has no dependency on Oct and exposes no interop-specific runtime API.
package specimen

type Threshold int

const DefaultThreshold Threshold = 2

func StrictlyAbove(value, threshold int) bool {
	return value > threshold
}

// Residual returns the non-negative trace remaining after a
// dimension-sensitive threshold correction.
func Residual(trace, dimension, threshold int) int {
	remaining := trace - dimension*threshold
	if remaining < 0 {
		return 0
	}
	return remaining
}
