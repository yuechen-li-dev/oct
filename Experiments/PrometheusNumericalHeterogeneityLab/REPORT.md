# Experiment report

M0 establishes a deterministic, non-authoritative simulation of the M49 numerical plant:

- explicit identification and held-out input families;
- depth-resolved recurrence with path/family-dependent injection;
- bounded source-side mitigation candidates with latency and memory costs;
- deterministic `when utility` selection;
- a shadow-only audit action whose decision cannot alter product execution;
- a native Octagon report plus reproducible CSV/JSON/Markdown and line-chart artifacts.

This lab is useful for testing the shape of the identification/controller contract. It does not establish real Vulkan accuracy, performance, determinism, or M48 EVT readiness.

M1 adds a strictly separate native mode. It loads a typed projection of the M49a RTX 3070 artifact, evaluates bounds fitted only from identification records, reports held-out support independently, and emits Octagon/CSV/JSON/Markdown/PNG outputs. The current import deliberately reports zero held-out support and therefore certifies no envelope or mitigation.

## Json migration (2026-10-07)

`M0SummaryJsonText` and `M1SummaryJsonText` built JSON text for
`Json.Object`. `M0BuildSummary` and `M1BuildSummary` build a record, published
with `Artifact.WriteJson(path, value)`, and each `[Artifact]` function loads
its summary back with `Json.Load<T>` and checks a count. `synthetic_summary.json`
and `native_import_summary.json` are regenerated: the same values, keys as the
record declares them (`CaseCount`, was `caseCount`), in the layout `Json`
writes. Every other published file is byte-identical. `oct artifact` of M0 and
M1 failed before this change, on the JSON read-back; it passes now.
