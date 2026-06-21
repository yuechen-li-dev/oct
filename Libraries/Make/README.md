# Make

`Make` is a first-party Octxiliary wrapper package for future `oct make` host-capability primitives. It is intentionally side-effectful and is not part of Oct's ambient scientific runtime.

The `octxiliary-makehost` sidecar requires explicit make authority (`OCT_MAKE_AUTHORITY=1` in MAKE1 tests and future `oct make` plumbing). Ordinary `oct run`/test execution that reaches these functions without authority receives a clear capability error instead of process or file access.

Process execution uses `program` plus `args`; it does not invoke a shell string. Launch failures are fallible errors. Non-zero process exits return `ProcessResult` with the non-zero `ExitCode`, captured `Stdout`, and captured `Stderr`.

File primitives are intended for project build orchestration. MAKE1 does not yet enforce project-root path policy inside the sidecar; future `oct make` will provide authority and project-root scoping. Ninja backends, C/C++ helpers, Go helpers, target DAGs, and `Make.oct` plan execution are deferred.

## C ABI artifact records

MAKE11 adds pure data records for C ABI artifact metadata. These records are plain schema values that can move through `Make.Plan` helpers and `Make.octest` assertions; they do not compile, link, copy runtime libraries, alter `CommandTarget` execution, or require Rust, Cargo, cgo, C/C++ compilers, or native tools by themselves.

`Make.CAbiHeader` describes one exported header file and the include root that should make that header visible to consumers:

- `Path`: header file path.
- `IncludeDir`: include root for compiler `-I`/equivalent flags.

`Make.CAbiLibrary` describes a produced C ABI library artifact:

- `Name`: logical artifact name.
- `Kind`: `Make.CAbiLibraryKind.Static` or `Make.CAbiLibraryKind.Shared`.
- `Headers`: exported `Make.CAbiHeader` values.
- `IncludeDirs`: include roots required by consumers.
- `LibraryPath`: main produced library path.
- `LinkName`: link name for `-l<name>`-style consumers.
- `LinkDirs`: library search directories.
- `RuntimeFiles`: files to copy or place near an executable for shared library runtime loading.
- `ImportLibraryPath`: Windows import library path for shared DLLs; empty for static libraries and Unix M0 artifacts.
- `CallingConvention`: ABI calling convention. `Make.CAbiCallingConvention.C` is the default/recommended M0 convention. `Make.CAbiCallingConvention.Stdcall` exists for future Windows-specific helpers.
- `Defines`: preprocessor defines required by consumers.

`Make.CAbiConsumer` describes generated or assembled consumption data for one or more C ABI libraries:

- `IncludeDirs`: include roots for compilation.
- `Defines`: preprocessor defines.
- `LinkDirs`: library search directories.
- `LinkNames`: `-l<name>`-style link names.
- `LibraryPaths`: direct library paths for consumers that link exact files.
- `LinkArgs`: additional linker arguments.
- `RuntimeFiles`: shared-library runtime files that must be staged near the executable or otherwise made discoverable.

Future helpers can translate `CAbiConsumer` into `CGO_CFLAGS`, `CGO_LDFLAGS`, Rust `cargo:rustc-link-*` build-script output, or C/C++ compile and link arguments. MAKE11 does not automatically consume these records.

Safety boundary: the C ABI is not a type-safety boundary. Strings, pointers, callbacks, heap ownership, structs passed by value, and panics/unwind crossing the ABI remain deferred and unsafe unless explicitly modeled by future helpers. M0 examples should use integer-only ABI surfaces.

These records are intended to appear later in plan snapshots, target traces, helper outputs, C ABI helper expansion, and `Make.octest` assertions. Trace integration and helper expansion are deferred unless they happen automatically through existing plan snapshot support.


## `Make.oct` and `Make.octest`

