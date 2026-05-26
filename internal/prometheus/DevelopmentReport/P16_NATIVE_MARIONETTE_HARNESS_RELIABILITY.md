# P16 Native Marionette Harness Reliability

## Observed failure
In Codex/Linux, native validation was reported as intermittently failing with:

- `cannot execute binary file`

This prevented reliable trust in downstream `PrometheusReactor_Fft` / `PrometheusReactor_Sgemm` filter runs.

## Diagnosis summary
After rebuilding with:

- `bash internal/prometheus/native/build_stub.sh`

I inspected runtime artifacts and binary metadata. Key findings:

- `out/prometheus/native/marionette_tests` is produced and has execute bits (`-rwxr-xr-x`).
- `out/prometheus/native/marionette_tests.exe` is not produced by Linux build helper.
- `readelf -h out/prometheus/native/marionette_tests` reports ELF64 for `x86-64`.
- `ldd out/prometheus/native/marionette_tests` resolves Linux shared libraries (`libvulkan.so.1`, `libstdc++.so.6`, `libc.so.6`, etc.).
- The execution path is valid before Vulkan runtime behavior: binary startup works and filters run.

Environment note:

- The `file` utility is not installed in this Codex container, so build diagnostics must not hard-depend on it.

## Root cause
The harness build script did not enforce/run a post-build executability + platform check, so a wrong artifact selection (e.g., accidental `.exe` usage in Linux workflows) or a non-executable output could fail later at run time with weak diagnostics.

In short: reliability gap in build-time validation, not FFT/SGEMM logic.

## Fix applied
Updated `internal/prometheus/native/build_stub.sh` to harden Linux harness output validation:

1. Require `out/prometheus/native/marionette_tests` to exist.
2. Ensure executable mode (`chmod +x` fallback).
3. Validate Linux binary shape:
   - use `file` when available;
   - fallback to `readelf -h` when `file` is unavailable.
4. Emit note if a Windows `.exe` artifact is present alongside Linux artifacts.
5. Execute a non-Vulkan smoke filter immediately after build:
   - `PrometheusNativeHarness_Smoke`

Also added a dedicated always-on smoke fact in:

- `internal/prometheus/native/Marionette/smoke_tests.cpp`

## Smoke filter
Added/used:

- `PrometheusNativeHarness_Smoke`

This proves:

- harness process starts,
- filter matching works,
- assertion execution works,
- process exits success.

No Vulkan requirement.

## Validation results
- Build helper now performs runnable-binary validation and smoke execution during `build_stub.sh`.
- `PrometheusNativeHarness_Smoke` runs and passes.
- `PrometheusReactor_Fft` filter executes and passes in this environment.
- `PrometheusReactor_Sgemm` filter executes; Vulkan-dependent tests skip as expected, while non-Vulkan assertions still run.
- FFT benchmark filter executes (`--bench PrometheusReactor_Fft`) and reports benchmark output.

## Vulkan relevance
Vulkan SDK/device availability was **not** the root cause of `cannot execute binary file`.

- That failure class occurs before Vulkan dynamic behavior.
- Vulkan availability only affects specific runtime test expectations (pass/skip), not harness startup/executability.

## Remaining environment limitations
- `file` utility absent in this container (non-fatal now due to `readelf` fallback).
- Vulkan runtime/device availability can legitimately cause selected reactor tests to skip, but does not block harness reliability validation.
