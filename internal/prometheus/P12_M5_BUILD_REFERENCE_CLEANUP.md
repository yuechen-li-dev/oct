# P12 M5 — Build Reference / Documentation Cleanup Report

## 1) M4 handoff summary

P12 M4 completed topology shape creation by adding inert future-family stubs (`reactor_vulkan_fft.c`, `reactor_vulkan_fused_reduction.c`) and wiring them into Linux/Windows native build lists while preserving SGEMM behavior and capability reporting.

M5 scope in this report is strictly cleanup: confirm active references, align developer-facing topology documentation, verify build/source-list parity, and validate behavior preservation.

## 2) Reference audit results

Audit commands run:

- `rg "reactor_vulkan\.c"`
- `rg "reactor_vulkan_sgemm\.c"`
- `rg "reactor_vulkan_common\.c"`
- `rg "reactor_vulkan_fft\.c"`
- `rg "reactor_vulkan_fused_reduction\.c"`

Classification:

### Active build/source references

- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

Both active build helpers already reference current topology files:

- `reactor_vulkan_common.c`
- `reactor_vulkan_sgemm.c`
- `reactor_vulkan_fft.c`
- `reactor_vulkan_fused_reduction.c`

### Current documentation / dev guidance

- Prior to M5, no native-topology README existed under `internal/prometheus/native/`.
- M5 added `internal/prometheus/native/README.md` as concise current topology guidance.

### Historical report references

`reactor_vulkan.c` mentions remain in historical reports/design notes, including:

- `internal/prometheus/P8*.md` / `P9*.md` / `P10*.md`
- `internal/prometheus/P12_M1_VULKAN_REACTOR_FILE_TOPOLOGY_PLAN.md`
- `internal/prometheus/P12_M2_VULKAN_COMMON_EXTRACTION.md`
- `internal/prometheus/P12_M3_REACTOR_VULKAN_SGEMM_RENAME.md`
- `docs/reports/prometheus/P4E_REPORT.md`
- `Experiments/PrometheusSgemmAlgorithmLab/M29/REPORT.md`

These are historical milestone records and were kept as historical context.

### Comment/doc references needing update

- Added current topology note in `internal/prometheus/native/README.md`.

### Acceptable historical mentions

- Historical mentions of `reactor_vulkan.c` in pre-M3 reports are acceptable when describing timeline-accurate past state.

### Stale/broken references

- None found in active compile paths.
- No active build helper or active source list references `reactor_vulkan.c`.

## 3) Active references updated

Updated active developer guidance by adding:

- `internal/prometheus/native/README.md`

This file now documents authoritative current topology and guardrails:

- common/shared Vulkan plumbing in `reactor_vulkan_common.c`
- complete SGEMM family implementation in `reactor_vulkan_sgemm.c`
- inert future-family stubs in `reactor_vulkan_fft.c` and `reactor_vulkan_fused_reduction.c`
- internal header role for `reactor_vulkan.h`
- rule: split by primitive family, not SGEMM subsystem

No active source-list changes were required because build scripts were already consistent after M4.

## 4) Historical references left / clarified

Left unchanged intentionally:

- historical milestone docs and reports that accurately describe pre-M3 topology (`reactor_vulkan.c` era).

Clarification strategy applied in M5:

- keep historical docs intact;
- provide a clear current-state topology reference in `internal/prometheus/native/README.md`;
- document this distinction explicitly in this M5 report.

No mass rewrite of historical reports was performed.

## 5) Topology documentation added/updated

Added:

- `internal/prometheus/native/README.md`

Content includes:

- current Vulkan reactor file topology and roles,
- non-claim status for FFT/fused stubs,
- topology rule: split by primitive family, not by internal SGEMM subsystem,
- scope guardrails to keep SGEMM policy/runtime localized and `common` narrowly shared.

## 6) Build/source-list parity

Verified Linux and Windows helpers compile the same reactor topology in both library and Marionette builds:

- `reactor_vulkan_common.c`
- `reactor_vulkan_sgemm.c`
- `reactor_vulkan_fft.c`
- `reactor_vulkan_fused_reduction.c`

Files verified:

- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

Intentional divergence:

- none for reactor source topology.

## 7) Header scope check (`reactor_vulkan.h`)

`reactor_vulkan.h` currently contains:

- shared Vulkan helper declarations (`prom_vk_*`),
- shared common wrapper type (`prom_vk_buffer`),
- stable `prom_reactor_runtime_*_impl` declarations used by `reactor_api.c`.

It does **not** currently expose SGEMM-private runtime structs/state aggregates from `reactor_vulkan_sgemm.c`.

Header bloat assessment:

- no immediate god-header drift observed in current M5 scope.
- no additional header refactor required for this cleanup pass.

## 8) Validation results

Commands executed:

- `bash internal/prometheus/native/build_stub.sh`
- `out/prometheus/native/marionette_tests P11_M20`
- `out/prometheus/native/marionette_tests PrometheusReactor_Sgemm`
- `out/prometheus/native/marionette_tests`
- topology audit searches listed in section 2

Observed results:

- Native build succeeded.
- Targeted Marionette filters succeeded.
- Full Marionette suite succeeded (passes with expected environment-dependent skips).
- Search confirms active topology references are current and complete.

Behavior preservation:

- No runtime logic changes were made in reactor implementation files.
- M5 changes are documentation/reference cleanup only.

## 9) Deferred scope

Explicitly deferred (unchanged):

- further helper extraction,
- SGEMM internal split,
- FFT implementation,
- fused-reduction implementation,
- public API/ABI changes,
- SGEMM/runtime behavior changes,
- performance/benchmark work.
