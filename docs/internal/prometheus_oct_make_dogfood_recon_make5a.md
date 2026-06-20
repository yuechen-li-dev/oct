# MAKE5a-DOGFOOD-RECON — Prometheus native build/test dogfood audit for `oct make`

Status: reconnaissance/report only. This document does **not** implement a Prometheus `Make.oct`, native code, typed C/C++ helpers, Go helpers, Ninja backend, executor behavior, Make host capability APIs, Octomata semantics, manifest build schema, or default CI/release gates.

## Executive recommendation

Prometheus is a good first serious `oct make` dogfood target because it already has a real native layout, platform-specific build scripts, Vulkan linkage, Marionette test binaries, Windows hardware benchmark scripts, generated reactor sidecars, and stable smoke evidence. The first dogfood pass should be deliberately small:

1. Keep the existing native scripts as the source of truth for actual build commands.
2. Add a future project-local Prometheus `Make.oct` as an orchestration layer, not a native-build rewrite.
3. Make the default dogfood target `TestNative`, a phony target depending on a small direct-backend `PrometheusDogfoodFlow`.
4. Split Linux software-Vulkan and Windows/NVIDIA concerns into separate opt-in flows or platform-specific phases; do not hide platform divergence inside brittle shell strings.
5. Use `Make.octest` only for pure plan/config facts and theories. Do not run builds, sidecars, Vulkan probes, or harnesses from ordinary `oct test Make.octest`.
6. Keep dogfood execution out of default `go test ./...`; require explicit gates such as `OCT_DOGFOOD_PROMETHEUS=1`, plus a narrower Vulkan gate where needed.

## Inputs inspected

Required design context:

- `docs/internal/oct_make_design_recon_make0.md`
- `docs/internal/make_octest_testing_recon_make3a.md`

Current implementation and Prometheus areas inspected:

- `Libraries/Make`
- `cmd/octxiliary-makehost`
- `internal/makecmd/makecmd.go`
- `internal/prometheus/native`
- `internal/prometheus/native/Marionette`
- `internal/prometheus/reactor`
- `internal/prometheus/DevelopmentReport`
- `tools/prometheus`
- Prometheus-related Go CLI tests under `cmd/oct`
- Prometheus SGEMM/FFT experiments under `Experiments/Prometheus*`

## Part 1 — Discovered Prometheus/native layout

### 1. Prometheus source locations

Primary implementation and integration locations:

- `internal/prometheus/` — Go-side Prometheus runtime/benchmark bridge, reports, and development reports.
- `internal/prometheus/native/` — native C/Vulkan reactor source, build scripts, public C API headers, and Marionette test harness.
- `internal/prometheus/native/Marionette/` — C++20/C++23 test harness and Prometheus native test/benchmark registrations.
- `internal/prometheus/reactor/` — copied runtime sidecar location used by bridge discovery after native builds.
- `Experiments/PrometheusBenchmarkHarness/` — Oct benchmark harness experiments that can use a real Prometheus reactor when supplied.
- `Experiments/PrometheusSgemmAlgorithmLab/` — SGEMM algorithm-lab benchmark families used by Windows native scripts.
- `Experiments/PrometheusFftAlgorithmLab/` — FFT algorithm-lab artifacts and deterministic visible outputs.
- `tools/prometheus/` — Windows-native PowerShell orchestration scripts for benchmark/hardware lanes.

### 2. Native C/C++ source locations

C reactor sources live under `internal/prometheus/native/` and currently include:

- C API and bridge headers: `bridge.h`, `reactor_api.h`, `reactor_vulkan.h`.
- Public/API implementation: `reactor_api.c`.
- Vulkan/common/runtime families: `reactor_vulkan_common.c`, `reactor_vulkan_sgemm.c`, `reactor_vulkan_fft.c`, `reactor_vulkan_fused_reduction.c`.
- Dominatus/policy subsystems: `reactor_judgment_engine.c`, `reactor_dominatus_*.c`, `reactor_policy_memory.c`, `reactor_slot_hfsm.c`.
- Companion headers for each subsystem.

C++ harness sources live under `internal/prometheus/native/Marionette/` and include:

- Harness core: `test_harness.cpp`, `test_harness.h`, `test_doom.cpp`, `test_doom.h`.
- Entrypoints: `test_main.cpp`, `test_main_slow.cpp`, `test_main_benchmarks.cpp`.
- Smoke and self-tests: `smoke_tests.cpp`, `test_harness_phase1_tests.cpp`, `test_harness_doom_tests.cpp`.
- Prometheus tests/benchmarks: `reactor_*_tests.cpp` and `reactor_fft_benchmarks.cpp`.

