# Prometheus acceleration (experimental)

[Reference home](../README.md) · [Coverage](../coverage.md)

> **Experimental:** Prometheus is a distinct subsystem.
> Its APIs, behavior, and runtime integration points may change.
> Do not treat Prometheus as stable core language surface.

## Overview

Prometheus supplies Oct's Vulkan acceleration. The numerical contract of
[`FFT()`](../language/builtins.md#fourier-transform) is the same on either backend.

For core builtin APIs, see [builtins](../language/builtins.md).
For standard library APIs, see [standard libraries](../language/standard-libraries.md).

## Current user-facing Prometheus surface

Prometheus-related language/runtime entry points currently include:

- `FFT(values)` automatically selects planned CPU or Vulkan binary64 execution.
- `PROMETHEUS { ... }` requires GPU execution of supported operations in ordinary
  functions, tests, or benchmarks.
- `PrometheusMatMul(a, b)` builtin for `Matrix<Float>` arguments.
- CLI command surfaces such as `oct prometheus-sgemm` and `oct prometheus-m1-async`.

## Automatic FFT selection

The first automatic call for a shape measures three CPU and three GPU timing
windows, each lasting at least 10 ms. GPU calls include packing, upload,
submission, completion, and readback. Device/pipeline creation is recorded
separately as a cold cost. The GPU must beat the CPU median by 20% and pass an
accuracy check before selection. This is a stability margin, not a hardcoded
size threshold. Calibration can cost much more than one small FFT.

One engine retains one GPU storage shape and one decision per program run.
Changing shape recalibrates. An unavailable or failed automatic GPU path selects
the CPU and retains its rejection reason. Nonfinite calibration results also
keep automatic execution on the CPU. Both paths retain binary64 real and
imaginary components; neither changes the source array.

Inspect full-call measurements and numerical error on your machine with:

```powershell
go run ./tools/fft_measure
```

The tool exits unsuccessfully if its explicit GPU verification cannot run.

## Require GPU execution

```oct
fn GpuSpectrum(values: Complex[]) -> Complex[] ! Error {
    PROMETHEUS {
        return FFT(values)?
    }
}
```

A direct `FFT()` call in the block bypasses CPU calibration and requires the
Vulkan FP64 path. Empty/non-power-of-two inputs remain recoverable input errors.
Missing Vulkan, missing `shaderFloat64`, unavailable shaders, oversized GPU
input, allocation failure, and execution failure return errors. They never
substitute a CPU transform. The GPU currently accepts 1 through 1,048,576
complex values; the ordinary CPU path accepts larger power-of-two inputs.
Software Vulkan devices such as llvmpipe do not satisfy a GPU requirement.

Direct matrix-matrix `@` and `PrometheusMatMul(a, b)` calls in the block also
require GPU completion. Their operands must be `Matrix<Float>`; this existing
SGEMM path uses FP32 arithmetic. Scalar control flow and preparation still run
on the host. A block does not turn arbitrary Oct code into a shader. Called
functions use their own lexical policy: put a block around the operation in
the helper when the helper must require GPU execution.

Outside a block, the older explicit `PrometheusMatMul(a, b)` builtin retains
its existing CPU fallback behavior.

## Native setup and qualification

Build the reactor with the platform helper and stage its shader package as
described in the [native guide](../../../internal/prometheus/native/README.md).
`OCT_PROMETHEUS_REACTOR` selects an explicit library. A library built before
M12 lacks the FP64 entry point and cannot satisfy a forced FFT call.

The [M12 report](../../../docs/internal/prometheus_concept_fft_m12.md) records
hardware evidence. The [Oct contracts](../../Prometheus/README.md) separate
ordinary, GPU-required, and missing-reactor environments.
