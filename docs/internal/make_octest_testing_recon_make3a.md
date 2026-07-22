# MAKE3a-DESIGN-RECON — `Make.octest` testing model audit

Status: design reconnaissance only. This report changes no `oct test`, `.octest`, xUnit attribute, `oct make`, Octomata, package-manager, manifest, Ninja, or Make host-capability semantics.

## Executive recommendation

`Make.octest` should mean exactly what any other `.octest` file means today: normal xUnit-style Oct tests loaded beside ordinary `.oct` files in the same package. It should not be a magic build-script test lane, should not be auto-run by `oct make`, and should not receive Make host authority unless the caller deliberately runs an explicit make-authorized sidecar/integration lane.

For MAKE3, the useful role is narrow and high-value:

1. Use `Make.octest` for pure plan/config tests that call `Plan()`, `DebugConfig()`, `ReleaseConfig()`, and target metadata helpers.
2. Keep side-effectful `Make.*` primitive tests in `Libraries/Make` or explicit integration fixtures run with `OCT_WRAPPER_PATH` plus `OCT_MAKE_AUTHORITY=1` when intentionally testing the sidecar.
3. Keep `oct make` executor behavior in Go CLI/integration tests that create temporary projects and run the actual command.
4. Defer `Make.ValidatePlan` / `Make.CheckPlan` until the executor validation rules are stable enough to expose through the Make package without creating a second source of truth.

## Current `.octest` behavior relevant to `Make.octest`

### Discovery and loading

`oct test <path>` calls `project.LoadForTest(path)`, which loads ordinary source plus tests. Directories include `.oct` files and, when test loading is enabled, `.octest` files. File selection is special: if the input path is a single `.octest` or `.oct` file, the loader records selected files for that entry package. During package file collection, all `.oct` files are retained, but `.octest` files are filtered down to the explicitly selected test file. This means `oct test Make.octest` loads sibling `.oct` implementation files in the same package while executing only tests from `Make.octest`.

The loader also skips generated compiled-test runner files named `zz_oct_test_runner_*.octest` unless they are explicitly selected. Directory loading sorts files deterministically before parsing.

Implication for `Make.octest`: a project package containing `Make.oct` and `Make.octest` can use the test file as a normal same-package test companion. The test file can call same-package functions from `Make.oct` such as `Plan()` without import ceremony, assuming the package names match and the functions typecheck.

### Relationship to sibling `.oct` files

`.octest` is not isolated from sibling source. It is parsed into the same package model as `.oct` files. Imported packages are loaded normally through the existing project/package resolver; by default their tests are not executed unless `--all-packages` is used.

`docs/TESTING.md` documents the command behavior directly: `oct test <path>` executes tests only from the selected entry package/root by default; transitive imports are still loaded for typechecking and symbol resolution, and imported-package tests require `--all-packages` to run.

Implication for `Make.octest`: tests can import packages normally and can call ordinary functions from `Make.oct`. If `Make.oct` imports `Make`, the test package can inspect returned `Make.Plan` records and `Make.Config` helpers just like any other Oct record values.

### `[Fact]`

The parser marks a function as a fact when `[Fact]` precedes it. `[Fact]` cannot be combined with `[Theory]`, `[Artifact]`, or `[Benchmark]`, cannot have `[InlineData]`, and cannot have `[CycleTime]`. The tester discovers `fn.IsFact` functions and executes one test case per function.

Interpreted facts execute through `interpret.ExecuteFunctionWithArgsAndOptions`. The test runtime records assertions. A completed fact with zero assertions is a failure. Fallible facts are supported: the tester records whether the function is fallible and the interpreter helper handles the call accordingly.

In automatic mode, the tester first attempts a compiled run. If compiled execution is unsupported, it falls back to interpreted mode and prints an informational fallback line. In `--execution compiled`, compiled failure is a test failure rather than a fallback.

