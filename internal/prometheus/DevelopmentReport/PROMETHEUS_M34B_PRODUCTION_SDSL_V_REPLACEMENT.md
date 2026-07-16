# Prometheus M34b production SDSL-V replacement

Status: **COMPLETE — success** (2026-07-11)

## Production ownership

| ID | shader | source | entry point | header | footprint |
|---:|---|---|---|---|---|
| 10 | SRT-2accum-K | `internal/prometheus/shaders/sdslv/production/sgemm/sgemm_srt_2accum_k.sdslv` | `SgemmSrt2AccumK_CS` | `reactor_vulkan_sgemm_srt_2accum_k_spirv.h` | 1x1 |
| 11 | B2x2-row-major-biased | `internal/prometheus/shaders/sdslv/production/sgemm/sgemm_b2x2_row_major_biased.sdslv` | `SgemmB2x2_CS` | `reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h` | 2x2 |
| 12 | A2x4-row-biased-accum8 | `internal/prometheus/shaders/sdslv/production/sgemm/sgemm_a2x4_row_biased_accum8.sdslv` | `SgemmA2x4_CS` | `reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h` | 2x4 |

The generated names are the deliberate entry-point policy. Registry IDs,
implementation IDs, policy-visible names, bindings 0/1/2, push constants
`{m,n,k}` (12 bytes), and 8x8x1 workgroups are unchanged. Packed4 and FP16
remain historical SPIR-V assets.

The old headers are no longer production registry dependencies; the M34a audit
source retains them as historical comparison fixtures.

## Generation and rollback

Run `powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1`.
It validates manifest-owned SDSL-V, emits HLSL/SPIR-V under `out/sdslv`, and
regenerates checked-in headers. The script removes native temporary outputs.

Rollback restores `reactor_shader_registry.c`, `native/shaders/manifest.json`,
`native_manifest.json`, and the three generated headers. Historical headers
remain at their original paths for audit evidence. Re-run the generator,
`spirv-val --target-env vulkan1.0`, manifest check, and Windows native build.

## Metadata and evidence

A2x4 implementation 5 now uses canonical 2x4 dispatch metadata: an 8x8 group
covers 16x32 logical outputs. The permanent registry test proves M=3,N=17,K=7
uses one Y group while preserving A2x4 policy identity.

Generated module SHA-256 values: SRT (2,268 bytes)
`78ee174ba47fa508b1fc4baa41f150726f5fc56cc2b28a7d564b882a0cdcd802`; B2x2
(3,776 bytes) `900fda8225422ae85fd211bb41e639a9b3dbbc10118c111f711554b46403042e`;
A2x4 (5,980 bytes) `be00306ed7e0ad8f3684c3c60b2f6de37d64b9cc651fcf1ad4653a682b8c9f15`.
All three passed `spirv-val`, the Windows native rebuild, new registry tests,
and M34a's archived original-versus-candidate harness.

## Validation-enabled production lane

`PROMETHEUS_VK_VALIDATION=1` is a narrow opt-in used only by the M34b native
test. It enumerates `VK_LAYER_KHRONOS_validation`, fails instance creation when
the requested layer is missing, requests that exact layer, enables
`VK_EXT_debug_utils`, and installs an instance-lifetime callback. Default
runtime creation is unchanged. The callback captures total/warning/error counts
and the latest severity, type, message ID, and text.

The rebuilt MSVC test `PrometheusM34bValidationEnabledProductionVariants`
passed on the real Vulkan device: requested=true, available=true, enabled=true,
enabled layers=`VK_LAYER_KHRONOS_validation`, debug-utils active=true, warnings=0,
errors=0, device loss=false. It executed production implementation IDs 3/4/5
(SRT/B2x2/A2x4), through registry pipelines, descriptor writes, 12-byte push
constants, dispatch, synchronization, readback, and CPU-oracle comparison.
A2x4 used M=3,N=17,K=7 with canonical 2x4 metadata and one Y group.

M34a's archived original comparison also passed after this rebuild. Prior
generated timing evidence remains SRT 8,256 ns, B2x2 10,912 ns, and A2x4
31,776 ns median; A2x4 optimization remains a focused follow-up.
