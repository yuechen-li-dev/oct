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
- `go run ./cmd/oct bench <path> --profile cpu` to emit a Go `pprof` CPU profile for the benchmark run target

When the `oct` binary is available, `oct test <path>` is equivalent workflow intent.
When the `oct` binary is available, `oct bench <path> --profile cpu` is equivalent workflow intent.

## Benchmark Profiling (CPU, opt-in)

Benchmark profiling is a CLI/runtime execution option, not Oct source syntax.

- Enable with `--profile cpu` on `oct bench`.
- Output artifact path is deterministic:
  - directory target: `<target>/bench.cpu.pprof`
  - file target: `<target-file>.bench.cpu.pprof` in the same directory
- The artifact is pprof-compatible and intended to be consumed with existing Go profiling tools (for example, `go tool pprof <artifact>`).

Without `--profile cpu`, benchmark behavior is unchanged and no CPU profile artifact is emitted.

## Test Placement Rules

- language behavior contract → `Language/`
- reusable package behavior → `Libraries/` package tests/contracts
- fixture/transitional input data → `testdata/`

Do not treat `testdata/` as the canonical owner of language semantics.