Recommended `Make.octest` use: `[Fact]` is the default for pure plan/config assertions such as default target, profile name, state directory, target counts, and target inputs/outputs/deps.

### `[Theory]`

A theory must have at least one `[InlineData(...)]` row. Inline data currently supports scalar literals and enum values. The tester expands each row into a separate test case with display names like `FunctionName[0]`, converts inline AST values to runtime values, and invokes the theory with those arguments. `[CycleTime(<Float<s>>)]` is valid only on theories and controls the per-case timeout; otherwise the default timeout is used.

Recommended `Make.octest` use: `[Theory]` is useful for target/profile matrix checks, for example validating that `Debug`, `Release`, or platform/profile helper functions produce expected target names and state directories. It should remain pure unless intentionally run in a privileged lane.

### `[Artifact]`

`oct artifact <path>` is a separate compiler/build phase. It loads tests,
typechecks, discovers functions marked `IsArtifact`, sorts them deterministically,
and runs them through the shared typed interpreter with a confined staged-output
capability. The retained `--execution compiled` spelling delegates to this same
evaluator; it no longer generates a runner or invokes selected-file compilation.

Artifact execution supports progress/checkpoint recorder output. It is not part of ordinary `oct test` fact/theory execution or application startup.

Recommended `Make.octest` use: `[Artifact]` can be considered later for pure plan snapshots, build-plan examples, or trace-like documentation outputs. It must not become a hidden host-build execution lane. If an artifact calls side-effectful Make APIs, the caller must explicitly provide sidecar discovery and make authority.

### `[Benchmark]`

`oct bench <path>` is also separate. It loads tests, discovers functions marked `IsBenchmark`, optionally filters by qualified name substring, and runs benchmarks as compiled binaries through generated runners. It can write benchmark/profiling metadata to Octagon outputs.

Recommended `Make.octest` use: `[Benchmark]` is not important for MAKE3. It may later measure pure planner/helper performance or dry-run validation overhead, but it should not mutate files by default and should not be used as a casual build executor.

### Selected-file compiled tests

Fact/theory compiled execution now writes `runner.octest` and its compiled binary inside an owned `octest-run-*` temporary scope, then calls `build.CompileForTestWithSelectedFilesInPackage` so package resolution still uses the source package directory without writing generated files there. The project loader keeps all `.oct` source files while filtering `.octest` files to the selected external runner and selected test file. Benchmark compiled paths use the same lifecycle-scoped selected-file model. Artifact generation no longer uses this runner lifecycle; it is the build-time interpreter phase described above. `OCT_KEEP_TEST_ARTIFACTS=1` is the explicit debug-retention escape hatch; ordinary `oct build` artifacts remain persistent.

Implication for `Make.octest`: selected-file compiled execution should see sibling `Make.oct` code and the selected `Make.octest`, but it should not compile unrelated `.octest` files in the directory. This is exactly the desired behavior for focused plan/config tests.

### Can tests call ordinary functions, import packages, and call `Plan()`?

Yes, with normal Oct rules:

- A same-package `Make.octest` can call ordinary functions from sibling `Make.oct` such as `Plan()`.
- It can import packages normally.
- It can call `Plan()` today if the package names line up and `Plan()` is ordinary Oct code.
- A pure `Plan()` returning a `Make.Plan` record can be tested without Make host authority.
- A `Plan()` that calls side-effectful Make host primitives during planning would require sidecar discovery and authority, and should be treated as privileged integration behavior, not normal unit-test behavior.

## Current make authority under tests

MAKE1 introduced the `Libraries/Make` wrapper package backed by `cmd/octxiliary-makehost`. The sidecar rejects all Make family requests unless `OCT_MAKE_AUTHORITY=1`, with the clear error message `Make host capabilities are only available under oct make`.

MAKE2's direct executor sets `OCT_MAKE_AUTHORITY=1` around `Plan()` evaluation and function-target execution using internal `withMakeAuthorityValue`. It preserves the normal wrapper discovery model: sidecars are found beside the executable or through `OCT_WRAPPER_PATH`.

