# P11 M2 / Prometheus SGEMM Algorithm Lab M38 — Executable Typed-Arena Rake Simulation (Follow-up)

## 1) Audit of prior M38 weakness (static artifact tables)

The previous M38 pass was not a true simulation. It relied on precomputed arrays in:

- `M38ArenaPolicyComparisonFrom()`
- `M38WorkloadRakeMatrixFrom()`
- `M38ImplementationContractFrom()`

Those functions returned fixed values, so tests only validated constants and string membership.

### What was static/precomputed

1. Policy comparison counts/ratios/recommendation labels were literal arrays.
2. Workload matrix rows (scenario/policy outcomes) were literal arrays.
3. Contract recommendation and rules were static strings with no model-derived selection.

### Which tests only validated static expectations

- `M38ProvidesExecutableSimulationBackstopForReportClaims`
- `M38RejectsExactFitRebuildAsPrimaryPolicyDueToThrash`
- `M38CompatibilityNamespacesPreventUnsafeRepresentationReuse`
- `M38RakeMatrixCoversRequiredScenarioSet`
- `M38RakeMatrixKeepsOwnershipSafeAndBudgetFailuresExplicit`
- `M38ContractStatesExactReuseAndFailureRules`

All of these checked values from prewritten tables rather than model transitions.

### Unsupported report claims in old pass

The old report discussed grow/shrink/rebuild dynamics, budget failure behavior, ownership safety, and dependency invalidation as if they were computed outcomes. They were not executed by policy logic.

### What is now replaced

This follow-up replaces static tables with:

- executable arena/request/decision state transitions,
- generated workload sequences,
- policy simulation (A/B/C/D),
- computed metrics and computed recommendation,
- artifacts emitted from simulation outputs.

## 2) Executable arena model implemented

The `.oct` model now includes explicit typed arena state and request structures:

- arena: role, required/capacity/committed/live bytes, generation, dependency key, layout namespace, precision-storage namespace, memory location, owner slot, valid/failure/in-flight, low-usage/cooldown,
- request: role, m/n/k/k-padded, namespaces, memory location, required bytes, owner slot, in-flight flag, budget,
- decision: reuse/grow/shrink/rebuild/invalidation/rejections/failure/recovery and hazard/avoidance flags.

Core step function: `M38ApplyDecision(...)`.

## 3) Policies now executed

- **Policy A (grow-only)**: reuse when legal, grow on demand, no shrink.
- **Policy B (grow + conservative hysteresis shrink)**:
  - low-usage if `capacity > 2x required`,
  - shrink only after 6 low-usage epochs,
  - cooldown 4 epochs between shrinks,
  - no shrink while in-flight,
  - 64 MiB floor.
- **Policy C (exact-fit rebuild)**: rebuild on size difference/compatibility transitions.
- **Policy D (budget-aware grow-only)**: explicit budget rejection/failure events under pressure.

## 4) Workloads now generated and executed

Generated request sequences now cover:

1. steady same-shape,
2. M-only churn,
3. N-only churn,
4. K-only churn,
5. layout churn,
6. precision churn,
7. capacity growth then shrink,
8. fixed-double two-slot handoff,
9. N-slot preview,
10. budget pressure/failure.

## 5) Computed metrics (direct)

All below are computed from simulation state transitions (not constants):

- peak committed bytes,
- average committed bytes,
- average slack bytes,
- slack ratio average (permille),
- grow/shrink/rebuild/reuse/invalidation counts,
- budget rejection/failure/recovery counts,
- generation increment count,
- ownership rejection count,
- layout/precision mismatch rejection count,
- false invalidation avoided count,
- missed invalidation hazard count,
- reuse legal and grow/rebuild required counts.

No estimated-only metrics are used in this pass.

## 6) Computed policy outcome summary

The recommendation is now model-derived by score (`rebuild*3 + slack_permille + budget_rejections*5`) across aggregated scenarios.

Computed winner remains:

- **`typed-arenas-grow-with-conservative-hysteresis-shrink` (Policy B)**.

Reason in model terms:

- lower thrash than exact-fit,
- lower retained slack risk than pure grow-only,
- explicit reject path under budget pressure remains available.

## 7) Artifacts regenerated from computed model

Artifacts are now emitted from simulation-backed constructors:

- `m38_arena_policy_comparison.octagon`
- `m38_artifact_dependency_rake.octagon`
- `m38_growth_shrink_table.octagon`
- `m38_layout_precision_churn_table.octagon`
- `m38_budget_failure_table.octagon`
- `m38_nslot_preview_table.octagon`
- `m38_final_contract.octagon`

## 8) Test updates (behavioral, not static)

Updated `.octest` checks now validate:

1. compatibility + sufficient capacity reuse,
2. grow when required exceeds capacity,
3. explicit budget rejection,
4. layout mismatch rejection,
5. precision mismatch rejection,
6. exact-fit rebuild thrash vs A/B,
7. hysteresis behavior vs exact-fit ping-pong,
8. M/N/K dependency invalidation pattern for A/B/C,
9. fixed-double ownership rejection while in-flight,
10. recommendation derived from computed scoring function.

## 9) Final answers required by milestone

1. **Did the executable model confirm or change prior recommendation?** Confirmed.
2. **Which policy wins and why?** Policy B; it balances reuse and memory discipline while avoiding exact-fit thrash.
3. **Which metrics are computed directly?** All listed in section 5.
4. **Which metrics remain estimates?** None in this pass.
5. **What implementation contract should P11 M3/M39 use?** Typed arenas with namespace-gated compatibility, generation increments on grow/rebuild/shrink/invalidation, explicit budget rejection events, and slot ownership gating.
6. **Is another rake lab needed before native implementation?** Not required for allocator policy choice; optional narrow pre-native pass is still useful for Vulkan fence/queue and granularity edge validation.

## 10) Language/reference consistency note

No deliberate syntax deviation from repository Oct style was introduced. If any parser/runtime mismatch appears during CI, treat it as an implementation drift and surface it explicitly.
