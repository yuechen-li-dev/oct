# `oct make` status and parking notes (MAKE5c)

This note parks the MAKE0-MAKE5 line of work. The current surface is usable M0 infrastructure for explicit build orchestration, but it is intentionally not a replacement for mature native build systems yet.

## Current implemented capabilities

- `oct make` command exists.
- `Make.oct` discovery and `--file` selection are supported.
- Build scripts expose `Plan() -> Make.Plan`.
- The direct backend is the only implemented executor backend.
- `CommandTarget` can run declared shell commands.
- `FunctionTarget` can call Oct build functions.
- `FlowTarget` can call Octomata flows through the direct backend.
- `PhonyTarget` can group dependencies without producing an artifact.
- `Make.Config` can configure plan behavior.
- `Staleness.Timestamp` is available for timestamp-based target decisions.
- `Staleness.Always` is available for always-run targets.
- `StateDir` is supported for build state placement.
- `state.octagon` records make state.
- `trace.octagon` records make trace data when tracing is requested.
- The `Make` host capability sidecar exists.
- `oct make` automatically grants make authority to the make host capability sidecar for the make invocation.
- `Make.octest` is the convention for pure plan/config testing.
- Prometheus has a dogfood `Make.oct` / `Make.octest` plan.
- Prometheus real dogfood remains opt-in and is not a default CI or release gate.

## Current known limitations

- No Ninja backend.
- No manifest `Build` section.
- Typed C/C++ direct-backend lowering now exists as MAKE-PROD-M0 work; real
  Prometheus package-root/bootstrap and Windows validation remain pending.
- No typed Go helpers.
- No C ABI artifact model yet.
- No hash-based cache/staleness.
- No parallel jobs.
- No persistent/resumable `FlowTarget` checkpointing.
- `FlowTarget` support is direct-backend only.
- `FlowTarget` support uses the current zero-argument flow plus `Int` result convention.
- `oct make` does not automatically execute `Make.octest` tests.
- Make host capabilities require sidecar build/discovery.
- Prometheus dogfood native execution is manual/opt-in.
- Linux software Vulkan setup is not repo-owned yet.
- Windows NVIDIA hardware lane is manual/opt-in.

## How to smoke current surface

Build sidecars explicitly:

```sh
go run ./tools/build_sidecars --out dist/sidecars
```

List Prometheus make targets:

```sh
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file internal/prometheus/Make.oct --list
```

Dry-run the Prometheus make plan with trace output:

```sh
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file internal/prometheus/Make.oct --dry-run --trace
```

Run pure Prometheus `Make.octest` tests:

```sh
go run ./cmd/oct test internal/prometheus/Make.octest --execution interpreted
go run ./cmd/oct test internal/prometheus/Make.octest --execution compiled
```

Optional real native dogfood remains explicit because it depends on platform tooling and Vulkan availability:

```sh
OCT_DOGFOOD_PROMETHEUS=1 \
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file internal/prometheus/Make.oct TestNative --trace
```

Real native dogfood depends on platform tools/Vulkan and is not default CI.

## Recommended future stages

### MAKE6 / C ABI artifact model recon

- `CAbiLibrary`.
- `CAbiHeader`.
- Producers/consumers.
- Go/Rust/C/C++ through C ABI.

### MAKE7 / typed C/C++ helpers

- Compile C/C++.
- Static/shared libraries.
- Harness executables.

### MAKE8 / Go helper integration

- Go binary.
- cgo environment flags.
- runtime library placement.

### MAKE9 / Ninja backend

- Command/phony targets only.

### MAKE10 / compiled `Make.oct` build-script distribution

- Emit standalone builder binary.

### MAKE11 / persistent Octomata build flows

- Checkpoint/resume build workflows.