### 3. Vulkan/shader asset locations

There are no standalone `.glsl`, `.comp`, `.vert`, `.frag`, or `.spv` files discovered in the repo. Current shader/kernel evidence is embedded as generated C headers in `internal/prometheus/native/`:

- `reactor_vulkan_tiled_spirv.h`
- `reactor_vulkan_packed4_spirv.h`
- `reactor_vulkan_fp16_spirv.h`
- `reactor_vulkan_b2x2_row_major_biased_spirv.h`
- `reactor_vulkan_a2x4_row_biased_accum8_spirv.h`
- `reactor_vulkan_srt_2accum_k_spirv.h`

Future `Make.VulkanShaderCompile` helper evidence exists, but the current build does not compile shader files from source.

### 4. Existing scripts/commands used to build

Linux:

```bash
bash internal/prometheus/native/build_stub.sh
VERBOSE=1 bash internal/prometheus/native/build_stub.sh
```

Windows:

```bat
internal\prometheus\native\build_windows.cmd
```

Windows benchmark/orchestration scripts:

```powershell
pwsh -File tools/prometheus/run_p6c_windows_native.ps1
pwsh -File tools/prometheus/run_m1_windows_native.ps1
pwsh -File tools/prometheus/run_m4d_windows_native.ps1
pwsh -File tools/prometheus/run_m6_windows_native.ps1
```

The PowerShell scripts invoke the Oct CLI benchmark commands after a native reactor DLL is available; they are not the primary DLL compiler script.

### 5. Existing scripts/commands used to run tests/harnesses

Linux build helper validates the main harness by running:

```bash
out/prometheus/native/marionette_tests PrometheusNativeHarness_Smoke
```

Manual harness commands documented by Marionette are:

```bash
out/prometheus/native/marionette_tests
out/prometheus/native/marionette_tests <filter>
out/prometheus/native/marionette_tests --bench
out/prometheus/native/marionette_tests --bench <filter>
```

Expected Windows equivalents after `build_windows.cmd` are:

```powershell
.\out\prometheus\native\marionette_tests.exe
.\out\prometheus\native\marionette_tests.exe PrometheusNativeHarness_Smoke
.\out\prometheus\native\marionette_benchmarks.exe
```

Oct/Go integration lanes also reference:

```bash
go test ./cmd/oct -run TestPrometheusSgemmCPUScenarioEmitsOctagonReport
go test ./cmd/oct -run TestWindowsBenchUsesPrometheusBackendAndWritesOctagon
```

The Windows Prometheus integration tests are gated and require `OCT_RUN_PROMETHEUS_INTEGRATION=1` plus `OCT_PROMETHEUS_REACTOR=<path-to-prometheus_reactor.dll>`.

### 6. Existing software Vulkan setup path for Linux/Codex Cloud

No dedicated, checked-in Prometheus software-Vulkan setup script was found. Linux build commands assume usable C/C++ tools plus linkable Vulkan loader (`-lvulkan`). A future dogfood lane should therefore distinguish:

- `CheckTools` — safe inspection of `cc`, `c++`, `bash`, `file`/`readelf`, and Vulkan loader/header availability.
- `SetupVulkanLinux` — explicit, opt-in setup or instruction target, not part of default `TestNative`.

Candidate Linux software Vulkan dependencies to document later are likely Mesa/lavapipe and Vulkan loader/dev packages, but exact install commands should not be baked into `Make.oct` until verified in the target Codex image.

### 7. Existing Windows/NVIDIA path

Windows native path is represented by:

- `internal/prometheus/native/build_windows.cmd` for MSVC/Vulkan SDK native compilation.
- `tools/prometheus/run_*_windows_native.ps1` for hardware-backed benchmark orchestration.
- Go integration tests requiring `OCT_RUN_PROMETHEUS_INTEGRATION=1` and `OCT_PROMETHEUS_REACTOR`.

The scripts assert or report the `windows_native_vulkan` environment when a real Prometheus backend is used. They assume a Windows host with MSVC developer tools, Vulkan SDK/import library, and real NVIDIA Vulkan hardware/driver for the serious hardware path.

### 8. Existing generated outputs/artifacts

Generated build outputs include:

