# Testing Guide

## Semantic Contracts

Language semantics are specified as:

- `.octest` for valid behavior
- `.octfail` for invalid/rejected behavior

These contracts live under `Language/` and are the canonical source for language behavior.

## Implementation and Integration Tests

Go-side tests should validate implementation and integration boundaries (CLI/runtime/backend mechanics), not duplicate language semantics already expressed in `Language/`.

## Practical Commands

In this environment, prefer:

- `go run ./cmd/oct test <path>` for contract execution
- `go run ./cmd/oct bench <path>` for benchmark execution
- `go run ./cmd/oct bench <path> --profile` to emit the default Oct-native benchmark profile artifact
- `go run ./cmd/oct bench <path> --profile --profile-format pprof` to emit raw Go `pprof` CPU profile data

When the `oct` binary is available, `oct test <path>` is equivalent workflow intent.
When the `oct` binary is available, `oct bench <path> --profile` is equivalent workflow intent.

## Benchmark Profiling (CPU, opt-in)

Benchmark profiling is a CLI/runtime execution option, not Oct source syntax.

- Enable with `--profile` on `oct bench`.
- Default output artifact path is deterministic:
  - directory target: `<target>/bench.cpu.octagon`
  - file target: `<target-file>.bench.cpu.octagon` in the same directory
- Raw pprof output remains available with `--profile-format pprof` (or `both`):
  - directory target: `<target>/bench.cpu.pprof`
  - file target: `<target-file>.bench.cpu.pprof` in the same directory
- Oct captures CPU profile data through pprof internally, then exposes an Oct-native summary artifact by default.

Without `--profile`, benchmark behavior is unchanged and no CPU profile artifact is emitted.

## Test Placement Rules

- language behavior contract → `Language/`
- reusable package behavior → `Libraries/` package tests/contracts
- fixture/transitional input data → `testdata/`

Do not treat `testdata/` as the canonical owner of language semantics.
