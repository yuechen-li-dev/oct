# P13 DVT-1 Windows Bring-Up

## 1. System configuration

- OS: Windows 10 Pro, version 2009, build 26200, HAL `10.0.26100.1`
- Primary validation GPU: NVIDIA GeForce RTX 3070
- NVIDIA display driver: `32.0.15.9636`
- Vulkan-reported NVIDIA driver: `596.36`
- Secondary GPU present: AMD Radeon(TM) Graphics
- Vulkan SDK: `C:\VulkanSDK\1.4.341.1`
- Vulkan instance version from `vulkaninfo --summary`: `1.4.341`

`vulkaninfo --summary` on the validation machine confirmed:

- `vkCreateInstance` path works
- physical devices enumerate successfully
- `GPU0` is `NVIDIA GeForce RTX 3070`
- Vulkan device type is `PHYSICAL_DEVICE_TYPE_DISCRETE_GPU`

## 2. Build steps

### Commands used

From repository root:

```bat
internal\prometheus\native\build_windows.cmd
out\prometheus\native\marionette_tests.exe
out\prometheus\native\marionette_benchmarks.exe
out\prometheus\native\marionette_tests.exe --bench
```

Additional environment validation command:

```bat
vulkaninfo --summary
```

### Changes required

`internal/prometheus/native/build_windows.cmd` was upgraded from a developer-shell-only helper into a repeatable Windows bring-up path that:

- auto-initializes the Visual Studio developer environment when `cl` is not already on `PATH`
- compiles shared reactor C sources once as C objects
- links those objects into:
  - `prometheus_reactor.dll`
  - `marionette_tests.exe`
  - `marionette_slow_tests.exe`
  - `marionette_benchmarks.exe`
- passes a Windows-safe `MARIONETTE_TEST_REPO_ROOT`
- includes the P13 occupancy benchmark test source in the Windows build

This matches the Linux helper more closely and avoids recompiling C sources as C++, which was the first Windows bring-up failure.

## 3. Runtime results

### Default test suite

Command:

```bat
out\prometheus\native\marionette_tests.exe
```

Observed result:

- `166` tests run
- `162` passed
- `4` skipped
- `0` failed

Skips were expected capability-dependent FP16 transition checks:

- `PrometheusReactor_BufferReuseSafety_BaselineThenFP16SameShape`
- `PrometheusReactor_BufferReuseSafety_FP16ThenBaselineSameShape`
- `PrometheusReactor_BufferReuseSafety_FP16ThenPacked4SameShape`
- `SmokeFactCanBeSkipped`

### DVT benchmark smoke binary

Command:

```bat
out\prometheus\native\marionette_benchmarks.exe
```

Observed result:

- `14` tests run
- `14` passed
- `0` failed

This validated the P13 M4/M5 benchmark smoke lane on Windows hardware, including correctness and artifact schema checks.

### Marionette benchmark registry

Command:

```bat
out\prometheus\native\marionette_tests.exe --bench
```

Observed result:

- `Benchmark Summary: 0 benchmark(s)`

This is not a bring-up failure. In this repository, `marionette_benchmarks.exe` currently runs FACT-based P13 smoke validation rather than registered `BENCHMARK(...)` entries.

### SGEMM / shader / pipeline bring-up

Validation on the RTX 3070 exercised the real hardware path without crash and confirmed:

- runtime creation succeeds
- shader module creation succeeds for the embedded shader families used by the runtime
- compute pipeline creation succeeds
- SGEMM baseline path matches the CPU oracle within the repository’s established tolerance conventions
- packed4 path executes on hardware and matches the CPU oracle within tolerance
- async and transfer-queue paths execute without crash after Windows-hardware stabilization
- P13 occupancy diagnostics remain populated on hardware

## 4. Issues encountered

### Build issues

1. Windows helper required a pre-opened VS developer shell.
   - Fix: auto-call `VsDevCmd.bat` when `cl` is not on `PATH`.

2. Windows helper only built `marionette_tests.exe`.
   - Fix: extend helper to also build `marionette_slow_tests.exe` and `marionette_benchmarks.exe`.

3. Windows helper compiled reactor `.c` files again during the C++ test build.
   - Result: MSVC failed on C-only constructs in `reactor_vulkan_sgemm.c`.
   - Fix: compile shared reactor sources once as C objects and link those objects into the C++ binaries.

4. Windows helper passed object files through `cl` in a way that made them look like C++ source inputs.
   - Fix: move shared objects to the `/link` section.

5. Windows helper passed a backslash-heavy repo-root macro that broke C++ string literal parsing.
   - Fix: normalize the define to forward slashes.

### Runtime / hardware issues

1. Async task ids started at `0`, while the test lane expected a non-zero issued task id.
   - Fix: initialize the runtime counter so the first issued async task id is positive.

2. Some async rejection paths updated local slot diagnostics but did not immediately publish them through the blackboard-visible runtime diagnostic snapshot.
   - Fix: add explicit slot runtime diagnostic snapshot staging/commit on those rejection paths.

3. Real GPU packed4 execution produced tiny floating-point differences versus exact CPU scalar results.
   - Fix: align those hardware-facing packed4 tests with the repository’s existing tolerance-based CPU-oracle correctness style instead of exact bit-equality.

4. Some async cleanup tests assumed immediate readiness/abandon behavior that is not stable on a real GPU submission path.
   - Fix: poll for explicit ready state before asserting abandon success in the hardware-facing tests.

### Documentation / test-shape inconsistency surfaced

There is a repo-local inconsistency worth keeping visible:

- `marionette_benchmarks.exe` is the requested DVT benchmark binary and passes its P13 smoke lane
- but the generic Marionette benchmark registry currently has `0` registered `BENCHMARK(...)` entries

This is a test-harness/documentation gap, not a Windows bring-up failure.

## 5. Current limitations

- `marionette_slow_tests.exe` currently reports `0` tests with the current filter in `test_main_slow.cpp`.
- MSVC still emits existing warnings in `reactor_vulkan_sgemm.c`:
  - signed/unsigned mismatch
  - several potentially uninitialized local-variable warnings
- `vulkaninfo --summary` reports loader warnings from third-party overlay layers already installed on the machine.
- The generic Marionette `--bench` registry is empty even though the P13 benchmark smoke executable is valid and passing.

## 6. Readiness for DVT-2

DVT-1 exit state: **Success**

Readiness statement:

- Windows build succeeds locally on the validation machine
- `marionette_tests.exe` passes on real RTX 3070 hardware
- `marionette_benchmarks.exe` passes on real RTX 3070 hardware
- SGEMM baseline correctness is validated on hardware
- packed4 and async/transfer hardware bring-up issues encountered during Windows validation were resolved without controller changes or variant recipe changes
- occupancy-variant diagnostics remain visible on hardware

The system is ready to proceed to DVT-2 full validation and reporting.