- Linux: `out/prometheus/native/libprometheus_reactor.so`, `out/prometheus/native/marionette_tests`, `out/prometheus/native/marionette_slow_tests`, `out/prometheus/native/marionette_benchmarks`, and object files under `out/prometheus/native/obj/`.
- Windows: `out/prometheus/native/prometheus_reactor.dll`, `.lib`, `.pdb`, `marionette_tests.exe`, `marionette_slow_tests.exe`, `marionette_benchmarks.exe`, and object/PDB files.
- Bridge discovery copies: `internal/prometheus/reactor/libprometheus_reactor.so` on Linux and `internal/prometheus/reactor/prometheus_reactor.dll` on Windows.
- Harness artifacts: `out/test-artifacts/<TestName>/`.
- PowerShell benchmark outputs: subdirectories under `out/prometheus/`, including native P6c, benchmark harness M1, SGEMM lab M4d, and M6a summaries/artifacts.
- Octagon reports from CLI/benchmark lanes when `--octagon-out` is used.

### 9. Existing logs/results stable enough to assert

Stable assertions already visible in code/docs:

- Linux build helper emits `Build complete.` on success.
- Linux build helper prints `Built reactor library: ...libprometheus_reactor.so` and `Built Marionette tests: ...marionette_tests`.
- Linux build helper runs `PrometheusNativeHarness_Smoke` as a smoke filter.
- Windows build helper prints built reactor and Marionette output paths.
- Prometheus CLI fallback path reports `backend_requested=prometheus backend_used=cpu status=fallback(prometheus_unavailable)` when no real reactor is available.
- Windows hardware integration path expects `backend_requested=prometheus backend_used=prometheus status=ok` and `vulkan_env=windows_native_vulkan` when a real reactor is supplied.

Avoid asserting exact compiler banners, GPU device names, benchmark timings, driver ordering, or large stdout/stderr blocks.

## Part 2 — Current manual build/test paths

### Linux/Codex Cloud path

#### Tool dependencies

Minimum inferred from scripts:

- `bash`
- POSIX utilities: `find`, `sort`, `cp`, `mkdir`, `chmod`
- C compiler available as `cc`
- C++ compiler available as `c++` with C++23 support for the full current helper
- `libm`, `pthread`
- Vulkan loader/development library linkable as `-lvulkan`
- `file` preferred, with `readelf` fallback

Likely software Vulkan runtime for headless Codex/Linux:

- Vulkan loader runtime
- Mesa Vulkan driver/lavapipe or equivalent software Vulkan ICD
- `vulkaninfo` optional diagnostic tool

The exact apt command is not yet documented in this repo and should remain an explicit setup/manual lane until verified.

#### Software Vulkan setup

No repo-owned software Vulkan setup script was found for Prometheus. The future `SetupVulkanLinux` target should initially be a diagnostic/instruction target or gated setup target under `OCT_PROMETHEUS_SOFTWARE_VULKAN=1`, not an automatic dependency of the default lane.

#### Build commands

```bash
bash internal/prometheus/native/build_stub.sh
```

Verbose diagnostics:

```bash
VERBOSE=1 bash internal/prometheus/native/build_stub.sh
```

#### Test/harness commands

Smoke during build:

```bash
out/prometheus/native/marionette_tests PrometheusNativeHarness_Smoke
```

Manual correctness/harness:

```bash
out/prometheus/native/marionette_tests
out/prometheus/native/marionette_slow_tests
out/prometheus/native/marionette_benchmarks
```

The slow and benchmark binaries should be opt-in, not default.

#### Expected outputs/logs

- `out/prometheus/native/libprometheus_reactor.so`
- `internal/prometheus/reactor/libprometheus_reactor.so`
- `out/prometheus/native/marionette_tests`
- `out/prometheus/native/marionette_slow_tests`
- `out/prometheus/native/marionette_benchmarks`
- `Build complete.`
- Smoke filter process exit `0`

#### Failure modes

- `cc`/`c++` missing.
- C++ compiler lacks the required C++23 support for current script.
- Vulkan development library missing, causing `-lvulkan` link failure.
- Vulkan headers missing if included through system paths.
- Runtime Vulkan ICD missing, causing harness tests that touch Vulkan to skip or fail.
- `file` and `readelf` both missing, blocking ELF validation fallback.
- Generated/copied sidecar location under `internal/prometheus/reactor/` may dirty the worktree if tracked/unignored paths change.

### Windows/NVIDIA path

#### PowerShell commands

Native benchmark orchestration scripts:

```powershell
pwsh -File tools/prometheus/run_p6c_windows_native.ps1
pwsh -File tools/prometheus/run_m1_windows_native.ps1
pwsh -File tools/prometheus/run_m4d_windows_native.ps1
pwsh -File tools/prometheus/run_m6_windows_native.ps1
```

Exact flags/parameters should be read from each script before automation. The M0 dogfood plan should not guess hidden parameters.

#### Tool dependencies

