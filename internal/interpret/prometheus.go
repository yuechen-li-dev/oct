package interpret

import "fmt"

// StrictPrometheusMatMul is installed by the native host at initialization.
// It must return an error when GPU execution cannot complete.
var StrictPrometheusMatMul func(Value, Value) (Value, error)

func strictPrometheusMatMul(left, right Value) (Value, error) {
	if StrictPrometheusMatMul == nil {
		return Value{}, fmt.Errorf("PROMETHEUS requires GPU matrix execution: backend unavailable")
	}
	return StrictPrometheusMatMul(left, right)
}
