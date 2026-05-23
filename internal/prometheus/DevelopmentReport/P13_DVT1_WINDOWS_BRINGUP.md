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

## 7. DVT-1 Follow-up Hygiene

### Slow lane fix

`marionette_slow_tests.exe` was corrected to behave as a real slow/stress lane instead of reporting `0` tests.

- `reactor_p11_m6_batch_tests.cpp` is now compiled into the slow-lane build path instead of being omitted from Windows entirely.
- The Windows and Linux build helpers both keep that batch-test source out of the default and benchmark binaries, preserving the original normal-lane coverage shape.
- `test_main_slow.cpp` now defaults to the explicit full slow-test names and also accepts an optional override filter for direct debugging.
- The slow lane remains separate from the normal lane; these tests were not moved back into `marionette_tests.exe`.

Validated slow coverage:

- `PrometheusReactor_P11_M11_OutOfOrderCompletionStillCommitsInEntryOrder`
- `PrometheusReactor_P11_M17_DrainTimeoutMarksUnsafeToReuse`
- `PrometheusReactor_P11_M20_FailureMatrix_DrainTimeoutSlowCase`

Observed Windows result:

- `3` tests run
- `3` passed
- `0` skipped
- `0` failed

### Benchmark registry decision

The benchmark discrepancy was documented rather than reworked.

- Supported P13 benchmark smoke command: `out\prometheus\native\marionette_benchmarks.exe`
- `out\prometheus\native\marionette_tests.exe --bench` still reports `0 benchmark(s)` because the current repository shape uses FACT-based benchmark smoke tests rather than registered `BENCHMARK(...)` entries.

This distinction is now documented in `internal/prometheus/native/README.md` so DVT-2 does not mistake the empty generic benchmark registry for a bring-up regression.

### Warning cleanup

Low-risk MSVC warning cleanup was completed without broad refactoring.

- `reactor_vulkan_sgemm.c`
  - initialized several local size/copy variables that MSVC warned could be used uninitialized
  - added one explicit cast for the signed/unsigned comparison around `mode` tracking
- `reactor_p11_m6_batch_tests.cpp`
  - added an explicit unsigned cast on the wrong-owner test flag assignment

Result:

- the prior `reactor_vulkan_sgemm.c` warnings observed during DVT-1 are cleared in the current Windows build
- no new warnings were introduced by the follow-up changes
- `internal/prometheus/native/build_stub.sh` was also normalized back to LF line endings so Bash can execute it directly during Linux parity checks

### Windows build docs

Windows is now documented as a first-class native build path in `internal/prometheus/native/README.md`.

Documented commands:

- Linux: `bash internal/prometheus/native/build_stub.sh`
- Windows: `internal\prometheus\native\build_windows.cmd`

Documented Windows outputs:

- `marionette_tests.exe`
- `marionette_slow_tests.exe`
- `marionette_benchmarks.exe`

### Validation commands and results

Windows follow-up validation:

```bat
internal\prometheus\native\build_windows.cmd
out\prometheus\native\marionette_tests.exe
out\prometheus\native\marionette_slow_tests.exe
out\prometheus\native\marionette_benchmarks.exe
out\prometheus\native\marionette_tests.exe PrometheusReactor_Sgemm
```

Observed results:

- build succeeded
- `marionette_tests.exe` passed
- `marionette_slow_tests.exe` ran `3` tests and passed
- `marionette_benchmarks.exe` passed
- targeted SGEMM tests passed
- `marionette_tests.exe --bench` still reports `Benchmark Summary: 0 benchmark(s)` as documented

Linux parity validation was also rerun after the shared test-entry changes:

```bash
bash internal/prometheus/native/build_stub.sh
out/prometheus/native/marionette_tests
out/prometheus/native/marionette_slow_tests
out/prometheus/native/marionette_benchmarks
```

Observed Linux parity result:

- `build_stub.sh` builds successfully again under Bash
- `marionette_slow_tests` passed (`3` tests)
- `marionette_benchmarks` passed (`14` tests)
- `marionette_tests` still reports two runtime failures under the current WSL Mesa `dzn` Vulkan path:
  - `PrometheusReactor_AsyncUseBeforeCompleteAndDoubleConsumeAreRejected`
  - `PrometheusReactor_M29_FixedDouble_CleanupRejectsInflightOwnership`

These Linux failures were surfaced during parity rerun and were not introduced by the Windows follow-up changes; the Windows RTX 3070 DVT path remains clean.

### Minimal sanity artifact

The existing P13 smoke lane already emits a lightweight artifact at:

- `out\test-artifacts\P13_M4_ArtifactSchemaFieldsPresent\p13_m4_smoke_artifact.txt`

This remains the most convenient sanity artifact for DVT-1/DVT-2 handoff because it captures the benchmark-smoke metadata shape without making any performance claim.

### Remaining TODOs

- `marionette_tests.exe --bench` still intentionally reports an empty generic benchmark registry; use `marionette_benchmarks.exe` for the supported P13 smoke lane.
- Machine-local Vulkan loader warnings from installed third-party overlay layers remain visible in `vulkaninfo --summary`, but they did not block runtime creation or device enumeration during DVT-1.