- Windows host.
- MSVC `cl` and `link`; `build_windows.cmd` attempts a hardcoded Visual Studio developer command path if `cl` is absent.
- Vulkan SDK with `Include\vulkan\vulkan.h` and `Lib\vulkan-1.lib`, or an environment where `vulkan-1.lib` resolves.
- PowerShell (`pwsh`) for orchestration scripts.
- Oct CLI binary available on PATH or passed through script parameters where supported.
- NVIDIA Vulkan driver/hardware for the hardware-backed benchmark lanes.

#### Vulkan/NVIDIA assumptions

The serious Windows path expects a real native Vulkan environment and reports `windows_native_vulkan` when Prometheus is actually used. It should be manual or hardware-tagged CI only.

#### Build commands

```bat
internal\prometheus\native\build_windows.cmd
```

#### Test/harness commands

```powershell
.\out\prometheus\native\marionette_tests.exe
.\out\prometheus\native\marionette_tests.exe PrometheusNativeHarness_Smoke
.\out\prometheus\native\marionette_benchmarks.exe
```

Go/Oct hardware integration after a reactor is built:

```powershell
$env:OCT_RUN_PROMETHEUS_INTEGRATION = "1"
$env:OCT_PROMETHEUS_REACTOR = "<repo>\out\prometheus\native\prometheus_reactor.dll"
go test .\cmd\oct -run TestWindowsBenchUsesPrometheusBackendAndWritesOctagon
```

#### Expected outputs/logs

- `out\prometheus\native\prometheus_reactor.dll`
- `out\prometheus\native\prometheus_reactor.lib`
- `out\prometheus\native\marionette_tests.exe`
- `out\prometheus\native\marionette_slow_tests.exe`
- `out\prometheus\native\marionette_benchmarks.exe`
- `internal\prometheus\reactor\prometheus_reactor.dll`
- PowerShell markdown/Octagon summaries under `out\prometheus\...`
- Real backend evidence: `backend_requested=prometheus backend_used=prometheus status=ok` and `vulkan_env=windows_native_vulkan`.

#### Failure modes

- `cl` not on PATH and hardcoded Visual Studio path absent or stale.
- Vulkan SDK absent or `VULKAN_SDK` points to an incomplete install.
- `vulkan-1.lib` link failure.
- NVIDIA/Vulkan hardware unavailable, leading to fallback or failure in hardware lanes.
- PowerShell quoting/path differences, especially when repo path contains spaces.
- Scripts assume specific output locations and may overwrite/copy native sidecars.

## Part 3 — Proposed `oct make` dogfood targets

### Minimal M0 target set

| Target | Kind | Purpose | M0? | Notes |
| --- | --- | --- | --- | --- |
| `ListEnvironment` | `FunctionTarget` | Print/record OS, relevant env vars, repo-relative paths, and tool probes through `Make.Tool`/`Make.Exists`. | Yes | Safe and diagnostic; no GPU dependency. |
| `CheckTools` | `FunctionTarget` | Check `bash`/`cmd`/`pwsh`, compilers, `file`/`readelf`, Vulkan SDK/loader hints. | Yes | Should record diagnostics rather than guess setup. |
| `BuildPrometheus` | `CommandTarget` initially | Run existing platform build script. | Yes | Linux: `bash internal/prometheus/native/build_stub.sh`; Windows: `cmd /c internal\prometheus\native\build_windows.cmd` or separate Windows command target. |
| `RunHarness` | `CommandTarget` | Run smoke harness filter against built binary. | Yes | Use smoke filter only for default. Full harness can be opt-in later. |
| `PrometheusDogfoodFlow` | `FlowTarget` | Orchestrate detect/check/build/smoke with state-history trace. | Yes | Main FlowTarget dogfood evidence. |
| `TestNative` | `PhonyTarget` | Default dogfood entrypoint depending on flow. | Yes | Recommended default target. |
| `Clean` | `FunctionTarget` | Remove `.octmake/prometheus` and optionally `out/prometheus/native`. | Maybe | Keep conservative; default should not delete broad outputs. |

### Deferred or opt-in targets

| Target | Kind | Reason to defer or gate |
| --- | --- | --- |
| `SetupVulkanLinux` | `FunctionTarget` or `CommandTarget` | No repo-owned verified setup command; should require `OCT_PROMETHEUS_SOFTWARE_VULKAN=1`. |
| `BuildHarness` | `CommandTarget` | Current scripts build reactor and harness together; separate target waits for split helper or typed helpers. |
| `RunSgemmSmoke` | `CommandTarget` or `FunctionTarget` | Good next target after smoke; requires selecting stable filter/output. |
| `RunFftSmoke` | `CommandTarget` or `FunctionTarget` | FFT benchmark path is currently truthfully limited/unavailable for production; should not overclaim. |
| `All` | `PhonyTarget` | Avoid broad default until dogfood boundaries are proven. |
| Windows hardware benchmark targets | `FlowTarget` | Should be manual/hardware CI only. |

