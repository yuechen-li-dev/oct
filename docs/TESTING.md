# Testing policy

## Default unit workflow

Use the default unit-test command for local iteration and normal CI:

```bash
go test ./...
```

Default tests should remain focused on:
- fast unit tests,
- small compiled smoke regressions,
- core CLI/compiler behavior.

Default tests should **not** require:
- long science/benchmark sweeps,
- large artifact sweeps,
- real Prometheus sidecars/reactors,
- Octxiliary sidecars.

## Slow test workflow

Slow integration/science style tests are gated by:

```bash
OCT_RUN_SLOW_TESTS=1 go test ./...
```

Use this mode before releases or when specifically validating broader compiled/runtime paths.

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
