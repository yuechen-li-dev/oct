// fft_measure records real automatic-selection evidence, then requires a GPU
// call for each shape. An unavailable GPU is a nonzero result, never CPU timing.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/fft"
	"github.com/yuechen-li-dev/oct/internal/prometheus"
)

func main() {
	sizes := flag.String("sizes", "32,256,4096,65536,1048576", "comma-separated powers of two")
	flag.Parse()
	type record struct {
		fft.Measurement
		FirstCallNS      int64   `json:"first_call_ns"`
		SelectedCallNS   int64   `json:"selected_call_ns"`
		ForcedCallNS     int64   `json:"forced_call_ns"`
		MaxAbsoluteError float64 `json:"max_absolute_error"`
		MaxScaledError   float64 `json:"max_scaled_error"`
	}
	report := struct {
		Schema   string   `json:"schema"`
		Platform string   `json:"platform"`
		Scope    string   `json:"scope"`
		Records  []record `json:"records"`
	}{Schema: "oct.fft.measurement.v1", Platform: runtime.GOOS + "/" + runtime.GOARCH,
		Scope: "binary64, planned CPU vs complete GPU calls including packing, upload, submission, wait, readback; cold device/pipeline setup retained separately"}
	for _, size := range strings.Split(*sizes, ",") {
		n, err := strconv.Atoi(size)
		if err != nil {
			fail(err)
		}
		if err = fft.Validate(n); err != nil {
			fail(err)
		}
		values := make([]complex128, n)
		for i := range values {
			values[i] = complex(math.Sin(float64(i)*0.17)+math.Ldexp(1, -35), math.Cos(float64(i)*0.13))
		}
		engine := prometheus.NewFFTEngine()
		start := time.Now()
		_, err = engine.Transform(values, false)
		first := time.Since(start).Nanoseconds()
		if err != nil {
			engine.Close()
			fail(err)
		}
		measurement := engine.Measurement()
		_, selected, err := timedCall(func() ([]complex128, error) { return engine.Transform(values, false) })
		if err != nil {
			engine.Close()
			fail(err)
		}
		gpu, forced, err := timedCall(func() ([]complex128, error) { return engine.Transform(values, true) })
		engine.Close()
		if err != nil {
			fail(err)
		}
		reference := fft.CPU(values)
		maxAbs, scale := 0.0, 0.0
		for i, value := range reference {
			maxAbs = math.Max(maxAbs, math.Hypot(real(value-gpu[i]), imag(value-gpu[i])))
			scale = math.Max(scale, math.Hypot(real(value), imag(value)))
		}
		report.Records = append(report.Records, record{measurement, first, selected, forced, maxAbs, maxAbs / math.Max(1, scale)})
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fail(err)
	}
}
func timedCall(call func() ([]complex128, error)) ([]complex128, int64, error) {
	start := time.Now()
	for calls := int64(1); ; calls++ {
		output, err := call()
		if err != nil {
			return nil, 0, err
		}
		elapsed := time.Since(start)
		if elapsed >= 10*time.Millisecond {
			return output, elapsed.Nanoseconds() / calls, nil
		}
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
