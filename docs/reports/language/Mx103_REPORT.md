# Mx103 Report — Oct-native Benchmark Profiling Surface

## 1) Prior MVP pprof implementation

Before Mx103, benchmark profiling was CPU-only and pprof-only:

- CLI accepted `oct bench <path> --profile cpu`.
- Runtime started `runtime/pprof.StartCPUProfile` before benchmark execution and stopped it at the end.
- Output was always raw `.pprof` (`bench.cpu.pprof`) with deterministic path rules.
- No Oct-native profile summary artifact was produced.

This implementation remains the capture foundation in Mx103.

## 2) New `.octagon` design

Mx103 introduces `BenchmarkProfileReport` as the Oct-native profile artifact written as `.octagon`.

The report includes:

- profile metadata (`RunID`, timestamp, mode, sample type/unit, duration, invocation, filter, environment)
- benchmark linkage (benchmarks included in the profiled run)
- top hot functions (`TopFunctions`) with flat/cumulative/percent and bounded callers/callees
- module aggregation (`TopModules`) sorted by hotness
- optional raw pprof linkage (`RawPprofPath`) when raw output is emitted in the same run

## 3) pprof → `.octagon` mapping

Mx103 still captures profiles via pprof, then parses the profile protobuf and projects it into Oct-native records:

- pprof samples drive function flat/cumulative accumulation
- adjacent stack frames provide bounded caller/callee relationships
- function names are grouped into module buckets for aggregate heat
- parsed duration/sample metadata and runtime context are attached as report metadata

No custom profiler was introduced; this is a thin summary layer over pprof.

## 4) CLI changes

`oct bench` now supports a native-first profile flow:

- `oct bench <path> --profile` → emits `.octagon` by default
- `oct bench <path> --profile --profile-format pprof` → emits raw `.pprof`
- `oct bench <path> --profile --profile-format both` → emits both

This keeps raw pprof explicit while making default profiling Oct-native.

## 5) Intentionally NOT implemented

Out of scope for Mx103 by design:

- full call graph reconstruction/visualization
- flamegraphs
- interactive UI
- parity with all pprof query/report features
- profiling expansion outside `oct bench`

## 6) Workflow improvement for Prometheus/benchmarking

Mx103 improves day-to-day benchmark workflow by making profile output immediately consumable as Oct artifacts:

- machine-readable `.octagon` suitable for artifact pipelines and automation
- human-readable profile summary printed in benchmark output
- explicit but preserved access to raw pprof for deep dives

This aligns profiling with the same Oct-native artifact workflow already used in benchmark and Prometheus reporting.
