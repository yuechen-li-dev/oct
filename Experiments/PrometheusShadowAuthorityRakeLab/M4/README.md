# PrometheusShadowAuthorityRakeLab M4

M4 is a **diagnostic-only Oct simulation lab** for healthy-boundary stability before native P15 M12 feature-flagged canary authority.

## Purpose

- Compare M3 recommended baseline (`ReasonBindingEMA_StrongerCommit`) against small local HEALTHY guards.
- Target remaining `BoundaryCanaryConfidence` over-promotion rake.
- Preserve recovery and stale/fallback safety behavior.

## Variants

1. `ReasonBindingEMA_StrongerCommit` (baseline from M3)
2. `HealthyMarginGate`
3. `HealthyStreakGate`
4. `HealthyMarginAndStreak` (small optional combination)

## No native changes

This lab does not change:
- native Prometheus code,
- P15 M8/M9/M10/M11 constants,
- dispatch/selector/lease/prestage authority.

## Run

- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M4`
- `go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M4 --suite Experiments.PrometheusShadowAuthorityRakeLab.M4.FlowSmoke`
- `go run ./cmd/oct artifact Experiments/PrometheusShadowAuthorityRakeLab/M4`

## Artifacts

- `shadow_authority_m4_boundary_report.octagon`
- `shadow_authority_m4_boundary_report.md`
- `scenario_metrics.csv`
- `scenario_summary.json`
- `FINDINGS.md`

## Interpreting findings

Use scenario-level metrics for evidence; use `FINDINGS.md` for interpretation, recommendation, and explicit M4 limits.
