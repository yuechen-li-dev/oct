# DVT-2 performance baseline

The timing-enabled fixed smoke produces per-evaluation and per-block host-visible evidence. It records reactor execution time, rebind time, payload-read time, and immutable bytes per closed block without changing arithmetic.

Read `internal/prometheus/DevelopmentReport/artifacts/Dvt2PreM0/dvt2_pipeline_timing.json` for run totals, `dvt2_native_stage_timing.json` for block timing, and `dvt2_bandwidth.json` for carefully bounded transfer evidence. GPU-copy timestamps, pinned-memory status, and whole-device allocation telemetry are not exposed by the current reactor ABI and are explicitly not inferred.

## Accepted performance history

| Milestone/profile | Scope | Canonical median | Candidate median | Improvement | Paired wins | Peak model-owned VRAM | Disposition |
|---|---|---:|---:|---:|---:|---:|---|
| DVT2-M6A / `FastMixedPrecision` | Real layer-0 W1/W3 boundary, RTX 3070 | 440.529 ms | 371.525 ms | 15.66% | 19/20 | 694,950,916 bytes | Accepted feasibility; preserved for future M6B, not production-promoted |

The candidate was finite and bitwise deterministic with Prefetch preserved.
The history is layer-0-only: it is neither a 30-layer nor final-image
performance or numerical claim. Canonical FP32 remains the default authority.
