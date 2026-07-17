# Prometheus Numerical Heterogeneity Lab

This experiment is the synthetic design companion to Prometheus M49. It models depth-wise numerical-error recurrence across conventional, cooperative, and A2x4 paths, evaluates source-side mitigation candidates on a separate held-out split, and emits an Octagon report plus CSV, JSON, Markdown, and PNG artifacts.

The experiment has no product authority. Native matched-input CPU/GPU audits remain the measurement source of truth, and M48 EVT remains postponed while M49 is in progress.

Run:

```powershell
go run ./cmd/oct test Experiments/PrometheusNumericalHeterogeneityLab
go run ./cmd/oct artifact Experiments/PrometheusNumericalHeterogeneityLab
```
