# DVT-2 performance baseline

The timing-enabled fixed smoke produces per-evaluation and per-block host-visible evidence. It records reactor execution time, rebind time, payload-read time, and immutable bytes per closed block without changing arithmetic.

Read `internal/prometheus/DevelopmentReport/artifacts/Dvt2PreM0/dvt2_pipeline_timing.json` for run totals, `dvt2_native_stage_timing.json` for block timing, and `dvt2_bandwidth.json` for carefully bounded transfer evidence. GPU-copy timestamps, pinned-memory status, and whole-device allocation telemetry are not exposed by the current reactor ABI and are explicitly not inferred.
