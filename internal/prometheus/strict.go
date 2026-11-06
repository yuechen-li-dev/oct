package prometheus

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/interpret"
)

// RunCompiledMatMulMMStrict never computes a CPU substitute. The existing
// explicit PrometheusMatMul builtin outside a block retains its fallback API.
func RunCompiledMatMulMMStrict(left, right [][]float64) ([][]float64, string, error) {
	request, _, err := compiledMatMulRequest(left, right)
	if err != nil {
		return nil, "", err
	}
	runtime, err := newNativeRuntime()
	if err != nil {
		return nil, "", fmt.Errorf("PROMETHEUS requires GPU matrix execution: %w", err)
	}
	defer runtime.Close()
	if err = runtime.requireHardwareGPU(); err != nil {
		return nil, "", fmt.Errorf("PROMETHEUS requires GPU matrix execution: %w", err)
	}
	output, status, err := runtime.SGEMM(request.Shape.M, request.Shape.N, request.Shape.K, request.A, request.B)
	if err != nil {
		return nil, "", fmt.Errorf("PROMETHEUS requires GPU matrix execution: %w", err)
	}
	if status.Kind != RunStatusOK {
		return nil, "", fmt.Errorf("PROMETHEUS matrix execution failed: %v", status)
	}
	return reshapeMatrixFloat64(request.Shape.M, request.Shape.N, output), runtime.Environment(), nil
}

func init() {
	interpret.StrictPrometheusMatMul = func(left, right interpret.Value) (interpret.Value, error) {
		convert := func(value interpret.Value) ([][]float64, error) {
			if value.Kind != interpret.ValueMatrix {
				return nil, fmt.Errorf("PROMETHEUS matrix execution requires Matrix<Float>")
			}
			rows := make([][]float64, value.Matrix.Rows)
			for r := range rows {
				rows[r] = make([]float64, value.Matrix.Cols)
				for c := range rows[r] {
					entry := value.Matrix.Elements[r*value.Matrix.Cols+c]
					if entry.Kind != interpret.ValueFloat {
						return nil, fmt.Errorf("PROMETHEUS matrix execution requires Matrix<Float>")
					}
					rows[r][c] = entry.Float
				}
			}
			return rows, nil
		}
		a, err := convert(left)
		if err != nil {
			return interpret.Value{}, err
		}
		b, err := convert(right)
		if err != nil {
			return interpret.Value{}, err
		}
		output, _, err := RunCompiledMatMulMMStrict(a, b)
		if err != nil {
			return interpret.Value{}, err
		}
		matrix := interpret.MatrixValue{Rows: len(output), Cols: len(output[0])}
		for _, row := range output {
			for _, value := range row {
				matrix.Elements = append(matrix.Elements, interpret.Value{Kind: interpret.ValueFloat, Float: value})
			}
		}
		return interpret.Value{Kind: interpret.ValueMatrix, Matrix: matrix}, nil
	}
}
