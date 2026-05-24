# Prometheus Shadow Authority Rake Lab M3

M3 is a threshold/chatter stability simulation pass for shadow authority policy.

- Scope: Oct-only diagnostic design lab.
- No native Prometheus dispatch/selector/lease/prestage authority changes.
- No native M8/M9/M10 constant changes.

## Variants

1. `M2BaselineEMA` (control)
2. `ReasonBindingEMA` (RecentFallback/RecentStale/HighArrivalError prevent HEALTHY)
3. `ReasonBindingEMA_StrongerCommit` (reason binding + stronger hysteresis/min_commit + lower alpha)

## Run

- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M3`
- `go run ./cmd/oct artifact Experiments/PrometheusShadowAuthorityRakeLab/M3`

## Artifacts

- `shadow_authority_m3_stability_report.octagon`
- `shadow_authority_m3_stability_report.md`
- `scenario_metrics.csv`
- `scenario_summary.json`
- `FINDINGS.md`
