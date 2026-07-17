# Experiment report

M0 establishes a deterministic, non-authoritative simulation of the M49 numerical plant:

- explicit identification and held-out input families;
- depth-resolved recurrence with path/family-dependent injection;
- bounded source-side mitigation candidates with latency and memory costs;
- deterministic `when utility` selection;
- a shadow-only audit action whose decision cannot alter product execution;
- a native Octagon report plus reproducible CSV/JSON/Markdown and line-chart artifacts.

This lab is useful for testing the shape of the identification/controller contract. It does not establish real Vulkan accuracy, performance, determinism, or M48 EVT readiness.
