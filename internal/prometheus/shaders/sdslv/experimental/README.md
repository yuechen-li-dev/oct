# Experimental SDSL-V shader policy

This directory is intentionally empty of kernel sources today. Add a candidate
only under `experimental/<reactor-family>/`; it must not be referenced by the
Prometheus production registry, `native/shaders/manifest.json`, or native build
inputs. Benchmark and correctness evidence may use an experimental source, but
promotion requires a reviewed move into `production/<reactor-family>/` and the
normal production artifact and registry workflow.

The existing `tools/sdslv_benchmark_host/experiments/` material is a host-tool
experiment, not a Prometheus shader source authority.
