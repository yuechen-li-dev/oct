# Testing policy

## Default unit workflow

Use the default unit-test command for local iteration and normal CI:

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

General slow integration/science style tests are gated by:

```bash
OCT_RUN_SLOW_TESTS=1 go test ./...
```

The explicit Octxiliary wrapper lane is:

```bash
go run ./tools/build_sidecars --out dist/sidecars
OCT_SLOW_TESTS=1 OCT_WRAPPER_PATH="$PWD/dist/sidecars" go test -count=1 -parallel 8 ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

PowerShell equivalent:

```powershell
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_SLOW_TESTS = "1"
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test -count=1 -parallel 8 ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
```

Use slow modes before releases, when wrapper/octxiliary code changes, or when specifically validating broader compiled/runtime paths.

## Prometheus integration workflow

Real Prometheus reactor integration remains separately gated:

```bash
OCT_RUN_PROMETHEUS_INTEGRATION=1 OCT_PROMETHEUS_REACTOR=<path-to-reactor> go test ./...
```

This gate is independent from `OCT_RUN_SLOW_TESTS`; both may be used together when needed.

## Explicit compiled/runtime checks

Keep explicit command checks available for focused regression validation:

```bash
go run ./cmd/oct test examples/SmartGreenhouseController --execution compiled
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

## Semantic Contracts

These contracts live under `Language/` and are the canonical source for language behavior.

## Implementation and Integration Tests

Go-side tests should validate implementation and integration boundaries, including CLI orchestration, compile/run boundaries, and artifact wiring.

## Test Placement Rules

- Language semantics belong in `Language/*.octest` and `Language/*.octfail`.
- Go-side tests should validate implementation and integration boundaries.
- Keep reusable user code in `Packages/` and temporary fixtures in `testdata/`.

## Existing package directories

Use `oct new <kind> <Name>` when creating a new package directory from scratch; from a project root with `Experiments/` or `Libraries/`, new experiments default under `Experiments/<Name>` and new libraries/wrapper-libraries default under `Libraries/<Name>`. Use `oct init experiment`, `oct init library`, or `oct init wrapper-library` from inside an existing directory when Oct source/tests already exist but `manifest.oct` is missing. `oct init` derives the package name from the current directory basename and refuses to overwrite an existing manifest. Existing experiment folders should normally use `oct init experiment`.
