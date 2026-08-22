# Testing policy

## Default unit workflow

Use the default unit-test command for local iteration and normal CI. See
`docs/GO_TEST_LANES.md` for the complete lane matrix:

```bash
go test -count=1 -parallel 8 ./...
```

PowerShell equivalent:

```powershell
go test -count=1 -parallel 8 ./...
```

Default tests should remain focused on:
- fast unit tests,
- small compiled smoke regressions,
- core CLI/compiler behavior.

Default tests should **not** require:
- long science/benchmark sweeps,
- large artifact sweeps,
- real Prometheus sidecars/reactors.

Sidecar-heavy compiled/auto wrapper tests are not part of the default fast lane. They are gated by `OCT_SLOW_TESTS=1` (or legacy `OCT_RUN_SLOW_TESTS=1`) and should still keep no-fallback and missing-sidecar assertions honest.

## Slow test workflow

Compiler/corpus integration tests use the `integration` build tag:

```bash
go test -tags=integration ./...
```

The explicit Octxiliary wrapper lane is:

```bash
go run ./tools/build_sidecars --out dist/sidecars
OCT_SLOW_TESTS=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" go test -count=1 -parallel 8 -tags=toolchain ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

PowerShell equivalent:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_SLOW_TESTS = "1"
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test -count=1 -parallel 8 -tags=toolchain ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

Use slow modes before releases, when wrapper/octxiliary code changes, or when specifically validating broader compiled/runtime paths.

## Make test lanes

The Make package has two intentionally separate test lanes. Pure Make library tests exercise ordinary package data, including the C ABI artifact records in `Libraries/Make/Make.CAbi.octest`, and do not require Make host authority:

```bash
go run ./cmd/oct test Libraries/Make --execution interpreted
go run ./cmd/oct test Libraries/Make --execution compiled
```

Privileged Make host primitive tests live at `Libraries/MakeHostPrivileged/Make.Primitives.octest`. They are side-effectful, call the `octxiliary-makehost` sidecar, and must be run explicitly with sidecar discovery plus make authority:

```bash
go run ./tools/build_sidecars --out dist/sidecars

OCT_MAKE_ENV_TEST_VALUE=hello OCT_MAKE_EMPTY_ENV_TEST_VALUE= \
OCT_MAKE_AUTHORITY=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
    go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution interpreted

OCT_MAKE_ENV_TEST_VALUE=hello OCT_MAKE_EMPTY_ENV_TEST_VALUE= \
OCT_MAKE_AUTHORITY=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
    go run ./cmd/oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution compiled
```

PowerShell equivalent:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_MAKE_AUTHORITY="1"
$env:OCT_WRAPPER_PATH="$PWD\dist\sidecars"
$env:OCT_MAKE_ENV_TEST_VALUE="hello"
$env:OCT_MAKE_EMPTY_ENV_TEST_VALUE=""
go run .\cmd\oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution interpreted
go run .\cmd\oct test Libraries/MakeHostPrivileged/Make.Primitives.octest --execution compiled
```

This separation preserves the authority boundary: ordinary `oct test Libraries/Make` does not set `OCT_MAKE_AUTHORITY=1`, while primitive host coverage remains available in an explicit privileged lane. The privileged lane also sets `OCT_MAKE_ENV_TEST_VALUE` and `OCT_MAKE_EMPTY_ENV_TEST_VALUE` so `Make.Env` coverage does not depend on user-machine ambient environment.

## Prometheus integration workflow

Real Prometheus reactor integration remains separately gated:

```bash
OCT_RUN_PROMETHEUS_INTEGRATION=1 OCT_PROMETHEUS_REACTOR=<path-to-reactor> go test ./...
```

This gate is independent from `OCT_RUN_SLOW_TESTS`; both may be used together when needed.

## Explicit compiled/runtime checks

Keep explicit command checks available for focused regression validation:

```bash
go run ./cmd/oct test Examples/SmartGreenhouseController --execution compiled
go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M5 --suite Experiments.FmBrownNoiseKalman.M5.FlowSmoke --execution compiled
go run ./cmd/oct test Experiments/FmBrownNoiseKalman/M4 --suite Experiments.FmBrownNoiseKalman.M4.FlowSmoke --execution compiled
go build -o .tmp/octxiliary-io ./cmd/octxiliary-io
OCT_WRAPPER_PATH=$(pwd)/.tmp/octxiliary-io go run ./cmd/oct test Language/Testing/CompiledOctxiliary/valid --execution compiled
go run ./cmd/oct test Libraries/String --execution compiled
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution interpreted
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution auto
go run ./cmd/oct test Language/Testing --all-packages
```

By default, `oct test <path>` executes tests only from the selected entry package/root. Transitive imports are still loaded for typechecking and symbol resolution, but imported-package tests are not executed unless `--all-packages` is passed.

Compiled Octest batching has two explicit boundaries:

```oct
[Suite("Numerics")]
[Fact]
fn Adds() -> Void {
    Assert.Equal(4, 2 + 2, "addition")
}
```

All selected cases with the same package-qualified Suite name share one native
harness, including cases in multiple `.octest` files. A file whose cases have
no `[Suite]` gets one harness for that file. Theory rows are data/case entries
inside the same harness and do not trigger additional compilation. Cases retain
stable IDs and are replayed through `--case <stable-id>` so assertion failures,
fatal exits, and per-case timeouts keep exact attribution.

## Semantic Contracts

These contracts live under `Language/` and are the canonical source for language behavior.

## Implementation and Integration Tests

Go-side tests should validate implementation and integration boundaries, including CLI orchestration, compile/run boundaries, and artifact wiring.

## Test Placement Rules

- Language semantics belong in `Language/*.octest` and `Language/*.octfail`.
- Go-side tests should validate implementation and integration boundaries.
- Keep reusable user code in `Packages/` and temporary fixtures in `testdata/`.

## Existing package directories

Use `oct new <kind> <Name>` when creating a new package directory from scratch; from a project root with `Experiments/` or `Libraries/`, new experiments default under `Experiments/<Name>` and new libraries/wrapper-libraries default under `Libraries/<Name>`. Use `oct init experiment`, `oct init library`, `oct init application`, or `oct init wrapper-library` from inside an existing directory when Oct source/tests already exist but `manifest.oct` is missing. `oct init` derives the package name from the current directory basename and refuses to overwrite an existing manifest. Existing experiment folders should normally use `oct init experiment`.
