// Package fft owns the binary64 FFT contract and measured backend selection.
// Backends provide execution, never a CPU substitute for a GPU request.
package fft

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type Backend interface {
	Transform([]complex128) ([]complex128, error)
	Close()
}

// NewBackend is installed once by the native host during package initialization.
// Keeping the adapter outside this package avoids an interpreter import cycle.
var NewBackend func() (Backend, error)

type Measurement struct {
	Elements  int    `json:"elements"`
	Selected  string `json:"selected"`
	Reason    string `json:"reason"`
	CPUWarmNS int64  `json:"cpu_warm_ns"`
	GPUColdNS int64  `json:"gpu_cold_ns"`
	GPUWarmNS int64  `json:"gpu_warm_ns"`
}

// Engine is scoped to a program run. One serialized backend and one current
// shape bound GPU storage; changing shape recalibrates rather than reusing stale
// warm measurements for a newly allocated plan. Close joins/releases the owner.
// An Engine must not be copied after first use.
type Engine struct {
	mu         sync.Mutex
	backend    Backend
	attempted  bool
	backendErr error
	last       Measurement
	roots      []complex128
	rootCount  int
}

func NewEngine() *Engine { return &Engine{} }

func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.backend != nil {
		e.backend.Close()
		e.backend = nil
	}
	e.attempted = false
	e.last = Measurement{}
	e.roots = nil
	e.rootCount = 0
}

func (e *Engine) Measurement() Measurement {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last
}

func (e *Engine) gpu(values []complex128) ([]complex128, error) {
	if !e.attempted {
		e.attempted = true
		e.backendErr = fmt.Errorf("Vulkan FFT backend unavailable")
		if NewBackend != nil {
			e.backend, e.backendErr = NewBackend()
		}
	}
	if e.backendErr != nil {
		return nil, e.backendErr
	}
	return e.backend.Transform(values)
}

func (e *Engine) Transform(values []complex128, requiredGPU bool) ([]complex128, error) {
	if err := Validate(len(values)); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if requiredGPU {
		if e.last.Elements != len(values) {
			e.last = Measurement{}
		}
		out, err := e.gpu(values)
		if err != nil {
			return nil, fmt.Errorf("PROMETHEUS FFT requires GPU execution: %w", err)
		}
		return out, nil
	}
	if e.last.Elements == len(values) {
		if e.last.Selected == "vulkan-fp64" {
			out, err := e.gpu(values)
			if err == nil {
				return out, nil
			}
			e.last.Selected = "cpu"
			e.last.Reason = "GPU execution failed: " + err.Error()
		}
		return e.cpu(values), nil
	}
	e.last = Measurement{Elements: len(values), Selected: "cpu"}
	var cpuTimes, gpuTimes []int64
	var reference []complex128
	for run := 0; run < 3; run++ {
		var elapsed int64
		reference, elapsed, _ = measure(func() ([]complex128, error) { return e.cpu(values), nil })
		cpuTimes = append(cpuTimes, elapsed)
	}
	e.last.CPUWarmNS = median(cpuTimes)
	start := time.Now()
	candidate, err := e.gpu(values)
	e.last.GPUColdNS = time.Since(start).Nanoseconds()
	if err != nil {
		e.last.Reason = err.Error()
		return reference, nil
	}
	if !agrees(reference, candidate) {
		e.last.Reason = "GPU calibration did not preserve binary64 accuracy"
		return reference, nil
	}
	for run := 0; run < 3; run++ {
		var elapsed int64
		candidate, elapsed, err = measure(func() ([]complex128, error) { return e.gpu(values) })
		gpuTimes = append(gpuTimes, elapsed)
		if err != nil {
			e.last.Reason = err.Error()
			return reference, nil
		}
		if !agrees(reference, candidate) {
			e.last.Reason = "GPU calibration did not preserve binary64 accuracy"
			return reference, nil
		}
	}
	e.last.GPUWarmNS = median(gpuTimes)
	// A named stability margin avoids selecting a GPU on timing noise. Both
	// candidates are measured full calls, including allocation/packing, upload,
	// submission, wait and readback. Cold setup is retained separately.
	if e.last.GPUWarmNS < e.last.CPUWarmNS*4/5 {
		e.last.Selected = "vulkan-fp64"
		e.last.Reason = "warm GPU call exceeds 20 percent speedup margin"
		return candidate, nil
	}
	e.last.Reason = "CPU wins measured full-call comparison with 20 percent stability margin"
	return reference, nil
}

// Aggregate calls until the timer's granularity is insignificant. In
// particular, Windows clocks can report zero for a single small transform.
// Three windows feed the median; every iteration still performs a full call.
func measure(call func() ([]complex128, error)) ([]complex128, int64, error) {
	start := time.Now()
	var output []complex128
	for count := int64(1); ; count++ {
		var err error
		output, err = call()
		if err != nil {
			return nil, 0, err
		}
		elapsed := time.Since(start)
		if elapsed >= 10*time.Millisecond {
			return output, elapsed.Nanoseconds() / count, nil
		}
	}
}

func median(values []int64) int64 {
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values[len(values)/2]
}

func agrees(reference, actual []complex128) bool {
	if len(reference) != len(actual) {
		return false
	}
	scale := 0.0
	for _, value := range reference {
		scale = math.Max(scale, math.Max(math.Abs(real(value)), math.Abs(imag(value))))
	}
	tolerance := 1e-12*scale + math.SmallestNonzeroFloat64*float64(len(reference))
	for i, value := range actual {
		if math.IsNaN(real(value)) || math.IsNaN(imag(value)) || math.IsInf(real(value), 0) || math.IsInf(imag(value), 0) ||
			math.IsNaN(real(reference[i])) || math.IsNaN(imag(reference[i])) || math.IsInf(real(reference[i]), 0) || math.IsInf(imag(reference[i]), 0) ||
			math.Abs(real(value)-real(reference[i])) > tolerance || math.Abs(imag(value)-imag(reference[i])) > tolerance {
			return false
		}
	}
	return true
}

func Validate(n int) error {
	if n == 0 {
		return fmt.Errorf("runtime error: FFT requires non-empty input")
	}
	if n&(n-1) != 0 {
		return fmt.Errorf("runtime error: FFT requires power-of-two input length")
	}
	return nil
}

// CPU requires a validated length and returns independent binary64 storage.
func CPU(values []complex128) []complex128 {
	return cpuWithRoots(values, roots(len(values)))
}

func (e *Engine) cpu(values []complex128) []complex128 {
	if e.rootCount != len(values) {
		e.roots = roots(len(values))
		e.rootCount = len(values)
	}
	return cpuWithRoots(values, e.roots)
}

func roots(n int) []complex128 {
	twiddles := make([]complex128, n/2)
	for j := range twiddles {
		angle := -2 * math.Pi / float64(n) * float64(j)
		twiddles[j] = complex(math.Cos(angle), math.Sin(angle))
	}
	return twiddles
}

func cpuWithRoots(values []complex128, twiddles []complex128) []complex128 {
	out := append([]complex128(nil), values...)
	n := len(out)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			out[i], out[j] = out[j], out[i]
		}
	}
	// Both candidates retain their shape's roots of unity. Calibration compares
	// planned CPU execution with planned GPU execution under the same contract.
	for span := 2; span <= n; span *= 2 {
		half := span / 2
		for base := 0; base < n; base += span {
			for j := 0; j < half; j++ {
				t := twiddles[j*(n/span)] * out[base+j+half]
				a := out[base+j]
				out[base+j] = a + t
				out[base+j+half] = a - t
			}
		}
		if span == n {
			break
		}
	}
	return out
}
