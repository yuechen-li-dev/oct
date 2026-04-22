# P8b Report — Vulkan Push Constant / Layout Hygiene Hardening

## Scope completed

P8b hardens the Vulkan SGEMM push-constant contract in `reactor_vulkan.c` by porting the M11 layout-hygiene rules into an explicit, compile-time-checked host/shader seam.

## M11 safe/unsafe patterns extracted and Vulkan mapping

### Extracted from M11 as required-safe

1. Host/shader field list and ordering must be identical.
2. No correctness dependency on implicit padding.
3. Avoid mixed-width interleaving and compact clever packing.
4. Struct evolution must be append-only.
5. Drift should be visible via explicit diagnostics/checks.

### Mapped into Vulkan C + shader contract

- **Identical host/shader field list/order**
  - Host push struct is now exactly `{m, n, k}` with no extra host-only field.
  - Shader-side contract remains the same three push constants in the embedded SGEMM SPIR-V comments and validated offsets.
- **No implicit padding reliance**
  - Contract constants pin field offsets (`0,4,8`) and total byte size (`12`), with `_Static_assert` checks on `offsetof` and `sizeof`.
- **No mixed-width fragility / no compact cleverness**
  - Push contract remains plain `uint32_t` scalars only.
- **Append-only evolution discipline**
  - Contract comment now explicitly states append-only evolution and synchronized host+shader updates.
- **Visible drift detection**
  - Compile-time assertions fail the build immediately on offset/size drift.
  - Push range byte count and `vkCmdPushConstants` byte count now use a shared explicit constant (`PROM_VK_SHADER_PUSH_BYTES`) instead of incidental `sizeof(...)` at callsites.

## Enforced explicit contract rules now in code

- Field order and offsets are explicit constants.
- Push-constant byte range is an explicit shared constant.
- Compile-time checks enforce host layout against shader contract assumptions.
- Code comments document synchronized host/shader update requirement.

## Checks/tests proving safer contract

Focused checks added (no broad framework):

1. `_Static_assert(offsetof(...))` for each push field.
2. `_Static_assert(sizeof(prom_vk_push) == PROM_VK_SHADER_PUSH_BYTES)`.
3. Runtime setup/dispatch use the same explicit byte constant (`PROM_VK_SHADER_PUSH_BYTES`) for pipeline push range and push command payload.

These checks make ordering/size drift and accidental compactness visible during compilation rather than silently at runtime.

## Deferred to later P8 milestones

- Any broader ABI-portability framework beyond this push constant seam.
- Staging, tiled SGEMM, async submission, and wider reactor redesign (explicitly out-of-scope for P8b).

## Inconsistency surfaced

M11 is protocol-level and does not model full Vulkan/std430/std140 ABI details. This hardening pass ports M11’s safety contract into explicit host checks and documented shader-side offsets, but does not claim full cross-vendor ABI simulation.
