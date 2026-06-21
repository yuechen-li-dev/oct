# Make

`Make` is a first-party Octxiliary wrapper package for future `oct make` host-capability primitives. It is intentionally side-effectful and is not part of Oct's ambient scientific runtime.

The `octxiliary-makehost` sidecar requires explicit make authority (`OCT_MAKE_AUTHORITY=1` in MAKE1 tests and future `oct make` plumbing). Ordinary `oct run`/test execution that reaches these functions without authority receives a clear capability error instead of process or file access.

Process execution uses `program` plus `args`; it does not invoke a shell string. Launch failures are fallible errors. Non-zero process exits return `ProcessResult` with the non-zero `ExitCode`, captured `Stdout`, and captured `Stderr`.

File primitives are intended for project build orchestration. MAKE1 does not yet enforce project-root path policy inside the sidecar; future `oct make` will provide authority and project-root scoping. Ninja backends, C/C++ helpers, Go helpers, target DAGs, and `Make.oct` plan execution are deferred.


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

Side-effectful Make primitive tests are different. Tests that call `Make.Exec`, `Make.WriteText`, `Make.ReadText`, `Make.HashFile`, or other host primitives require explicit make authority and sidecar discovery. Keep that coverage in `Libraries/Make`, explicit sidecar/integration lanes, or Go CLI tests for `oct make`; ordinary `oct test Make.octest` does not receive `OCT_MAKE_AUTHORITY=1`.

A future pure helper such as `Make.ValidatePlan` or `Make.CheckPlan` may be useful, but it is deferred until it can reuse executor validation logic without creating a second source of truth.


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

`StateDir` controls where `oct make` writes `state.octagon` and `trace.octagon`; an empty value falls back to `.octmake`. `Trace: true` writes trace evidence without `--trace`, while `--trace` can still force trace writing for operational debugging. `Staleness.Timestamp` uses input/output modified times. `Staleness.Always` reruns selected command/function/flow targets. Hash staleness, Ninja output, and typed C/C++ or Go helper targets are intentionally deferred.

## Read-only reporting

`oct make --plan-out <file.octagon>` writes a valid Octagon snapshot of the full validated plan without adding execution decisions. The snapshot is intended for later comparison and includes the make file, default target, config, and all target metadata.

`oct make explain [target] [--file <path>]` reports the selected target closure and current staleness reasons without executing targets or mutating state. `oct make doctor [--file <path>]` reports make health: profile, state directory, backend, default target, target counts, validation/dependency status, state/trace existence, and referenced programs.

Example commands:

```sh
oct make --file Examples/ChimeraHello/Make.oct --plan-out .octmake/plan.octagon
oct make explain --file Examples/ChimeraHello/Make.oct TestChimera
oct make doctor --file internal/prometheus/Make.oct
```

Plan diffing, replay, failure artifact directories, hash staleness, and richer tool reports remain future work.


## `FlowTarget`

`Make.FlowTarget` lets `oct make` run a named zero-argument Octomata flow as a direct-backend target action. A flow target participates in the same target graph, dependency validation, `--list`, `--dry-run`, timestamp staleness, state, and trace paths as command and function targets. It is intentionally direct-backend-only; Ninja lowering for flows is not implemented and should not be inferred.

MAKE4 uses the narrow `Int` result convention: a completed flow returning `0` succeeds, and a completed flow returning any non-zero integer fails the target. `MaxSteps` must be positive and bounds flow transitions so accidental non-terminating flows fail clearly. If a flow suspends before completion, the target fails with a diagnostic explaining that persistent make-flow resume is not supported in MAKE4. `trace.octagon` records flow name, max steps, executed steps, state history, result code when available, suspended status, and errors; `state.octagon` records the target as kind `flow` with status and path state.
