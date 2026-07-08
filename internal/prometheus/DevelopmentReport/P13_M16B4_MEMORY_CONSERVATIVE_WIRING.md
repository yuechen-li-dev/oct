# P13 M16b-4 — Memory-Conservative SGEMM Variant Wiring

`PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE` is now a real EVT-wired SGEMM occupancy variant instead of an alias placeholder.

Implemented in this milestone:

- `reactor_vulkan_memory_conservative_spirv.h` is now loaded as its own Vulkan shader module and compute pipeline in `reactor_vulkan_sgemm.c`.
- Tiled benchmark and production dispatch now bind `memory_conservative_pipeline` when `requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE`.
- Variant diagnostics now report:
  - `requested/executed = memory-conservative`
  - `path_status = wired`
  - `path_id = memory_conservative`
  - `fallback_reason = none`
- Benchmark explicit dispatch coverage now includes memory-conservative identity and CPU-oracle correctness checks.

Repo-reality deviation from `internal/prometheus/native/README memory conservative.md`:

- The README only called out path-status promotion. In current repo reality, a distinct diagnostic `path_id` was also required so the new pipeline no longer masquerades as the baseline path after wiring.

Deferred intentionally:

- No selector redesign or retuning beyond preserving existing reachability.
- No SAFE-mode relaxation.
- No P15 mismatch correction.
- No new DVT/PVT/production eligibility claims.

Production-selection coverage note:

- The judgment engine already has selector logic that can recommend `memory-conservative`, and Marionette unit coverage now asserts that reachability directly.
- A full runtime production-path test that deterministically forces a low-register/low-workgroup device profile is still deferred because the current runtime test seam does not expose capability overrides for those Vulkan-derived occupancy classes.
