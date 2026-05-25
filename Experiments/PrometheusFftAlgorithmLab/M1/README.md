# Prometheus FFT Algorithm Lab M1

Oct-side correctness tapeout for radix-2 FFT using explicit Re/Im arrays.

## Package convention

The experiment-level manifest lives at `Experiments/PrometheusFftAlgorithmLab/manifest.oct`. Milestone folders do not need their own manifests. You can still target `M1` directly with `oct test`/`oct artifact`, and package context resolves from the experiment root.

## Run

- `go run ./cmd/oct test Experiments/PrometheusFftAlgorithmLab/M1`
- `go run ./cmd/oct artifact Experiments/PrometheusFftAlgorithmLab/M1`

## Artifacts

Artifacts are emitted deterministically under:

- `out/prometheus_fft_algorithm_lab/m1/m1_fft_cases.octagon`
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_results.octagon`
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_plan_traces.octagon`
- `out/prometheus_fft_algorithm_lab/m1/m1_fft_report.md`

Quick verification:

- `find out/prometheus_fft_algorithm_lab/m1 -maxdepth 1 -type f -print`
