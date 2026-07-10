# SDSL-V M20 ExactTail SGEMM

The detailed M20 report lives in:

- `internal/prometheus/DevelopmentReport/SDSL_V_M20_EXACTTAIL_SGEMM.md`

M20 remains the explicit benchmark-only exact-tail reg2x2 baseline for later source-architecture experiments.

Follow-up:

- M24 keeps the same geometry, metadata, and exact/tail behavior, but rewrites the source around M21-M23 `board` / `flow` / `state`.
- See `internal/prometheus/DevelopmentReport/SDSL_V_M24_FLOW_BOARD_SGEMM.md`.