Ordinary `oct test Make.octest` does not set `OCT_MAKE_AUTHORITY=1`. It should not. Ordinary tests are the safe, ambient lane for pure language/package behavior. Granting Make authority to every test that happens to import `Make` would turn test execution into a file/process mutation surface and would blur the boundary that MAKE1 deliberately created.

### What ordinary tests can and cannot cover

Ordinary `oct test Make.octest` can cover:

- `Plan()` shape when `Plan()` is pure.
- config helper functions such as `DebugConfig()` and `ReleaseConfig()`.
- target names, default target, target inputs, outputs, deps, command metadata, and function metadata.
- pure structural checks and invariants that only inspect records and arrays.

Ordinary `oct test Make.octest` should not cover:

- `Make.Exec`, `Make.WriteText`, `Make.ReadText`, `Make.HashFile`, `Make.MkdirAll`, or other host primitives unless the command is explicitly run with authority and sidecar discovery.
- end-to-end executor staleness/state/trace behavior; those belong in `oct make` CLI/integration tests.

### Existing side-effectful/privileged lane pattern

The repository already has an explicit slow/sidecar convention. `docs/TESTING.md` says sidecar-heavy compiled/auto wrapper tests are outside the default fast lane and are gated by `OCT_SLOW_TESTS=1` or legacy `OCT_RUN_SLOW_TESTS=1`, with `OCT_WRAPPER_PATH` pointing at built sidecars. Focused wrapper tests build sidecars into temp dirs and set `OCT_WRAPPER_PATH` in Go tests.

`Libraries/MakeHostPrivileged/Make.Primitives.octest` contains side-effectful Make primitive facts. Those tests create/remove files, invoke `go version`, and check process results. They are appropriate as make-host primitive coverage only when the lane explicitly supplies `OCT_WRAPPER_PATH` and `OCT_MAKE_AUTHORITY=1`. They are not a model for ordinary project `Make.octest` files.

Recommendation: make-authorized tests should remain separate from ordinary tests. Use explicit environment-gated sidecar lanes for primitive coverage and Go CLI tests for `oct make` executor behavior.

## Recommended `Make.octest` layers

### Layer A — pure plan/config tests

This is the recommended normal project `Make.octest` model.

Properties:

- no side effects;
- no `Make.Exec`;
- no file mutation;
- tests `Plan()`, config helper functions, target metadata, target names, default target, target inputs/outputs/deps;
- runs under normal `oct test`;
- works in interpreted, compiled, or auto execution subject to current compiler support for the records/features used.

Example shape using current `Assert.*` style:

```oct
package Main

[Fact]
fn PlanDefaultIsBuild() -> Void {
    let plan = Plan()
    Assert.Equal("Build", plan.Default, "default target should be Build")
}

[Fact]
fn ReleaseConfigUsesReleaseStateDir() -> Void {
    let cfg = ReleaseConfig()
    Assert.Equal("Release", cfg.Profile, "release profile should be named Release")
    Assert.Equal(".octmake/release", cfg.StateDir, "release state should be isolated")
    Assert.True(cfg.Trace, "release config should enable trace")
}
```

### Layer B — Make helper/primitive tests

This layer is side-effectful and privileged.

Properties:

- uses `Make.WriteText`, `Make.ReadText`, `Make.HashFile`, `Make.Exec`, or similar primitives;
- requires make authority and sidecar discovery;
- should live in explicit integration fixtures such as `Libraries/MakeHostPrivileged`;
- should be run by an explicit command such as:

```bash
OCT_WRAPPER_PATH="$PWD/dist/sidecars" OCT_MAKE_AUTHORITY=1 go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution interpreted
```

Use this lane to validate the primitive capability surface, not ordinary project build plans.

### Layer C — `oct make` integration tests

This layer belongs in Go CLI/integration tests.

Properties:

