# P14 — Dominatus Subsystem Naming Hygiene

## 1) Dominatus subsystem meaning

For Prometheus documentation and internal comments, **Dominatus subsystem** means:

- Dominatus Judgment Engine decision layer,
- Dominatus HFSM / stack state-machine lifecycle layer,
- Dominatus blackboard staged/visible fact store,
- Dominatus lease-control seam,
- planned Dominatus measurement-filter seam,
- planned Dominatus predictor / lease-ahead seam.

This naming clarifies that Dominatus is the control subsystem boundary, not only a blackboard data structure.

## 2) Files/comments/docs updated

- `internal/prometheus/native/reactor_dominatus_blackboard.h`
  - Added subsystem-scoping comment so the blackboard is explicitly documented as one part of the Dominatus subsystem.
- `internal/prometheus/DevelopmentReport/P13_M9_RESOURCE_LEASE_CONTROLLER.md`
  - Updated wording to avoid implying “Dominatus = blackboard only”.
  - Clarified that blackboard integration is one seam in a broader control stack including Judgment Engine and slot HFSM.

## 3) Intentionally not renamed

To keep this pass low-risk and behavior-neutral, broad symbol/API renames were intentionally skipped:

- No C ABI type/function renames (`prom_dom_*`, `prom_judgment_*`, HFSM symbols).
- No broad test case renames.
- No file/module renames.

Reason: these would cause avoidable churn and are not required to fix subsystem-boundary clarity in docs/comments.

## 4) Validation results

Validation commands for this hygiene pass:

- `go test ./...`
- `bash internal/prometheus/native/build_stub.sh`
- `out/prometheus/native/marionette_tests ResourceLease`
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm`

Results are recorded from the command outputs run after the documentation/comment updates.
