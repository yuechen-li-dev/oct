# Mx101c report — `fft(...)` builtin + `FastFourierTransform` reference split

## 1) First-cut supported `fft(...)` surface

- Surface: `fft(values: Complex[]) -> Complex[] ! Error`
- Scope: 1D forward radix-2 FFT only (no inverse builtin in this milestone)
- Input policy:
  - non-empty input
  - power-of-two length
- Failure behavior: returns `Error` on invalid shape

## 2) Reference implementation rename/preservation

- Renamed pure Oct reference surface from `FFT(...)` to `FastFourierTransform(...)` in `Libraries/Mathematics/Mathematics.Transforms.oct`.
- Kept `IFFT(...)` as-is for reference round-trip workflows.
- Updated library tests/docs to use `FastFourierTransform(...)` and explicitly mark it as the reference/oracle path.

## 3) Interpreted mode support

- Added builtin registration for `fft`.
- Added typechecking for `fft(Complex[]) -> Complex[] ! Error`.
- Added runtime execution path in interpreter builtin dispatch.
- Added CPU implementation in Go (`internal/interpret/fft.go`) using iterative radix-2 Cooley-Tukey with bit-reversal permutation.

## 4) Compiled mode support

- Added compiled-call lowering support for builtin `fft`.
- Added compiled codegen support that calls generated helper `__octFFT`.
- Added generated helper implementation for CPU FFT and error handling, returning the fallible result type.
- Added compiled support for `Complex` type/builtin plumbing required by FFT programs (`Complex` go type mapping and constructor lowering).

## 5) Future Reactor acceleration seam

- `fft(...)` is now runtime/compiler-owned and implemented in Go helpers (interpreter + compiled helper function), not delegated to library Oct code.
- This isolates a clean backend seam so future routing can dispatch to CPU (today) or Reactor/Prometheus acceleration (later) behind the same builtin surface.

## 6) Intentionally deferred FFT features

- No inverse builtin (`ifft`) added.
- No multidimensional FFT.
- No real-input specialized FFT families.
- No windowing/DSP expansion.
- No Vulkan/Reactor FFT implementation in this milestone.