### Target-kind classification rationale

- Use `CommandTarget` for existing script invocations and harness binaries because they are stable external commands with clear inputs/outputs.
- Use `FunctionTarget` for tool probing, environment listing, conservative cleanup, and command selection logic that benefits from `Make.*` host primitives.
- Use `FlowTarget` for the dogfood lane because it needs platform detection, setup decision points, failure/fallback trace evidence, and state history.
- Use `PhonyTarget` for `TestNative` grouping.

## Part 4 — Recommended first Prometheus build flow

### One FlowTarget or multiple FunctionTargets first?

Use one small `FlowTarget` as the main dogfood lane, plus helper `FunctionTarget`s for reusable probes. The point of MAKE5 dogfood is to exercise FlowTarget trace/state evidence against a real native workflow, but the flow should not contain every platform-specific command inline.

Recommended M0 shape:

```text
PrometheusDogfoodFlow:
  DetectPlatform
  CheckTools
  SelectBuildPath
  BuildNative
  RunSmokeHarness
  Done
  Failed
```

A separate `PrometheusWindowsHardwareFlow` can be added later for NVIDIA benchmark scripts. Do not force Linux/Codex and Windows/NVIDIA into one monolithic flow until the M0 trace contract is stable.

### Recommended states

- `DetectPlatform` — classify `linux`, `windows`, or unsupported.
- `CheckTools` — verify required tool presence and record diagnostics.
- `PrepareVulkan` — Linux: record whether software Vulkan gate is enabled and whether Vulkan loader/runtime appears usable; Windows: verify `VULKAN_SDK`/library hints. In M0 this should not auto-install by default.
- `BuildNative` — run the existing platform build script.
- `RunSmokeHarness` — run `PrometheusNativeHarness_Smoke` using the platform binary name.
- `Done` — return `0`.
- `Failed` — return non-zero with recorded diagnostics.

### Stop-immediately failures

- Unsupported OS for the selected flow.
- Missing native build script.
- Missing required compiler/linker for selected platform.
- Native build command returns non-zero.
- Expected output binary/library path missing after a successful build.
- Smoke harness returns non-zero.
- Flow exceeds `MaxSteps`.

### Trace-and-continue/fallback failures

- Missing optional `file` command when `readelf` fallback exists.
- Missing `vulkaninfo` diagnostic tool.
- Linux software Vulkan setup gate not set: record `SkippedSoftwareVulkanSetup`, continue only if the smoke target does not require it or native tests self-skip safely.
- Windows hardware benchmark lane not gated: record `SkippedWindowsHardware`, do not run hardware scripts.
- Missing real `OCT_PROMETHEUS_REACTOR` for Oct benchmark integration: record fallback evidence, do not claim Prometheus backend success.

### Linux and Windows sharing

Recommendation: share pure plan/config helpers and target names, but split execution flows:

- `PrometheusLinuxDogfoodFlow` for Linux/Codex software Vulkan and smoke harness.
- `PrometheusWindowsDogfoodFlow` for Windows native build and smoke harness.
- Future `PrometheusWindowsHardwareFlow` for NVIDIA benchmark scripts.

A top-level `PrometheusDogfoodFlow` can dispatch by platform later, but separate flows make traces easier to read and keep platform failures isolated.

### `MaxSteps`

Recommended M0 `MaxSteps`: `16`.

Rationale: the M0 flow has roughly 6-8 states. `16` allows one or two diagnostic/fallback transitions without hiding accidental loops. Avoid large values until trace size and repeated-state behavior are proven.

### `Int` result codes

Use the current MAKE4 convention: `0` succeeds, non-zero fails. Suggested semantic codes for documentation:

- `0` — success.
- `10` — unsupported platform or disabled required gate.
- `20` — required tool missing.
- `30` — Vulkan setup/probe failure.
- `40` — native build failed.
- `50` — expected output missing.
- `60` — harness failed.
- `70` — trace/state write or validation problem.

### Expected state history in trace

Linux smoke happy path:

```text
DetectPlatform -> CheckTools -> PrepareVulkan -> BuildNative -> RunSmokeHarness -> Done
```

Windows smoke happy path:

```text
DetectPlatform -> CheckTools -> PrepareVulkan -> BuildNative -> RunSmokeHarness -> Done
```

