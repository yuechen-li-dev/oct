# P2 Report — Prometheus SGEMM Vertical Slice

## Implemented vertical slice

P2 adds one explicit, correctness-gated SGEMM path with fixed scope:

- operation: `C = A × B`
- type: `float32`
- layout: row-major
- backends: explicit `cpu` or explicit `prometheus`
- starter corpus: `(1,1,1)`, `(4,4,4)`, `(3,5,7)`, `(64,64,64)`, `(256,64,1024)`

The end-to-end path now exists across:

1. native C ABI bridge (`internal/prometheus/native/bridge.h`, `bridge.cc`)
2. Go bridge/runtime + policy (`internal/prometheus/*.go`)
3. CLI harness surface (`oct prometheus-sgemm <cpu|prometheus> [--octagon-out file.octagon]`)
4. machine-readable `.octagon` report emission

## Backend modes in current environment

- `cpu` backend executes deterministic host SGEMM and passes correctness gate.
- `prometheus` backend is explicit and wired through native bridge.
  - if runtime is unavailable before dispatch, result is explicit `fallback(prometheus_unavailable)` and backend used is `cpu`.
  - if dispatch fails after execution starts, result is explicit `error(stage,code)` with no silent CPU reroute.

This slice is architecture/correctness scaffolding, not a performance claim milestone.

## Correctness policy

Each run compares backend output against deterministic CPU oracle and records:

- max absolute error
- max relative error
- failing element count
- first failing index/value pair
- NaN/Inf rejection behavior

Correctness failures hard-fail the run.

## Status/fallback/error reporting

Every run reports one visible status string:

- `ok`
- `fallback(<reason>)`
- `error(<stage>,<code>)`

Fallback and true Prometheus success remain distinct in output and `.octagon` reporting.

## Intentionally deferred

Still out of scope in P2:

- multiple kernels
- autotuning/tile selection
- mixed precision
- transpose flags
- batched GEMM
- streams/async
- persistent device residency
- generalized tensor runtime
- broad performance claims