- creates temporary projects;
- writes `manifest.oct`, `Make.oct`, and any source inputs needed;
- runs `oct make` or calls the CLI entrypoint;
- asserts outputs, state files, trace files, diagnostics, target selection, staleness decisions, and authority behavior.

This is the right place for executor behavior because the motivating behavior is the external command and host authority boundary, not pure Oct semantics.

### Layer D — artifact/benchmark build evidence

This layer is future/design-only for MAKE3.

Potential future `[Artifact]` uses:

- write a pure plan snapshot;
- write a documented target table;
- emit example dry-run trace data generated without host process/file mutation.

Potential future `[Benchmark]` uses:

- measure pure plan construction;
- measure dry-run planner/checker performance;
- compare config helper strategies.

Do not use `[Artifact]` or `[Benchmark]` as hidden build execution lanes. Host builds remain explicit and authorized.

## Should `oct make` run tests?

No, not in M0/MAKE3.

Answers:

- `oct make` should not automatically run `Make.octest` before building.
- `Make.octest` should be run manually by `oct test Make.octest`.
- A future `oct make --check-plan` or `oct make --test-plan` may be useful, but it should be a deliberate mode, not an implicit pre-build step.
- A built-in `TestPlan` target convention should not be added in MAKE3. Users can define ordinary phony/function targets if they want a project-specific check workflow, but the command should not create special semantics around that name yet.

Rationale: `oct make` is an executor. `oct test` is the xUnit test lane. Automatically mixing them would add surprising latency, duplicate the test runner's job, create authority ambiguity, and make failure modes less legible. The safer M0 boundary is explicit commands: `oct test Make.octest` for pure tests; `oct make` for builds.

## `ValidatePlan` / `CheckPlan` design audit

A pure validation helper would be valuable eventually:

```oct
Make.ValidatePlan(plan) ! Error -> Int
```

or:

```oct
Make.CheckPlan(plan) ! Error -> Int
```

However, MAKE3 should probably defer it.

Findings:

1. Plan validation can be exposed to Oct tests only if it is the same validation rule set used by the Go executor, or if the relationship is explicitly documented as a weaker structural precheck. Duplicating validation in Oct would violate the Go-implements-language / Oct-expresses-contracts separation and would risk drift.
2. `Make.octest` authors should eventually be able to call a pure checker. That would let projects assert that `Plan()` is structurally valid without running actions.
3. Before MAKE3, the executor validation rules are still moving: `Make.Config`, `.octmake/state.octagon`, richer traces, staleness policy, and record `with` config style all affect what should be checked.
4. MAKE3 should not add `ValidatePlan` unless the implementation can expose the executor's validation logic directly through a host-provided pure builtin/package function. A docs-only note saying this is a future helper is safer.

Recommended name: prefer `Make.ValidatePlan(plan)` if the helper returns rich fallible validation errors and enforces executor-grade rules. Use `Make.CheckPlan(plan)` only if it is intentionally lightweight and diagnostic-oriented. The former is clearer for tests.

## Attribute usage guidance for Make tests

- `[Fact]`: best default for pure plan/config assertions.
- `[Theory]`: useful for target-name, profile, platform, and config matrix cases via `[InlineData]`.
- `[Artifact]`: possible future plan snapshot or build-trace example lane. It must not run host builds casually and must not imply make authority.
- `[Benchmark]`: likely unimportant for MAKE3. If used later, keep it pure or dry-run-only by default.

Side-effect classification rule: attributes do not grant authority. A `[Fact]`, `[Theory]`, `[Artifact]`, or `[Benchmark]` that calls Make host primitives is privileged only because the surrounding process was explicitly launched with `OCT_MAKE_AUTHORITY=1` and sidecar discovery, not because of the attribute.

## Proposed fixture/doc examples

A future pure fixture should use the current `Make.Config`/`Make.Plan` records and record `with` style once MAKE3 finalizes it. Suggested `Make.oct`:

