# PrometheusShadowAuthorityRakeLab M5

## Purpose
M5 models the feedforward actuator dispatch contract after native P15 M13 failed to converge end-to-end.

## Relation to native M13 failure
This lab encodes deterministic validate-before-consume behavior so native retry can own full dispatch semantics instead of only a reservation seam.

## Contract modeled
- Validate reservation and gate conditions before consume.
- Use feedforward only when all checks pass.
- Fall back to judgment for disabled, unhealthy, reason-binding, margin, fallback-required, no-matured, stale/cancelled/consumed, shape mismatch, variant mismatch, capability mismatch.
- Consume once when feedforward is actually used.
- Keep prestage eligibility diagnostic-only.

## No native changes
M5 modifies only `Experiments/PrometheusShadowAuthorityRakeLab/M5` Oct lab files and artifacts.

## Run
- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M5`
- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M5 --suite Experiments.PrometheusShadowAuthorityRakeLab.M5`
- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M5 --suite Experiments.PrometheusShadowAuthorityRakeLab.M5.FlowSmoke`
- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M5 --suite Experiments.PrometheusShadowAuthorityRakeLab.M5.FlowSmoke --execution compiled`
- `go run ./cmd/oct artifact Experiments/PrometheusShadowAuthorityRakeLab/M5`

## Artifact outputs
- `shadow_authority_m5_feedforward_contract_report.octagon`
- `shadow_authority_m5_feedforward_contract_report.md`
- `scenario_metrics.csv`
- `scenario_summary.json`
- `FINDINGS.md`
