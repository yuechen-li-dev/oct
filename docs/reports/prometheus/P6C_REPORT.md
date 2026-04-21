# P6c Report — Windows-native Prometheus Characterization Baseline

## Scope

P6c measured the first trustworthy Windows-native Prometheus path without
changing the SGEMM kernel, memory model, or backend-selection semantics.

The goal was to establish an honest baseline for:

- CPU matrix multiply benchmarks
- Prometheus matrix multiply benchmarks
- real Windows-native Vulkan execution on RTX 3070

## Benchmark corpus used

The official experiment surface remained:

- `Experiments/PrometheusBenchmarkHarness/M0`

The corpus was expanded to the following paired CPU/Prometheus shapes:

- tiny: `1x1x1`, `2x2x2`, `4x4x4`, `8x8x8`
- awkward: `3x5x7`, `7x3x11`, `5x17x9`
- medium square: `16x16x16`, `32x32x32`, `64x64x64`
- medium rectangular: `64x16x64`, `16x64x64`, `64x64x16`
- larger sanity: `128x128x128`

Each shape is authored through the same `[Benchmark]` mechanism:

- `Main.MatrixMulCPUReference_M...`
- `Main.MatrixMulPrometheus_M...`

## Measurement protocol used

Characterization was run with:

- Windows-native `prometheus_reactor.dll`
- `CGO_ENABLED=1`
- MinGW `gcc/g++` configured for the benchmark child builds
- `OCT_PROMETHEUS_REACTOR` pointed at the repo-local reactor DLL
- `tools/prometheus/run_p6c_windows_native.ps1`

Protocol:

- warmup runs: `1`
- measured runs: `5`
- aggregation: median and average across measured runs

Per-run artifacts were written to:

- `out/prometheus/native/p6c/measured_01.octagon`
- `out/prometheus/native/p6c/measured_02.octagon`
- `out/prometheus/native/p6c/measured_03.octagon`
- `out/prometheus/native/p6c/measured_04.octagon`
- `out/prometheus/native/p6c/measured_05.octagon`

Aggregated summaries were written to:

- `out/prometheus/native/p6c/summary.json`
- `out/prometheus/native/p6c/summary.md`

## Artifact/output structure

`.octagon` output remains loadable and truthful. Per benchmark case it now
preserves:

- `Name`
- `DurationNs`
- `BackendRequested`
- `BackendUsed`
- `Status`
- `Environment`
- `ReportedWallNs`

Example from `out/prometheus/native/p6c/measured_01.octagon`:

```text
BenchmarkRun {
    Cases: [BenchmarkCaseResult {
        Name: "Main.MatrixMulCPUReference_M001_N001_K001"
        DurationNs: (472708400)
        BackendRequested: "cpu"
        BackendUsed: "cpu"
        Status: "ok"
        Environment: "not_applicable"
        ReportedWallNs: (0)
    }, BenchmarkCaseResult {
        Name: "Main.MatrixMulPrometheus_M001_N001_K001"
        DurationNs: (665744900)
        BackendRequested: "prometheus"
        BackendUsed: "prometheus"
        Status: "ok"
        Environment: "windows_native_vulkan"
        ReportedWallNs: (1884200)
    }]
}
```

Interpretation:

- `DurationNs` is the outer compiled benchmark process runtime
- `ReportedWallNs` is the inner Prometheus `wall=` timing emitted by the real
  runtime path

## Correctness / truth status

Across the warmup plus five measured runs:

- no crashes were observed
- CPU cases stayed `backend_used=cpu status=ok`
- Prometheus cases stayed `backend_used=prometheus status=ok`
- Prometheus environment stayed `windows_native_vulkan`
- benchmark stdout reported `correctness=true` for Prometheus runs
- no fallback data was included in the measured Prometheus baseline

## Summarized results

Median outer process timings:

| Shape | CPU median | Prometheus median | Prometheus inner wall | Prometheus / CPU |
| --- | --- | --- | --- | --- |
| `1x1x1` | `472.708 ms` | `665.745 ms` | `1.018 ms` | `1.41x` |
| `4x4x4` | `449.921 ms` | `664.440 ms` | `0.672 ms` | `1.48x` |
| `16x16x16` | `455.089 ms` | `667.914 ms` | `0.672 ms` | `1.47x` |
| `32x32x32` | `455.201 ms` | `668.109 ms` | `0.502 ms` | `1.47x` |
| `64x64x64` | `460.349 ms` | `686.095 ms` | `1.084 ms` | `1.49x` |
| `64x16x64` | `475.547 ms` | `674.115 ms` | `0.503 ms` | `1.42x` |
| `128x128x128` | `469.462 ms` | `685.244 ms` | `1.197 ms` | `1.46x` |

Important observed pattern:

- CPU outer timings stay almost flat around `0.45-0.49 s`
- Prometheus outer timings stay almost flat around `0.66-0.69 s`
- Prometheus inner wall stays around `0.50-1.36 ms`

So the measured shape growth is tiny compared with the fixed outer runtime.

## Observed shape-dependent behavior

### 1. For what shape ranges does CPU clearly win?

CPU clearly wins the entire measured corpus on end-to-end benchmark time.

There is no tested shape where Prometheus outer runtime beats CPU outer runtime.

### 2. For what shape ranges does Prometheus begin to look plausible?

Prometheus does not look plausible yet on end-to-end benchmark latency.

It only begins to look plausible in the narrower sense that the inner
Prometheus `wall` time is already in the sub-`1.5 ms` range even at
`128x128x128`.

That means Prometheus could become competitive only after the fixed setup cost
is substantially amortized or removed.

### 3. What bottleneck class dominates?

The dominant bottleneck is not visible as kernel-scaling cost.

The evidence points to fixed setup overhead:

- outer times are nearly shape-invariant for both CPU and Prometheus
- Prometheus adds an almost constant extra `~190-225 ms` over CPU
- the inner Prometheus wall is only `~0.5-1.3 ms`, far smaller than the outer
  process runtime

This strongly suggests current losses are dominated by:

- per-invocation runtime/dispatch/setup overhead
- likely runtime create/probe/teardown plus associated driver/buffer setup
- not the SGEMM arithmetic itself

Transfer cost may be part of that overhead, but the present baseline most
strongly isolates setup/dispatch amortization as the first-order issue.

### 4. Single recommended next optimization target

The single most justified next optimization target is:

> eliminate or amortize per-call Prometheus runtime setup by reusing a live
> Prometheus/Vulkan runtime across benchmark invocations before touching the
> SGEMM kernel.

Why this is the right next step:

- it is directly supported by the flat timing profile
- it attacks the `~200 ms` constant Prometheus penalty, which is far larger
  than the measured inner compute time
- kernel work is still too small a share of end-to-end time to justify SGEMM
  optimization first

## Acceptance check

P6c now has:

- a meaningful Windows-native benchmark corpus
- paired CPU and Prometheus benchmark cases in the official experiment harness
- usable `.octagon` artifacts
- repeated stable measured execution
- truthful backend/environment reporting
- correctness preserved
- an evidence-based next optimization target

No SGEMM kernel optimization, staging-buffer work, tiling, FFT, CUDA, or auto
mode scope was introduced.