```oct
package Main

import Make

fn DebugConfig() -> Make.Config {
    return Make.Config {
        Profile: "Debug"
        StateDir: ".octmake"
        Trace: false
        Staleness: Make.Staleness.Timestamp
    }
}

fn ReleaseConfig() -> Make.Config {
    return DebugConfig() with {
        Profile: "Release"
        StateDir: ".octmake/release"
        Trace: true
    }
}

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: DebugConfig()
        CommandTargets: []
        FunctionTargets: [
            Make.FunctionTarget {
                Name: "Build"
                Inputs: ["src/main.oct"]
                Outputs: ["dist/app"]
                Deps: []
                Function: "Build"
            }
        ]
        PhonyTargets: []
    }
}
```

Suggested `Make.octest`:

```oct
package Main

[Fact]
fn PlanDefaultIsBuild() -> Void {
    let plan = Plan()
    Assert.Equal("Build", plan.Default, "default target should be Build")
}

[Fact]
fn ReleaseConfigUsesReleaseStateDir() -> Void {
    let cfg = ReleaseConfig()
    Assert.Equal("Release", cfg.Profile, "release profile should be named Release")
    Assert.Equal(".octmake/release", cfg.StateDir, "release state should be isolated")
    Assert.True(cfg.Trace, "release config should enable trace")
}
```

Do not add this fixture until MAKE3's exact `Make.Config` and record `with` expectations are settled enough to avoid churn. If added during MAKE3, run only the focused command for that fixture.

## MAKE3 interaction recommendations

MAKE3 plans `Make.Config`, `.octmake/state.octagon`, richer traces, and config records with `with`. It should account for `Make.octest` as follows:

1. Add docs saying `Make.octest` is a normal `.octest` companion intended primarily for pure plan/config tests.
2. Add, if small and stable, one fixture proving `Make.octest` can test `Plan()` and config helpers without authority.
3. Do not require or teach `oct make` to run tests.
4. Do not make Make host capabilities ambient in ordinary `oct test`.
5. Keep side-effectful Make primitive tests in `Libraries/Make` or explicit sidecar lanes.
6. Keep `oct make` executor coverage in Go CLI tests.
7. Defer `Make.ValidatePlan` unless MAKE3 can route it to the executor validation code without duplicating rules.

## Exact next prompt additions/edits for MAKE3

Add these requirements to the MAKE3 prompt:

```text
Make.octest policy:
- Document that Make.octest is an ordinary .octest companion for Make.oct.
- Do not make oct make auto-run Make.octest.
- Do not add make-specific test attributes or change [Fact]/[Theory]/[Artifact]/[Benchmark].
- Do not grant Make host authority to ordinary oct test.
- Keep normal project Make.octest examples pure: Plan(), config helpers, target metadata, and assertions only.
- If adding a fixture, prove oct test Make.octest can call sibling Make.oct Plan()/config helpers without OCT_MAKE_AUTHORITY.
- Keep side-effectful Make primitive tests in explicit integration lanes that set OCT_WRAPPER_PATH and OCT_MAKE_AUTHORITY=1.
- Keep oct make executor/state/trace/staleness tests as Go CLI integration tests.
- Mention Make.ValidatePlan as a future pure helper; do not implement it unless it reuses executor validation directly and remains side-effect free.
```

Suggested fixture acceptance command if a pure fixture is added:

```bash
go run ./cmd/oct test <fixture-path>/Make.octest --execution interpreted
```

Use compiled or auto only if the fixture features are already known to compile reliably in the selected-file compiled path.

## Final position

`Make.octest` should be boring by design. It is the normal xUnit companion to `Make.oct`, best used for pure assertions over plan/config data. Authority remains outside ordinary tests. Builds remain in `oct make`. Side-effectful Make primitives remain explicit sidecar/integration coverage. This lets MAKE3 improve configuration, state, and trace behavior without creating a second build/test semantics surface.
