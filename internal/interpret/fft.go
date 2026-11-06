package interpret

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/fft"
)

// fftTransform adapts Oct values without changing binary64 precision.
func fftTransform(engine *fft.Engine, values []Value, requiredGPU bool) ([]Value, error) {
	input := make([]complex128, len(values))
	for index, value := range values {
		if value.Kind != ValueComplex {
			return nil, fmt.Errorf("runtime invariant violation: FFT expects Complex[] input")
		}
		input[index] = value.Complex
	}
	out, err := engine.Transform(input, requiredGPU)
	if err != nil {
		return nil, err
	}
	n := len(out)

	result := make([]Value, n)
	for i := range out {
		result[i] = Value{Kind: ValueComplex, Complex: out[i]}
	}
	return result, nil
}