Linux no-software-Vulkan-gate but smoke still self-skips/succeeds:

```text
DetectPlatform -> CheckTools -> PrepareVulkan -> BuildNative -> RunSmokeHarness -> Done
```

with explicit trace detail recording that software Vulkan setup was skipped.

Missing compiler path:

```text
DetectPlatform -> CheckTools -> Failed
```

Build failure path:

```text
DetectPlatform -> CheckTools -> PrepareVulkan -> BuildNative -> Failed
```

## Part 5 — Future pure `Make.octest` plan/config assertions

`Make.octest` should remain pure and authority-free. It should not call `Make.Exec`, run build scripts, invoke Vulkan, mutate files, or require `OCT_MAKE_AUTHORITY=1`.

Example assertion style using current `Assert.*` syntax:

```oct
package Main

import Make

[Fact]
fn PlanDefaultIsTestNative() -> Void {
    let plan = Plan()
    Assert.Equal("TestNative", plan.Default, "default dogfood target should be TestNative")
}

[Fact]
fn DogfoodConfigUsesIsolatedStateDirAndTrace() -> Void {
    let cfg = DogfoodConfig()
    Assert.Equal("PrometheusDogfood", cfg.Profile, "dogfood profile should be named")
    Assert.Equal(".octmake/prometheus", cfg.StateDir, "dogfood state must be isolated")
    Assert.True(cfg.Trace, "dogfood config should write trace by default")
}

[Fact]
fn MainDogfoodTargetIsFlow() -> Void {
    let plan = Plan()
    Assert.True(HasFlowTarget(plan, "PrometheusDogfoodFlow"), "main dogfood target should be a FlowTarget")
}

[Fact]
fn FlowTargetHasPositiveMaxSteps() -> Void {
    let flow = FindFlowTarget(Plan(), "PrometheusDogfoodFlow")
    Assert.True(flow.MaxSteps > 0, "FlowTarget MaxSteps must be positive")
}

[Fact]
fn TestNativeDependsOnDogfoodFlow() -> Void {
    let target = FindPhonyTarget(Plan(), "TestNative")
    Assert.True(Contains(target.Deps, "PrometheusDogfoodFlow"), "TestNative should depend on dogfood flow")
}

[Theory]
[InlineData("TestNative")]
[InlineData("CheckTools")]
[InlineData("BuildPrometheus")]
[InlineData("RunHarness")]
fn PlanContainsExpectedTargets(name: String) -> Void {
    Assert.True(HasAnyTarget(Plan(), name), "expected target should be present")
}

[Fact]
fn NoTargetNamesAreEmpty() -> Void {
    let plan = Plan()
    Assert.True(AllTargetNamesNonEmpty(plan), "target names must be non-empty")
}

[Fact]
fn CommandTargetsUseProgramAndArgs() -> Void {
    let plan = Plan()
    Assert.True(CommandTargetHasProgram(plan, "BuildPrometheus"), "command targets should set Program")
    Assert.True(CommandTargetArgsAreStructured(plan, "BuildPrometheus"), "command targets should use Args, not shell strings")
}
```

Recommended facts/theories:

1. Default target is `TestNative`.
2. dogfood config uses `.octmake/prometheus`.
3. trace is enabled for dogfood config.
4. plan includes `CheckTools`.
5. plan includes Linux setup/probe target.
6. plan includes a Windows/NVIDIA target or explicitly separate Windows hardware flow.
7. plan includes native harness target.
8. main dogfood target is a `FlowTarget`.
9. `FlowTarget.MaxSteps > 0` and likely equals `16` if locked.
10. `TestNative` depends on the dogfood flow.
11. no target names are empty.
12. command targets use `Program` plus structured `Args`, not one shell command string.
13. Windows paths and Linux paths are expressed as data/helper choices, not as accidental string concatenation in tests.

## Part 6 — Integration/dogfood test boundaries

### Pure `Make.octest`

Belongs here:

- `Plan()` shape.
- `Make.Config` values.
- target names, kinds, dependencies, inputs, outputs.
- platform matrix as pure data.
- `MaxSteps` and state-dir/trace defaults.

Does not belong here:

- `Make.Exec` or host primitives.
- native build scripts.
- Vulkan setup/probes that inspect the real host.
- GPU/hardware execution.

### Go CLI integration tests

Belongs here:

- `oct make --list` output includes expected targets.
- `oct make --dry-run --trace` writes valid trace for a fixture or future Prometheus plan.
- state/trace file shape for direct executor.
- authority boundary behavior.
- environment-gated invocation of Prometheus dogfood commands when explicitly enabled.

