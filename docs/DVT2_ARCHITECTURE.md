# DVT-2 post-bootstrap architecture

## Current status

DVT2-M6A is closed as an accepted successful feasibility milestone. Canonical
FP32 remains the default authoritative production compute profile. The
cooperative F16×F16→F32 route is preserved separately as
`FastMixedPrecision`, is not production-promoted, and cannot be selected by the
current Auto/default policy. It is eligible for a future M6B only after a new
milestone is explicitly started; no M6B or complete-image cooperative work is
part of the M6A closeout.

DVT-2 is ready for the Build Week packet checkpoint with M6A closed and M6B
not started.

The canonical `5e-5` numerical threshold is unchanged. `FastMixedPrecision`
requires its own whole-transformer and final-image authority before any future
production-eligibility decision.

The stable path is `intent -> replaceable conditioning producer -> Prometheus compiled model session -> replaceable projection/decoder -> artifact`. Prometheus is the compiled-model and lock authority; Python is accepted bootstrap infrastructure, not model authority.

The formal layer map, ownership, lifecycle, and M0 decision are the deterministic records in `internal/prometheus/DevelopmentReport/artifacts/Dvt2PreM0/`. `dvt2_architecture_layers.json` is the concise ownership map and `dvt2_session_lifecycle.json` is the actual current lifecycle.

The current fixed production-shaped smoke remains `tools/zimage_prometheus_smoke.py`: Qwen/embedding, Python scheduler, source final projection, VAE, and PNG stay outside the Prometheus core behind explicit seams.
