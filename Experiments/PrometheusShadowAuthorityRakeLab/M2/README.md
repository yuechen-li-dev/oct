# PrometheusShadowAuthorityRakeLab M2

This rake lab compares two **diagnostic-only** shadow authority gates in Octomata under mixed-mode stress:

- cumulative gate (M1-style lifetime-ish memory)
- EMA/recency gate (first-order recency weighted)

The purpose is to stress reason attribution and recovery/block timing under production-shaped interleavings (match/fallback/stale/late/physical-not-ready/cancelled). This pass does **not** change native Prometheus P15 M8/M9/M10 behavior or dispatch authority.

## Run

```bash
go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M2
go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M2 --suite Experiments.PrometheusShadowAuthorityRakeLab.M2
go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M2 --suite Experiments.PrometheusShadowAuthorityRakeLab.M2.FlowSmoke
go run ./cmd/oct test Experiments/PrometheusShadowAuthorityRakeLab/M2 --suite Experiments.PrometheusShadowAuthorityRakeLab.M2.FlowSmoke --execution compiled
go run ./cmd/oct artifact Experiments/PrometheusShadowAuthorityRakeLab/M2
```

## Outputs

- `shadow_authority_m2_rake_report.octagon`
- `shadow_authority_m2_rake_report.md`
- `scenario_metrics.csv`
- `scenario_summary.json`
- `FINDINGS.md`

## Json migration (2026-10-07)

`scenario_summary.json` is a `ScenarioSummary` record published with
`Artifact.WriteJson(path, value)`; it was JSON text built with
`String.Concat`. The values are the same. The keys are the record's field
names (`ScenarioCount`, was `scenarioCount`), in the layout `Json` writes. The
`[Artifact]` function loads the summary back with `Json.Load<ScenarioSummary>`
and checks its count; `oct artifact` of this directory failed on the old
read-back and passes now.