`Make.oct` is ordinary Oct source. It may contain build plan functions, configuration helper functions, target metadata helpers, and action helper functions. The current direct `oct make` path looks for a `Plan() -> Make.Plan` function, but the source file itself is still normal Oct code.

`Make.octest` is the natural xUnit-style companion test file for `Make.oct`. It has the same meaning as any other `.octest` file: a same-package test file loaded beside sibling `.oct` sources. Run it explicitly with `oct test Make.octest`; `oct make` does not automatically discover or run `Make.octest` before building. This keeps the build executor and the xUnit test lane separate.

Pure `Make.octest` tests should focus on plan/config data that can be checked without make authority:

- `Plan()` shape and selected default target;
- config helper functions such as debug/release profiles;
- target metadata;
- target inputs, outputs, and deps;
- `with`-based config/profile composition.

`[Fact]` and `[Theory]` are the recommended attributes for these pure assertions. `[Artifact]` may later be useful for pure plan snapshots or target-table evidence, but it must not become hidden build execution. `[Benchmark]` is not important for ordinary Make plan tests.

Side-effectful Make primitive tests are different. Tests that call `Make.Exec`, `Make.WriteText`, `Make.ReadText`, `Make.HashFile`, or other host primitives require explicit make authority and sidecar discovery. That coverage lives in `Libraries/MakeHostPrivileged/Make.Primitives.octest` and must be run explicitly with `OCT_MAKE_AUTHORITY=1` plus `OCT_WRAPPER_PATH`; ordinary `oct test Libraries/Make` and ordinary `oct test Make.octest` do not receive `OCT_MAKE_AUTHORITY=1`. This keeps pure Make package tests authority-free while preserving the Make host boundary.

A future pure helper such as `Make.ValidatePlan` or `Make.CheckPlan` may be useful, but it is deferred until it can reuse executor validation logic without creating a second source of truth.

## Test lanes

Pure Make package tests exercise ordinary package data and helper behavior without host authority. They include `Make.CAbi.octest` and any future pure `Make.octest` plan/config assertions in `Libraries/Make`. Run them from the repository root with either execution backend:

```sh
go run ./cmd/oct test Libraries/Make --execution interpreted
go run ./cmd/oct test Libraries/Make --execution compiled
```

Privileged Make host primitive tests are side-effectful: they create/remove files and invoke host programs through `octxiliary-makehost`. They are intentionally outside the broad `Libraries/Make` lane at `Libraries/MakeHostPrivileged/Make.Primitives.octest`, and they must be selected explicitly after building sidecars and granting make authority:

```sh
go run ./tools/build_sidecars --out dist/sidecars

OCT_MAKE_AUTHORITY=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
    go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution interpreted

OCT_MAKE_AUTHORITY=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
    go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution compiled
```

PowerShell equivalent:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_MAKE_AUTHORITY="1"
$env:OCT_WRAPPER_PATH="$PWD\dist\sidecars"
go run .\cmd\oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution interpreted
go run .\cmd\oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution compiled
```

The split is deliberate: pure Make tests do not require host authority, Make primitive tests remain explicit sidecar/authority tests, and ordinary `oct test` does not gain ambient Make host capabilities.


## `oct make` plan configuration

`Make.Plan` includes a typed `Make.Config` record. Build configuration belongs in Oct data rather than a growing set of CLI profile flags. The default values are:

```oct
Make.Config {
    Profile: "Default"
    StateDir: ".octmake"
    Trace: false
    Staleness: Make.Staleness.Timestamp
}
```

Profiles can be expressed by composing records with immutable `with` updates:

```oct
let Base = Make.Config {
    Profile: "Debug"
    StateDir: ".octmake"
    Trace: false
    Staleness: Make.Staleness.Timestamp
}

