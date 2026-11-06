package prometheus

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/fft"
)

type reactorFFT64 func(reactorRuntimeHandle, []complex128) ([]complex128, error)

type fftBackend struct {
	runtime   *nativeRuntime
	transform reactorFFT64
}

func init() { fft.NewBackend = newFFTBackend }

// NewFFTEngine binds automatic and strict execution to the same native adapter.
func NewFFTEngine() *fft.Engine { return fft.NewEngine() }

func newFFTBackend() (fft.Backend, error) {
	runtime, err := newNativeRuntime()
	if err != nil {
		return nil, err
	}
	if err = runtime.requireHardwareGPU(); err != nil {
		runtime.Close()
		return nil, err
	}
	symbol, err := runtime.lib.Resolve("prometheus_reactor_runtime_fft64")
	if err != nil {
		runtime.Close()
		return nil, err
	}
	transform, ok := symbol.(reactorFFT64)
	if !ok {
		runtime.Close()
		return nil, fmt.Errorf("reactor lacks a binary64 FFT adapter")
	}
	return &fftBackend{runtime: runtime, transform: transform}, nil
}
func (b *fftBackend) Transform(values []complex128) ([]complex128, error) {
	if len(values) > 1<<20 {
		return nil, fmt.Errorf("Vulkan FFT supports at most 1048576 complex values")
	}
	return b.transform(b.runtime.handle, values)
}
func (b *fftBackend) Close() { b.runtime.Close() }

func (r *nativeRuntime) requireHardwareGPU() error {
	if !r.caps.Available || r.caps.BackendType != 2 {
		return fmt.Errorf("GPU execution requires hardware Vulkan; backend is %s", r.Environment())
	}
	return nil
}
