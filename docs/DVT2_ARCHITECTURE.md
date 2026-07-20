# DVT-2 post-bootstrap architecture

The stable path is `intent -> replaceable conditioning producer -> Prometheus compiled model session -> replaceable projection/decoder -> artifact`. Prometheus is the compiled-model and lock authority; Python is accepted bootstrap infrastructure, not model authority.

The formal layer map, ownership, lifecycle, and M0 decision are the deterministic records in `internal/prometheus/DevelopmentReport/artifacts/Dvt2PreM0/`. `dvt2_architecture_layers.json` is the concise ownership map and `dvt2_session_lifecycle.json` is the actual current lifecycle.

The current fixed production-shaped smoke remains `tools/zimage_prometheus_smoke.py`: Qwen/embedding, Python scheduler, source final projection, VAE, and PNG stay outside the Prometheus core behind explicit seams.
