# MAKE5b — Prometheus `oct make` dogfood M0

Status: opt-in plan/config dogfood layer. This does not replace Prometheus native scripts, add typed C/C++ or Go helpers, add a Ninja backend, change `oct make` executor behavior, or add a default CI/release gate.

## Location

The Prometheus dogfood files live beside the Prometheus project internals:

- `internal/prometheus/Make.oct`
- `internal/prometheus/Make.octest`

This is deliberately project-local to Prometheus rather than a repository-root `Make.oct` for all Oct development.

## Plan shape

`Plan()` returns a typed `Make.Plan` whose default target is `TestNative`. The plan uses `DogfoodConfig()`:

- `Profile: "PrometheusDogfood"`
- `StateDir: ".octmake/prometheus"`
- `Trace: true`
- `Staleness: Make.Staleness.Timestamp`

M0 targets:

| Target | Kind | Role |
| --- | --- | --- |
| `ListEnvironment` | `FunctionTarget` | M0 diagnostic placeholder; real host/environment probing remains gated future work. |
| `CheckTools` | `FunctionTarget` | M0 diagnostic placeholder for tool checks. |
| `BuildPrometheus` | `CommandTarget` | Wraps the existing native build script with `Program: "bash"` and `Args: ["internal/prometheus/native/build_stub.sh"]`. |
| `RunHarness` | `CommandTarget` | Runs the existing smoke harness filter `PrometheusNativeHarness_Smoke`. |
| `PrometheusDogfoodFlow` | `FlowTarget` | No-argument Octomata flow with `MaxSteps: 16`. |
| `TestNative` | `PhonyTarget` | Default target depending on `PrometheusDogfoodFlow`. |
| `Clean` | `FunctionTarget` | Conservative placeholder; it does not remove generated Prometheus artifacts in M0. |

The M0 flow records the planned state path:

```text
DetectPlatform -> CheckTools -> PrepareVulkan -> BuildNative -> RunSmokeHarness -> Done
```

Current `FlowTarget` support can execute a no-argument flow and record state-history trace evidence. The M0 Prometheus flow is intentionally diagnostic: it does not fake command-target invocation from inside Octomata. The command targets remain present in the plan as explicit build/harness metadata and can be promoted into a fuller gated execution lane once Make host/platform APIs can express the required environment decisions without brittle shell-string wrappers.

## Gates and platform split

Real Prometheus native dogfood execution is opt-in and should be treated as manual until a later milestone wires host/platform checks into the flow:

- `OCT_DOGFOOD_PROMETHEUS=1` gates real build/test intent.
- `OCT_PROMETHEUS_SOFTWARE_VULKAN=1` is reserved for future Linux software Vulkan setup/probe work.
- `OCT_PROMETHEUS_WINDOWS_NVIDIA=1` is reserved for future/manual Windows NVIDIA hardware lanes.

Linux M0 command metadata points at the existing source-of-truth build script, `internal/prometheus/native/build_stub.sh`. Windows remains documented as a separate concern through the existing `internal\prometheus\native\build_windows.cmd` script; the single M0 `CommandTarget` record cannot express platform selection without moving logic into a shell string, so Windows/NVIDIA benchmark scripts under `tools/prometheus` are deferred and are not default dogfood commands.

`--list` and `--dry-run --trace` are safe plan-inspection paths and do not require the gates because they do not execute native commands. Ordinary `Make.octest` plan/config tests also do not require the gates, `OCT_MAKE_AUTHORITY=1`, or `OCT_WRAPPER_PATH`.

## Commands

Build sidecars when exercising `oct make` from the repository checkout:

```bash
go run ./tools/build_sidecars --out dist/sidecars
```

List Prometheus targets:

```bash
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
  go run ./cmd/oct make --file internal/prometheus/Make.oct --list
```

Dry-run the default plan with trace enabled:

```bash
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
  go run ./cmd/oct make --file internal/prometheus/Make.oct --dry-run --trace
```

Manual opt-in dogfood execution:

```bash
OCT_DOGFOOD_PROMETHEUS=1 \
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
  go run ./cmd/oct make --file internal/prometheus/Make.oct TestNative --trace
```

Windows PowerShell equivalent:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_WRAPPER_PATH="$PWD\dist\sidecars"
go run .\cmd\oct make --file internal\prometheus\Make.oct --list
go run .\cmd\oct make --file internal\prometheus\Make.oct --dry-run --trace
$env:OCT_DOGFOOD_PROMETHEUS="1"
go run .\cmd\oct make --file internal\prometheus\Make.oct TestNative --trace
```

## Pure `Make.octest` coverage

`internal/prometheus/Make.octest` is a normal xUnit-style companion. It asserts plan/config facts only:

- default target is `TestNative`;
- `DogfoodConfig()` uses the Prometheus dogfood profile, state dir, trace, and timestamp staleness;
- target counts match the M0 plan;
- flow max steps are fixed at 16;
- `TestNative` depends on `PrometheusDogfoodFlow`;
- build and harness command metadata use structured `Program` and `Args` fields.

It does not call side-effectful `Make.*` host primitives and does not run Vulkan/native build/harness commands.

## Deferred work

- Host/platform capability APIs for environment checks and platform selection.
- Typed C/C++ or Go build helpers.
- Ninja lowering for Prometheus targets.
- Linux software Vulkan installation/probe automation.
- Windows NVIDIA hardware benchmark targets.
- Slow/full benchmark lanes.
- FFT production claims beyond current Prometheus limitations.
