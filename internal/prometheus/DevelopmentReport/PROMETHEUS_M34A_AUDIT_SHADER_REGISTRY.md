# Prometheus M34a audit shader registry

Status: **IN PROGRESS — meaningful progression** (2026-07-11)

## Implemented audit boundary

`native/reactor_prometheus_audit.{h,cpp}` is a Marionette-only component.  It
is listed only in `native_manifest.json`'s `native_test_sources`, never in
`production_sources`; neither `reactor_shader_registry.c` nor
`reactor_vulkan_sgemm.c` includes it.  Production selection therefore cannot
enumerate, find, or dispatch an audit candidate.

`PrometheusAuditShaderDescriptor` uses named fields for identity, SPIR-V
ownership (embedded words or a file), explicit entry point, workgroup,
`outputs_per_invocation_m/n`, packing, precision, provenance, and comparison
group.  The small `PrometheusAuditShaderRegistry` accepts explicit embedded or
file registrations and only exposes deterministic name lookup/enumeration.

The file path validates nonempty, four-byte-aligned SPIR-V, magic, requested
compute entry point, `LocalSize`, and descriptor workgroup agreement before a
future Vulkan pipeline may be created.  It produces a FNV-1a module hash,
canonical dispatch geometry, deterministic replay identity, and compact JSON.

## Canonical footprint contract

| Shader | Rows/invocation | Columns/invocation |
|---|---:|---:|
| SRT-2accum-K | 1 | 1 |
| B2x2-row-major-biased | 2 | 2 |
| A2x4-row-biased-accum8 | 2 | 4 |
| Packed4FP32 | 1 | 1 |
| FP16-storage/FP32-accum | 1 | 1 |

The audit helper computes `groupsX = ceil(M / (localX * rows))` and
`groupsY = ceil(N / (localY * columns))`.  It refuses zero/overflowing
footprints and dispatches that do not cover M×N.  The A2x4 audit test proves
M=3,N=5 maps to one 8×8 workgroup using the truthful 2×4 contract.

## A2x4 historical metadata trace

The production 2×2 value is `k_meta_reg2x2` for implementation ID 5 in
`reactor_shader_registry.c`. `reactor_sgemm_dispatch_metadata.h` consumes it
directly for geometry, so it affects behavior: it over-dispatches the actual
2×4 shader in N, subject to the shader's guarded stores.  Buffer allocation
and CPU correctness output footprint are M×N and do not consume it.  Occupancy
and judgment code consume the historical `AGGRESSIVE_4X4_ACCUM8` label for
policy/telemetry, not output sizing.  P13/PX16 benchmark rows report metadata
for generated SDSL variants but do not correct the historical A2x4 descriptor.
No production metadata was changed.

## Remaining execution seam

The existing real execution stack is concentrated in
`reactor_vulkan_sgemm.c`: it selects/creates a pipeline, packs scalar/float4/
u32-half inputs, writes set-0 bindings 0/1/2, pushes `{m,n,k}` (12 bytes),
records the command buffer, timestamps, synchronizes, and reads C back.  Its
smallest remaining seam is an explicit *test-only* pipeline and dispatch
metadata override at the `selected_pipeline`/`dispatch_geometry` boundary.
That must be threaded through the existing function rather than copied into a
shadow host stack. It is not implemented in this progression, so no candidate
has been run on Vulkan and no replacement decision changes are warranted.

## Tests and evidence

`reactor_prometheus_audit_tests.cpp` covers embedded registration, production
registry isolation by construction, bad magic, bad byte length, missing entry,
workgroup mismatch, file loading, canonical A2x4 dispatch, deterministic
replay, and JSON footprint output. A standalone Marionette build ran all four
tests successfully. The normal Windows native build launcher failed before
compilation because the current shell's pre-existing MSVC environment lacks
standard-library include paths (`stdint.h` cannot be found); the audit source
itself compiled with the installed UCRT64 C++23 compiler.

## Production-safety proof

`git diff` confirms `reactor_shader_registry.c` and
`native/shaders/manifest.json` are unchanged. Audit code is test-source-only;
deleting it and its test registration cannot alter production IDs, assets,
manifests, selection policy, or packaged reactor binaries.

## M34b handoff

No `.sdslvbench`, `[Benchmark]`, statistical benchmark framework, graphics
work, or M34b work was added. The next M34a step is the narrow Vulkan override
above, followed by the bounded five-pair workload matrix, validation layers,
and replacement recommendations.
