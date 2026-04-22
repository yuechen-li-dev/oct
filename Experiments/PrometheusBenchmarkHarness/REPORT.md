# Prometheus Benchmark Harness

This experiment is the official `[Benchmark]` authoring surface for the
Windows-native Prometheus characterization baseline.

## Current M0 corpus

`M0` now contains paired CPU and Prometheus benchmark cases for:

- tiny: `1x1x1`, `2x2x2`, `4x4x4`, `8x8x8`
- awkward: `3x5x7`, `7x3x11`, `5x17x9`
- medium square: `16x16x16`, `32x32x32`, `64x64x64`
- medium rectangular: `64x16x64`, `16x64x64`, `64x64x16`
- larger sanity: `128x128x128`

Each shape is authored twice through the same official surface:

- `matrix_mul_cpu_reference.octest`
- `matrix_mul_prometheus.octest`

Case names encode the shape directly, for example:

- `Main.MatrixMulCPUReference_M064_N064_K064`
- `Main.MatrixMulPrometheus_M064_N064_K064`

## Harness reporting

Benchmark `.octagon` artifacts preserve per-case:

- `Name`
- `DurationNs`
- `BackendRequested`
- `BackendUsed`
- `Status`
- `Environment`
- `ReportedWallNs`

`DurationNs` is the compiled benchmark process runtime.

`ReportedWallNs` is the inner Prometheus `wall=` value when the benchmark emits
one; CPU cases keep it at `0`.

## Current M1 validation milestone

`M1` is the real-hardware validation milestone for the current reactor stack.

It keeps the benchmark surface deliberately small and focuses on:

- tiny direct baseline correctness
- forced direct+tiled reachability
- forced staged baseline reachability
- forced staged+tiled reachability
- non-multiple staged+tiled correctness
- rectangular staged+tiled correctness
- natural auto-selected staged+tiled reachability

Async validation is intentionally narrow and is driven by the repo-local M1
runner rather than by trying to encode the full async lifecycle inside ordinary
`[Benchmark]` functions.

## Characterization runner

Use the repo-local Windows runner:

`tools/prometheus/run_p6c_windows_native.ps1`

For M1 real-hardware validation use:

`tools/prometheus/run_m1_windows_native.ps1`

That runner:

- sets `CGO_ENABLED=1`
- configures a repo-local MinGW toolchain when needed
- points `OCT_PROMETHEUS_REACTOR` at the Windows reactor DLL
- performs warmup runs plus repeated measured runs
- stores per-run `.octagon` artifacts under `out/prometheus/native/p6c/`
- writes aggregated summaries to:
- `out/prometheus/native/p6c/summary.json`
- `out/prometheus/native/p6c/summary.md`

The M1 runner writes structured validation artifacts under:

- `out/prometheus/benchmark_harness_m1/`

## Primary report

See [docs/reports/prometheus/P6C_REPORT.md](/C:/Users/yuech/source/repos/oct/docs/reports/prometheus/P6C_REPORT.md)
for the measured Windows-native baseline, truth status, and the recommended
next optimization target.
