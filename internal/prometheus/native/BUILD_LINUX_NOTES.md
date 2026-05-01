# Prometheus Native Build Linux Notes

## Root cause of the observed failure

The `build_stub.sh` failure seen in the prior run was **not** caused by a missing `file` host tool inside the build script.

What happened:

1. `build_stub.sh` failed during Marionette C++ compilation in `reactor_p13_m4_occupancy_benchmark_tests.cpp` due to stale field references (`selected_variant`) against a struct that now uses `selector_recommended_variant`.
2. The script was run in non-verbose mode, so compiler stderr was redirected to `/dev/null` and the true compile error was hidden.
3. A separate follow-up shell check attempted to run `file out/prometheus/native/marionette_tests`; this environment has no `file` utility installed, producing:
   - `/bin/bash: line 1: file: command not found`

So the missing `file` message was a **post-failure diagnostic command**, not the build root cause.

## Where `file` is invoked

- `internal/prometheus/native/build_stub.sh`: **no invocation** of `file`.
- `internal/prometheus/native/build_windows.cmd`: **no invocation** of `file`.

`file` was only invoked externally in ad-hoc shell diagnostics.

## Is `file` required?

`file` is **optional/diagnostic-only** for this repo’s Prometheus native build flow. The Linux build script does not need it for correctness.

## Why missing `file` appeared to abort progress

The build already exited non-zero earlier because of C++ compile errors under `set -euo pipefail`. After that, a separate command attempted to inspect output binaries with `file`; because `file` was missing, it added a second error message that looked related but was not causal.

## Fix applied

Fixed the real regression by updating `reactor_p13_m4_occupancy_benchmark_tests.cpp` to use the current `CaseResult` field name (`selector_recommended_variant`) consistently.

No runtime behavior or controller logic changed; this was a test/build consistency repair.

## Host-tool assumptions (Linux path)

Current Linux helper (`build_stub.sh`) assumes availability of:

- required: `bash`, `cc`, `c++`, `find`, `sort`, Vulkan link libs (`-lvulkan`), libc toolchain basics.
- not used by this script: `file`, `realpath`, `readlink`, `objdump`, `ldd`, `pkg-config`, `glslangValidator`, `cmake`, `ninja`.

Other project workflows may use some of those tools, but this specific helper does not currently require them.

## Validation results

Validated on Linux/Codex environment:

- `bash internal/prometheus/native/build_stub.sh` -> success; emits:
  - `out/prometheus/native/marionette_tests`
  - `out/prometheus/native/marionette_slow_tests`
  - `out/prometheus/native/marionette_benchmarks`
- `out/prometheus/native/marionette_tests ResourceLease` -> pass
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm` -> pass
- `out/prometheus/native/marionette_benchmarks` -> pass
- `go test ./...` -> pass

## Remaining caveats

- Non-verbose mode in `build_stub.sh` suppresses compiler stderr for cleaner logs, which can hide root-cause diagnostics during failures. Use `VERBOSE=1` to surface compiler output when debugging.