Go tests should not duplicate language semantics already expressed in `Language/` or pure `Make.octest`.

### Manual dogfood commands

Recommended commands after MAKE5b implements the future `Make.oct`:

```bash
OCT_DOGFOOD_PROMETHEUS=1 go run ./cmd/oct make TestNative --file internal/prometheus/Make.oct --trace
OCT_DOGFOOD_PROMETHEUS=1 go run ./cmd/oct make PrometheusDogfoodFlow --file internal/prometheus/Make.oct --trace
```

Linux software Vulkan setup, if added:

```bash
OCT_DOGFOOD_PROMETHEUS=1 OCT_PROMETHEUS_SOFTWARE_VULKAN=1 go run ./cmd/oct make SetupVulkanLinux --file internal/prometheus/Make.oct --trace
```

Windows hardware path:

```powershell
$env:OCT_DOGFOOD_PROMETHEUS = "1"
$env:OCT_PROMETHEUS_WINDOWS_NVIDIA = "1"
go run .\cmd\oct make PrometheusWindowsHardwareFlow --file internal\prometheus\Make.oct --trace
```

### CI opt-in jobs

Recommended gates:

```text
OCT_DOGFOOD_PROMETHEUS=1
OCT_WRAPPER_PATH=<path-to-built-sidecars>
```

For Linux software Vulkan:

```text
OCT_PROMETHEUS_SOFTWARE_VULKAN=1
```

For Windows hardware:

```text
OCT_PROMETHEUS_WINDOWS_NVIDIA=1
OCT_PROMETHEUS_REACTOR=<path-to-prometheus_reactor.dll>
```

The Prometheus dogfood lane must not become default `go test ./...`, and should not be release-mandatory without later explicit approval.

## Part 7 — Stable vs brittle dogfood assertions

### Stable assertions to prefer

- `oct make --list` includes `TestNative`, `CheckTools`, `BuildPrometheus`, `RunHarness`, and the dogfood flow.
- `oct make --dry-run --trace` writes `.octmake/prometheus/trace.octagon` or the configured trace path.
- trace selected target equals the requested target.
- trace records `Profile: "PrometheusDogfood"` and `StateDir: ".octmake/prometheus"`.
- trace includes FlowTarget state history with expected state names.
- `CheckTools` records present/missing diagnostics for each required tool.
- expected output path exists after build: shared library/DLL and smoke harness binary.
- smoke harness process exits `0`.
- generated log/Octagon/markdown summary exists when a target promises it.
- `state.octagon` records selected target `LastStatus: "Succeeded"` after success.
- intentional missing-tool fixture records `Failed` and useful error text in trace.
- Prometheus fallback CLI path truthfully reports CPU fallback when reactor is absent.

### Brittle assertions to avoid

- exact GPU device name.
- exact compiler version string.
- exact Vulkan driver/ICD enumeration order.
- exact benchmark timing.
- exact absolute paths.
- entire stdout/stderr equality.
- large trace equality including timestamps.
- expecting FFT production success before Vulkan FFT is actually implemented.
- assuming `vulkaninfo` exists on every developer host.

## Part 8 — C/C++ + Go future typed-helper implications

Prometheus provides concrete evidence for these future helpers, but MAKE5a should not implement them:

- `Make.CxxStaticLibrary` — less immediately useful than shared/dynamic library for current reactor, but useful for splitting common native objects/harness code.
- `Make.CxxExecutable` — Marionette binaries are real C++ executables built from a source set, defines, common objects, and platform linker flags.
- `Make.VulkanShaderCompile` — current SPIR-V is embedded in headers; if shader sources are restored, this helper should own glslang/dxc invocation and generated headers.
- `Make.GoBinary` — useful for building `oct` or `octxiliary-*` sidecars as dependencies of dogfood lanes, especially with `OCT_WRAPPER_PATH`.
- `Make.CAbiLibrary` — strong fit for `prometheus_reactor.dll` / `libprometheus_reactor.so` with stable C ABI and bridge discovery copies.
- `Make.CopyRuntimeLibraries` — current scripts copy built reactors into `internal/prometheus/reactor/` for bridge discovery.
- `Make.Toolchain` — needed to model MSVC vs GCC/Clang, Vulkan SDK, Windows developer shells, Linux packages, and linker/library paths without shell soup.

Evidence summary: current scripts hand-maintain source lists, compiler standards, defines, object directories, platform binary suffixes, Vulkan library names, sidecar copies, and multiple harness outputs. Those are exactly the seams typed helpers should eventually own.

