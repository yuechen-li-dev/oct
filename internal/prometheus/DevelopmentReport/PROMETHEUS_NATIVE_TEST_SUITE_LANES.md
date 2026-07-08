# Prometheus Native Test Suite Lanes

## Audit summary
- Split Marionette execution into normal, slow/stress, and benchmark lanes.
- Kept semantic coverage; only changed lane membership.

## Lane definitions
- `marionette_tests`: default correctness lane; excludes slow and benchmark facts.
- `marionette_slow_tests`: includes slow/stress cases plus normal correctness tests.
- `marionette_benchmarks`: validated benchmark lane driven by explicit benchmark-category registration rather than hardcoded name prefixes.

## Files/tests moved
- Guarded slow tests in `reactor_p11_m6_batch_tests.cpp` with `MARIONETTE_EXCLUDE_SLOW_TESTS`.
- Extracted M20 drain-timeout matrix case into `PrometheusReactor_P11_M20_FailureMatrix_DrainTimeoutSlowCase`.
- Guarded benchmark-harness tests in `reactor_p13_m4_occupancy_benchmark_tests.cpp` with `MARIONETTE_EXCLUDE_BENCHMARK_TESTS`.

## Intentionally kept in normal suite
- `PrometheusReactor_P11_M20_WorkloadChurnMatrix` retained in normal lane (moderate workload; not pathological).

## Build outputs / commands
- `out/prometheus/native/marionette_tests`
- `out/prometheus/native/marionette_slow_tests`
- `out/prometheus/native/marionette_benchmarks`

## Linux/Windows build changes
- Linux script now builds all three binaries.
- Windows script TODO: align lane outputs/macros with Linux script.

## Validation results
- Built and executed all three binaries in Linux environment.

## Remaining cleanup TODOs
- Complete Windows lane parity in `build_windows.cmd`.
