# PrometheusShadowAuthorityRakeLab M1

This is a rake lab experiment for Prometheus P15 shadow authority diagnostics.

- Uses Octomata flow/state/board to model shadow lifecycle calibration and authority gate decisions.
- Uses `when policy` (with hysteresis/min_commit) for gate arbitration.
- Simulation/design only: **not** native Prometheus implementation and does not change dispatch authority.

## Run

- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M1`
- `go run ./cmd/oct artifact Experiments/PrometheusShadowAuthorityRakeLab/M1`

## Json migration (2026-10-07)

`scenario_summary.json` is a `ScenarioSummary` record published with
`Artifact.WriteJson(path, value)`; it was JSON text built with
`String.Concat`. The values are the same. The keys are the record's field
names (`ScenarioCount`, was `scenarioCount`), in the layout `Json` writes. The
`[Artifact]` function loads the summary back with `Json.Load<ScenarioSummary>`
and checks its count; `oct artifact` of this directory failed on the old
read-back and passes now.
