# P12 M2 — Vulkan Common Extraction Report

## 1) M1 handoff summary

P12 M1 defined the staged topology migration with `reactor_vulkan_common.c` as the destination for only low-risk shared Vulkan plumbing, while SGEMM runtime policy/execution remains locally auditable in the SGEMM reactor file. M2 implements the first extraction step without renaming `reactor_vulkan.c` yet.

## 2) Candidate helper audit

### Safe to move now

- `set_status(...)` → moved as `prom_vk_set_status(...)`.
  - Reason: generic status-out parameter helper; no SGEMM state coupling.
- `checked_mul_u32(...)` → moved as `prom_vk_checked_mul_u32(...)`.
  - Reason: generic overflow guard usable by any compute family.
- `find_memory_type(...)` → moved as `prom_vk_find_memory_type(...)`.
  - Reason: generic Vulkan memory-type discovery utility.
- `create_buffer(...)` / `destroy_buffer(...)` → moved as `prom_vk_create_buffer(...)` / `prom_vk_destroy_buffer(...)`.
  - Reason: generic Vulkan buffer/memory lifecycle plumbing, reused by SGEMM today and applicable to future families.

### Keep in SGEMM

- `checked_float_buffer_size(...)`
  - Reason: tied to SGEMM matrix shape-to-buffer sizing call sites and SGEMM path behavior.
- `checked_packed_fp16_buffer_size(...)`
  - Reason: explicitly SGEMM Packed/FP16 layout behavior; non-common by M2 scope.
- Any slot/runtime/arena/artifact/transfer-selector/batch worker helpers.
  - Reason: SGEMM policy/runtime diagnostics ownership.

### Defer until after SGEMM rename

- Any broader command/fence helper extractions that currently interleave with SGEMM batch/async orchestration blocks.
  - Reason: higher coupling risk before `reactor_vulkan.c` → `reactor_vulkan_sgemm.c` rename/section cleanup.

### Not common

- Packed4/FP16 transforms and layout selectors.
- Typed arena ownership and artifact invalidation helpers.
- SGEMM controller and Dominatus integration helpers.

## 3) Helpers moved

- `set_status` → `prom_vk_set_status` in `reactor_vulkan_common.c`.
- `checked_mul_u32` → `prom_vk_checked_mul_u32` in `reactor_vulkan_common.c`.
- `find_memory_type` → `prom_vk_find_memory_type` in `reactor_vulkan_common.c`.
- `create_buffer` → `prom_vk_create_buffer` in `reactor_vulkan_common.c`.
- `destroy_buffer` → `prom_vk_destroy_buffer` in `reactor_vulkan_common.c`.

All SGEMM call sites were switched to these shared helpers; behavior and test-flag semantics were preserved.

## 4) Helpers intentionally left in SGEMM

- SGEMM path sizing helpers (`checked_float_buffer_size`, `checked_packed_fp16_buffer_size`).
- SGEMM runtime registry, slot lifecycle, typed arenas, transfer policy, async/batch orchestration.
- SGEMM diagnostics and public impl entrypoint logic.

This preserves the “single complete SGEMM reactor” auditability requirement in M2.

## 5) `reactor_vulkan.h` changes

Header changes were kept minimal:

- added shared Vulkan helper declarations for M2 extractions;
- added shared `prom_vk_buffer` type so common and SGEMM files can share buffer wrapper shape;
- kept existing `prom_reactor_runtime_*_impl` declarations unchanged for `reactor_api.c` forwarding stability.

No SGEMM-private runtime structs were moved into the header.

## 6) Build script updates

Added `reactor_vulkan_common.c` to both Linux and Windows native source lists:

- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

`reactor_vulkan.c` remains compiled in place (no rename in M2).

## 7) Behavior preservation evidence

- Native build helper succeeded.
- Required Marionette filter runs succeeded.
- Full Marionette suite succeeded (with expected environment-dependent skips and zero failures).

No public ABI symbol renames were introduced; `reactor_api.c` forwarding remains compatible.

## 8) Deferred scope

Explicitly deferred to later milestones:

- renaming `reactor_vulkan.c` to `reactor_vulkan_sgemm.c`,
- adding `reactor_vulkan_fft.c` / `reactor_vulkan_fused_reduction.c` stubs,
- SGEMM-internal sub-splitting,
- SGEMM policy/runtime behavior changes,
- public API changes.
