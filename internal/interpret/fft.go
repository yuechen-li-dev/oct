package interpret

import (
	"fmt"
	"math"
)

// fftCPU is the production CPU implementation used by the fft(...) builtin.
// This is intentionally runtime-owned (not delegated to Libraries/Mathematics)
// so the builtin remains a clean seam for future acceleration backends.
func fftCPU(values []Value) ([]Value, error) {
	n := len(values)
	if n == 0 {
		return nil, fmt.Errorf("runtime error: fft requires non-empty input")
	}
	if !isPowerOfTwo(n) {
		return nil, fmt.Errorf("runtime error: fft requires power-of-two input length")
	}

	out := make([]complex128, n)
	for i, value := range values {
		if value.Kind != ValueComplex {
			return nil, fmt.Errorf("runtime invariant violation: fft expects Complex[] input")
		}
		out[i] = value.Complex
	}

	for i := 0; i < n; i++ {
		j := reverseBits(i, n)
		if j > i {
			out[i], out[j] = out[j], out[i]
		}
	}

	for span := 2; span <= n; span *= 2 {
		halfSpan := span / 2
		phaseStep := -2.0 * math.Pi / float64(span)
		for blockStart := 0; blockStart < n; blockStart += span {
			for j := 0; j < halfSpan; j++ {
				angle := phaseStep * float64(j)
				twiddle := complex(math.Cos(angle), math.Sin(angle))
				odd := out[blockStart+j+halfSpan]
				t := twiddle * odd
				even := out[blockStart+j]
				out[blockStart+j] = even + t
				out[blockStart+j+halfSpan] = even - t
			}
		}
	}

	result := make([]Value, n)
	for i := range out {
		result[i] = Value{Kind: ValueComplex, Complex: out[i]}
	}
	return result, nil
}

func isPowerOfTwo(value int) bool {
	if value <= 0 {
		return false
	}
	return value&(value-1) == 0
}

func reverseBits(index int, width int) int {
	value := index
	reversed := 0
	n := width
	for n > 1 {
		lowBit := value & 1
		reversed = reversed*2 + lowBit
		value /= 2
		n /= 2
	}
	return reversed
}
