# P13 M17 — Diagnostics Truth Separation Hardening

## 1) DVT-2 assumption issues

DVT-2 showed ambiguity between selector recommendation, benchmark request, and executed variant in occupancy artifacts. Prior artifact schema (`prometheus.sgemm.occupancy_dvt2.rtx3070.v1`) did not make all truth dimensions explicit in a single record.

## 2) Truth dimensions defined

M17 enforces explicit, independent fields for:

- Device capability: `dedicated_transfer_available`, `device_supports_fp16`
- Selector recommendation: `selector_recommended_variant`
- Benchmark request: `requested_variant`
- Runtime execution: `executed_variant`, `path_id`, `path_status`, `fallback_reason`
- Runtime feature selection: `runtime_selected_fp16`
- Resource usage: `transfer_queue_selected`, `transfer_queue_used`, queue family fields, lease counters
- Timing truth: timestamp availability/validity and timing source/confidence fields

## 3) Schema changes

- Updated occupancy DVT artifact schema ID to `prometheus.sgemm.occupancy_dvt2.rtx3070.v2`.
- Added explicit `selector_recommended_variant` field.
- Added explicit separation between transfer selection and transfer usage.
- Added explicit runtime feature block for FP16 selection vs capability.

## 4) Test changes

- Added `P13_M17_DvtArtifactTruthFieldsSeparated` Marionette test.
- Extended DVT validation assertions to check that selector/request/executed identities remain independently representable.

## 5) Examples of separated truth

- Selector recommends SRT while request/execution can remain B2x2.
- Transfer queue selected by policy can differ from transfer queue used.
- Device FP16 capability can be present while runtime FP16 selection remains off.

## 6) Impact on future DVT/PVT runs

These changes reduce artifact ambiguity and prevent downstream report logic from conflating policy recommendation, benchmark request, runtime execution, and runtime actuation in a single "variant" interpretation.

## Inconsistency surfaced

The DVT artifact path still contains `rtx3070` in the schema namespace for historical continuity, even though M17 is cloud-safe/generalized truth hardening. The truth-model split is now explicit, but schema naming remains hardware-flavored.