## Part 9 — Risk audit and mitigations

| Risk | Evidence | Mitigation |
| --- | --- | --- |
| Platform assumptions | Linux uses `cc`/`c++`/`-lvulkan`; Windows uses MSVC/Vulkan SDK. | Split Linux and Windows flows; make `CheckTools` explicit. |
| Missing tools in Codex/Linux CI | No Prometheus setup script exists. | Add opt-in diagnostics first; gate setup with `OCT_PROMETHEUS_SOFTWARE_VULKAN=1`. |
| GPU dependency | Windows path expects native NVIDIA Vulkan for real backend success. | Keep hardware lane manual or hardware-tagged CI only. |
| Software Vulkan fragility | No verified lavapipe/Mesa path in repo. | Document setup separately; do not make default. |
| Windows path quoting | Batch/PowerShell scripts carry many quoted paths. | Use `Program` + `Args`; avoid shell-string command targets where possible. |
| PowerShell differences | `pwsh` availability and execution policy vary. | Treat PowerShell scripts as explicit opt-in targets with tool checks. |
| Shell script vs program+args mismatch | Existing Linux build is `bash script`, not typed compile graph. | M0 `CommandTarget` wraps script; later typed helpers can replace internals. |
| Outputs not stable | Native outputs under `out/` and copied reactors under `internal/prometheus/reactor/`. | Assert existence/status, not full path text; keep generated outputs uncommitted. |
| Scripts too broad/side-effectful | Build scripts compile, link, copy sidecars, and run smoke. | Dogfood starts with one flow and documents side effects; later split build/run targets. |
| Trace/stdout size | Native builds and benchmarks can be verbose. | Capture bounded diagnostics; avoid whole-output equality; prefer summary artifacts. |
| FFT overclaim | FFT runtime production path is currently limited/inert in docs/reports. | Keep `RunFftSmoke` deferred and truthfulness-focused. |
| Release creep | Dogfood could become release-mandatory accidentally. | Require explicit environment gates and later human approval for release gates. |

## Part 10 — Recommended next prompt for MAKE5b implementation

```text
You are working in the Oct repository.

Task: MAKE5b-DOGFOOD-PROMETHEUS-M0 — add an opt-in Prometheus `oct make` dogfood plan without changing native code or executor behavior.

Read first:
- docs/internal/prometheus_oct_make_dogfood_recon_make5a.md
- docs/internal/oct_make_design_recon_make0.md
- docs/internal/make_octest_testing_recon_make3a.md
- Libraries/Make/README.md
- internal/prometheus/native/README.md
- internal/prometheus/native/Marionette/README.md

Implement only the minimal M0 dogfood layer recommended by MAKE5a:
1. Add a Prometheus `Make.oct` in the smallest appropriate Prometheus project location.
2. Define pure `Plan() -> Make.Plan` with isolated config: Profile `PrometheusDogfood`, StateDir `.octmake/prometheus`, Trace `true`.
3. Add targets: `ListEnvironment`, `CheckTools`, `BuildPrometheus`, `RunHarness`, one platform-specific dogfood `FlowTarget`, `TestNative`, and conservative `Clean` only if safe.
4. Use existing scripts as commands. Do not rewrite native compilation and do not add typed C/C++ or Go helpers.
5. Add `Make.octest` pure plan/config tests only. Do not run host build commands from `Make.octest`.
6. Add or update docs with exact opt-in commands and gates: `OCT_DOGFOOD_PROMETHEUS=1`; use `OCT_PROMETHEUS_SOFTWARE_VULKAN=1` only for future Linux setup; keep Windows NVIDIA hardware manual/opt-in.
7. If adding Go integration tests, gate any real Prometheus dogfood execution so it is skipped by default and does not run under default `go test ./...` without env approval.
8. Do not modify native Prometheus code, Make executor behavior, Make host capability APIs, Octomata semantics, manifest schema, or default release gates.

Run lightweight pure tests only, such as the new `Make.octest` plan/config test and targeted `oct make --list`/`--dry-run --trace` if they do not execute native commands. Do not run native build/test commands unless explicitly gated and documented.

Commit message: add Prometheus oct make dogfood plan
```

## Final MAKE5a conclusion

This recon reaches **success** for a report-only task: the current Prometheus native build/test layout is discoverable, the existing manual Linux and Windows paths are identifiable, the absence of a verified Linux software-Vulkan setup script is explicit, and the recommended M0 dogfood target set is bounded. The next blocker is implementation of a small opt-in Prometheus `Make.oct` plus pure `Make.octest` plan/config assertions; that work is isolated in the MAKE5b prompt above.
