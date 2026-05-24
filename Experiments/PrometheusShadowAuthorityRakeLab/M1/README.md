# PrometheusShadowAuthorityRakeLab M1

This is a rake lab experiment for Prometheus P15 shadow authority diagnostics.

- Uses Octomata flow/state/board to model shadow lifecycle calibration and authority gate decisions.
- Uses `when policy` (with hysteresis/min_commit) for gate arbitration.
- Simulation/design only: **not** native Prometheus implementation and does not change dispatch authority.

## Run

- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M1`
- `go run ./cmd/oct artifact Experiments/PrometheusShadowAuthorityRakeLab/M1`