let Release = Base with {
    Profile: "Release"
    Trace: true
    StateDir: ".octmake/release"
}
```

`StateDir` controls where `oct make` writes `state.octagon` and `trace.octagon`; an empty value falls back to `.octmake`. `Trace: true` writes trace evidence without `--trace`, while `--trace` can still force trace writing for operational debugging. `Staleness.Timestamp` uses input/output modified times plus command identity hashing for command targets. `Staleness.Always` reruns selected command/function/flow targets. Ninja output and typed C/C++ or Go helper targets are intentionally deferred.

## Command identity hashing

`oct make` tracks a deterministic `CommandHash` for every `Make.CommandTarget`. The hash is SHA-256 over stable, length-prefixed target metadata: target kind (`command`), target name, `Program`, `Args` in order, explicit `Env` entries in order, `Cwd`, `Outputs` in order, `Inputs` in order, and `Deps` in order. It deliberately excludes volatile values such as timestamps, command output, durations, current time, temporary absolute state paths, tool versions, and ambient operating-system environment variables that are not explicitly listed in `CommandTarget.Env`.

After a successful command target run, `state.octagon` stores the current `CommandHash` on that target's state record. Existing state files that do not have `CommandHash` still parse; when outputs are otherwise present and up to date, a command target with older state but no hash is stale with reason `CommandHashMissing`. If the previous hash exists but differs from the current command metadata, the target is stale with reason `CommandChanged`. Missing outputs and missing inputs are still reported before command hash reasons; command hash reasons are checked before `InputNewerThanOutput`.

`trace.octagon` records both `CommandHash` and `PreviousCommandHash` for command decisions, including dry runs and failed command executions. `oct make --plan-out` includes the computed `CommandHash` in each command target snapshot so plan snapshots can be diffed for command identity changes. `oct make explain` computes and compares hashes without executing commands or mutating state.

Example:

```oct
Make.CommandTarget {
    Name: "Build"
    Program: "go"
    Args: ["build", "-o", "out/app", "./cmd/app"]
    Env: ["CGO_ENABLED=1"]
    Cwd: ""
    Inputs: []
    Outputs: ["out/app"]
    Deps: []
}
```

Changing `Args`, `Env`, `Program`, `Cwd`, or the declared inputs/outputs/dependencies makes `Build` stale even when `out/app` already exists and file timestamps are unchanged. Future work may add input content-hash staleness, tool path/version hashing, and hermetic environment/input/output tracking.

## Read-only reporting

`oct make --plan-out <file.octagon>` writes a valid Octagon snapshot of the full validated plan without adding execution decisions. The snapshot is intended for later comparison and includes the make file, default target, config, and all target metadata.

`oct make explain [target] [--file <path>]` reports the selected target closure and current staleness reasons without executing targets or mutating state. `oct make doctor [--file <path>]` reports make health: profile, state directory, backend, default target, target counts, validation/dependency status, state/trace existence, and referenced programs.

Example commands:

```sh
oct make --file Examples/ChimeraHello/Make.oct --plan-out .octmake/plan.octagon
oct make explain --file Examples/ChimeraHello/Make.oct TestChimera
oct make doctor --file internal/prometheus/Make.oct
```

Plan diffing, replay, failure artifact directories, input content-hash staleness, and richer tool reports remain future work.


## `FlowTarget`

`Make.FlowTarget` lets `oct make` run a named zero-argument Octomata flow as a direct-backend target action. A flow target participates in the same target graph, dependency validation, `--list`, `--dry-run`, timestamp staleness, state, and trace paths as command and function targets. It is intentionally direct-backend-only; Ninja lowering for flows is not implemented and should not be inferred.

MAKE4 uses the narrow `Int` result convention: a completed flow returning `0` succeeds, and a completed flow returning any non-zero integer fails the target. `MaxSteps` must be positive and bounds flow transitions so accidental non-terminating flows fail clearly. If a flow suspends before completion, the target fails with a diagnostic explaining that persistent make-flow resume is not supported in MAKE4. `trace.octagon` records flow name, max steps, executed steps, state history, result code when available, suspended status, and errors; `state.octagon` records the target as kind `flow` with status and path state.
