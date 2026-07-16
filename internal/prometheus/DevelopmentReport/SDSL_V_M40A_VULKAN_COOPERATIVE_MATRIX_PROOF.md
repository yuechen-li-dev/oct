# SDSL-V M40a Vulkan Cooperative Matrix Proof

## Outcome

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
Production classification: **2 — research-capable but not yet production-worthy**

M40a answers all four bounded questions on the real target machine:

1. The RTX 3070 and NVIDIA 596.36 driver expose usable standardized
   `VK_KHR_cooperative_matrix` functionality.
2. The installed DXC can emit the required cooperative-matrix SPIR-V through
   a small compiler-owned HLSL intrinsic island.
3. A readable SDSL-V-owned proof executes correctly through Prometheus Vulkan
   with validation enabled and real timestamp-query timings.
4. The proof materially improves kernel time, but the present readback-heavy
   end-to-end path does not establish a strong realistic advantage over the
   best FP32 kernel when only weights are persistent.

The defensible result is a measured **cooperative-matrix hardware path**. This
report does not claim exact tensor-core occupancy or a particular physical
instruction issue rate.

## Scope and ownership

The proof remains experimental. No production shader ID, SGEMM selector rank,
or production selection policy changed.

| Item | Value |
|---|---|
| Experimental asset ID | `m40a-cooperative-sgemm-f16-f32-m16n16k16` |
| Implementation identity | `m40a-khr-subgroup-m16n16k16-f16-f16-f32-f32-aligned-v1` |
| Source | `internal/prometheus/shaders/sdslv/experimental/sgemm/cooperative/sgemm_cooperative_f16_f32_m16n16k16.sdslv` |
| Entry point | `CooperativeSgemmF16F32M16N16K16_CS` |
| Production authority | `experimental` |
| Selector eligible | `false` |
| Source SHA-256 | `872ef19abeb1d9a0f894fa238bd5e6b8ec1d9b8762a3e47ef5c3756aecb0b3b4` |
| SPIR-V SHA-256 | `247e410eb526f25c2276d127a732bb4def0c7949bca0ad0fdc5434ea95d17fea` |
| Runtime FNV shader hash | `b4d54fafc456757c` |

The manifest records this under `experimental_shader_assets`, outside the
production registry. The workspace checker rejects selector eligibility or
presence in the production C shader registry.

## Device and extension audit

The standalone probe is
`internal/prometheus/native/m40a_cooperative_matrix_probe.c`. Its deterministic
machine-readable result is
`DevelopmentReport/artifacts/M40A/capability_probe_rtx3070.json`.

| Property | Actual value |
|---|---|
| Vulkan SDK / loader | 1.4.350 / packed `4211038` |
| Requested instance API | 1.3 / packed `4206592` |
| Physical-device API | 1.4.329 / packed `4211017` |
| Device | NVIDIA GeForce RTX 3070 |
| Vendor / device ID | `0x10de` / `0x2488` |
| Driver | NVIDIA proprietary, driver ID 4, 596.36, packed `2500395008` |
| Subgroup size | 32 |
| Cooperative compute stages | `VK_SHADER_STAGE_COMPUTE_BIT` (`32`) |
| Supported subgroup operations | packed mask `2047` |

All relevant exposed paths were queried, but only the KHR path was enabled:

| Extension | Spec | Feature | Property function | M40a use |
|---|---:|---:|---|---|
| `VK_KHR_cooperative_matrix` | 2 | true | `vkGetPhysicalDeviceCooperativeMatrixPropertiesKHR` | selected and enabled |
| `VK_NV_cooperative_matrix` | 1 | true | `vkGetPhysicalDeviceCooperativeMatrixPropertiesNV` | audited only |
| `VK_NV_cooperative_matrix2` | 1 | all seven probed features true | `vkGetPhysicalDeviceCooperativeMatrixFlexibleDimensionsPropertiesNV` | audited only |

The KHR property count is 11. The legacy NV property count is 10. The NV2
flexible-dimension property count is 25; it also reports workgroup maximum 256,
maximum flexible dimension 1024, and 8192 bytes reserved shared memory. These
NV facts did not influence KHR tuple selection and the device was not created
with both vendor and standardized cooperative paths.

The enabled feature chain is bounded to:

- `VkPhysicalDeviceCooperativeMatrixFeaturesKHR.cooperativeMatrix`;
- `VkPhysicalDeviceShaderFloat16Int8Features.shaderFloat16`;
- `VkPhysicalDeviceVulkanMemoryModelFeatures.vulkanMemoryModel`.

`cooperativeMatrixRobustBufferAccess` is false and is neither required nor
enabled. Ordinary startup treats the extension as optional. The environment
seam `PROMETHEUS_VK_DISABLE_COOPERATIVE_MATRIX=1` proves an extension-absent
device still starts and retains ordinary Vulkan SGEMM.

Runtime capability state is explicit: unavailable, extension/no useful tuple,
useful tuple, compiler route unavailable, feature enabled, and executable.
Successful proof dispatch promotes the state to executable.

## Exact KHR cooperative matrix properties

All tuples below have subgroup scope, support multiply-accumulate, and report
non-saturating accumulation.

| M | N | K | A | B | C | Result | Usefulness |
|---:|---:|---:|---|---|---|---|---|
| 16 | 8 | 8 | f16 | f16 | f16 | f16 | FP16-like inference |
| 16 | 8 | 8 | f16 | f16 | f32 | f32 | FP16 inference / FP32 accumulation |
| 16 | 8 | 16 | f16 | f16 | f16 | f16 | FP16-like inference |
| 16 | 8 | 16 | f16 | f16 | f32 | f32 | FP16 inference / FP32 accumulation |
| 16 | 8 | 32 | s8 | s8 | s32 | s32 | integer; outside first proof |
| 16 | 8 | 32 | u8 | u8 | u32 | u32 | integer; outside first proof |
| 16 | 16 | 16 | f16 | f16 | f16 | f16 | FP16-like inference |
| **16** | **16** | **16** | **f16** | **f16** | **f32** | **f32** | **selected** |
| 16 | 16 | 16 | bf16 | bf16 | f32 | f32 | useful inference tuple; unsupported by current SDSL-V scalar/storage contract |
| 16 | 16 | 32 | s8 | s8 | s32 | s32 | integer; outside first proof |
| 16 | 16 | 32 | u8 | u8 | u32 | u32 | integer; outside first proof |

Selection is deterministic and requires the KHR extension, cooperative feature,
shader f16, Vulkan memory model, subgroup size 32, and the exact selected tuple.
No NVIDIA architecture marketing assumption participates in selection.

## Compiler route investigation

Installed tools:

- Vulkan SDK `C:\VulkanSDK\1.4.350.0`;
- DXC SDK build `1.9.0.5347`, binary identity
  `1.10.5347-fe261573`;
- SPIRV-Tools `v2026.2`.

Minimal local DXC probes established that this binary accepts generic HLSL
SPIR-V extension machinery: `vk::SpirvOpaqueType` and
`[[vk::ext_instruction]]`. It accepts the cooperative types and operations with
`cs_6_9`, Vulkan 1.3 / SPIR-V 1.6, native 16-bit types, and Vulkan memory model.
The generated probes contained cooperative instructions and validated.

The chosen route is route 2: a small compiler-owned HLSL intrinsic island.
It is preferable here because it:

- keeps SDSL-V as the readable source authority;
- uses the already installed and owned DXC path;
- adds no Slang or GLSL frontend;
- avoids a direct SPIR-V builder;
- hides Khronos syntax from ordinary SDSL-V source;
- is closed to one audited target contract.

The emitted DXC flags are:

```text
-spirv -T cs_6_9 -fspv-target-env=vulkan1.3 -O3
-fspv-use-vulkan-memory-model -enable-16bit-types
```

Two fresh generations produced byte-identical output and matched the committed
artifacts:

| Artifact | SHA-256 |
|---|---|
| Generated HLSL, first / second / committed | `09e171fa3415951bed2d4c2efcf2797b2edb04d0318ea1a08f45ba8dfde2551c` |
| Generated SPIR-V, first / second / committed | `247e410eb526f25c2276d127a732bb4def0c7949bca0ad0fdc5434ea95d17fea` |

## SDSL-V semantic abstraction

The only ordinary-source operation added is:

```text
CooperativeMatMul<F16F32M16N16K16Subgroup>(A, B, C, groupId,
                                            groupIndex, m, n, k)
```

Validation closes the contract to exactly eight arguments, the selected tag,
the declared resource types/access, and compute `[numthreads(32,1,1)]`.
Unsupported contract, arity, resource, stage, or local-size uses fail at compile
time with source spans and stable `SDSL-V4001` through `SDSL-V4005`
diagnostics. VD-MIR carries the backend-neutral intrinsic identity and explicit
capability requirements. No fragment generics, user templates, or raw backend
extension names were added to ordinary SDSL-V syntax.

The manifest requirements are:

- Vulkan extension `VK_KHR_cooperative_matrix`;
- Vulkan features `cooperativeMatrix`, `shaderFloat16`, and
  `vulkanMemoryModel`;
- SPIR-V extension `SPV_KHR_cooperative_matrix`;
- SPIR-V capabilities `CooperativeMatrixKHR`, `Float16`, and
  `VulkanMemoryModel`.

## Kernel and data contract

The proof implements row-major `C[M,N] = A[M,K] * B[K,N]`.

| Contract item | Value |
|---|---|
| Logical caller input | finite f32 values |
| Shader storage | f16 pairs packed into u32, low lane first |
| Conversion | trusted existing host half conversion, outside kernel timing |
| A/B tile | 16x16 f16 staged into workgroup memory |
| Accumulator / output | f32 / f32 |
| Work ownership | one full 32-lane subgroup per 16x16 output tile |
| Local size | 32x1x1 |
| Dispatch | `(M/16, N/16, 1)` |
| Alignment | M, N, and K must each be divisible by 16 |
| Tail policy | reject before pipeline creation |

Persistent-weight mode retains already packed B while repacking/reuploading A.
Persistent-input mode retains both packed inputs. Conversion and packing remain
outside kernel-only time and inside end-to-end time.

## SPIR-V validation and inspection

`spirv-val --target-env vulkan1.3` passes. The committed machine-readable
inspection is `artifact_inspection.json` beside the shader.

The disassembly proves:

- `OpCapability CooperativeMatrixKHR`;
- `OpCapability Float16` and `OpCapability VulkanMemoryModel`;
- `OpExtension "SPV_KHR_cooperative_matrix"`;
- three cooperative matrix types for f16 A, f16 B, and f32 accumulator;
- two `OpCooperativeMatrixLoadKHR` operations;
- one `OpCooperativeMatrixMulAddKHR`;
- one `OpCooperativeMatrixStoreKHR`;
- no ordinary scalar multiply loop for accumulation;
- GLCompute entry point and `LocalSize 32 1 1`;
- set 0 bindings 0/1/2 and a 12-byte M/N/K push-constant contract.

This establishes cooperative SPIR-V lowering; extension exposure alone was not
treated as execution evidence.

## Vulkan execution proof

The experimental path reuses the Prometheus instance, physical/logical device,
compute queue, command pool/buffer, descriptor layout/set, pipeline layout,
validation callbacks, buffers, query pools, and audit execution owners. There
is no second runtime and no CPU execution fallback.

With validation enabled, sizes 16, 256, 512, and 1024 each ran one warmup plus
three timestamped repetitions. Every output matched the deterministic CPU-side
identity-matrix oracle within `0.002`; all values remained finite. Pipeline
creation and dispatch succeeded, no device loss occurred, the query-pool
timestamps were valid, and validation reported zero errors.

The awkward 257x257x257 case is rejected before pipeline creation. Five host
measurements recorded median below the Windows clock resolution and maximum
500 ns. This is an honest reject strategy, not hidden padding or fallback.

## Kernel-only benchmark

All rows used the same deterministic logical row-major computation, one warmup,
five measured timestamp-query samples, validation enabled, and correctness
readback. Times are nanoseconds. Production-selected exposes p95 rather than a
separate maximum through its existing resident benchmark API.

| Size | Kernel | Precision | Min | Median | Max / p95 | Effective GFLOP/s |
|---:|---|---|---:|---:|---:|---:|
| 256 | tiled | f32 | 113,376 | 114,304 | 114,496 | 293.6 |
| 256 | memory-conservative | f32 | 148,576 | 149,120 | 150,528 | 225.0 |
| 256 | B2x2 | f32 | 78,720 | 79,616 | 80,128 | 421.5 |
| 256 | A2x4 | f32 | 116,928 | 117,792 | 118,304 | 284.9 |
| 256 | Packed4 | f32 | 147,776 | 148,416 | 148,896 | 226.1 |
| 256 | FP16-storage | f16 input / f32 accum | 154,688 | 155,488 | 156,512 | 215.8 |
| 256 | cooperative | f16 input / f32 accum | 33,568 | **33,728** | 33,888 | **994.9** |
| 256 | production-selected | f32 policy result | 153,216 | 155,392 | 157,888 p95 | — |
| 512 | tiled | f32 | 805,024 | 805,408 | 805,696 | 333.3 |
| 512 | memory-conservative | f32 | 1,076,896 | 1,078,176 | 1,078,976 | 249.0 |
| 512 | B2x2 | f32 | 583,584 | 585,280 | 585,920 | 458.6 |
| 512 | A2x4 | f32 | 368,320 | 368,640 | 369,824 | 728.2 |
| 512 | Packed4 | f32 | 1,096,672 | 1,098,720 | 1,110,560 | 244.3 |
| 512 | FP16-storage | f16 input / f32 accum | 1,099,008 | 1,101,600 | 1,103,616 | 243.7 |
| 512 | cooperative | f16 input / f32 accum | 125,440 | **125,664** | 130,752 | **2,136.1** |
| 512 | production-selected | f32 policy result | 1,111,168 | 1,116,960 | 1,139,424 p95 | — |
| 1024 | tiled | f32 | 6,308,288 | 6,314,688 | 6,317,760 | 340.1 |
| 1024 | memory-conservative | f32 | 8,495,392 | 8,496,352 | 8,499,200 | 252.8 |
| 1024 | B2x2 | f32 | 4,295,360 | 4,296,128 | 4,297,120 | 499.9 |
| 1024 | A2x4 | f32 | 2,558,848 | 2,561,568 | 2,636,448 | 838.3 |
| 1024 | Packed4 | f32 | 8,504,064 | 8,528,672 | 8,552,192 | 251.8 |
| 1024 | FP16-storage | f16 input / f32 accum | 8,601,376 | 8,603,424 | 8,605,376 | 249.6 |
| 1024 | cooperative | f16 input / f32 accum | 1,010,912 | **1,013,728** | 1,017,472 | **2,118.4** |
| 1024 | production-selected | f32 policy result | 4,422,400 | 4,427,040 | 4,521,792 p95 | — |

Median cooperative speedups:

| Size | vs best conventional FP32 | vs conventional FP16-storage | vs production-selected |
|---:|---:|---:|---:|
| 256 | 2.36x (B2x2) | 4.61x | 4.61x |
| 512 | 2.93x (A2x4) | 8.77x | 8.89x |
| 1024 | **2.53x (A2x4)** | **8.49x** | **4.37x** |

The precision distinction is material: cooperative and conventional
FP16-storage rows consume f16-rounded inputs and accumulate/output f32; FP32
rows consume f32 inputs. The benchmark does not present these as identical
input precision.

## Preparation and end-to-end cost

Preparation measurements use 512x512x512, one warmup, five measured samples,
pure device-local storage, and correctness readback. Values below are median
nanoseconds.

| Kernel | Mode | Kernel | Convert/pack | Upload | Readback | Preparation total | End-to-end |
|---|---|---:|---:|---:|---:|---:|---:|
| A2x4 | reupload | 368,704 | 88,200 | 86,464 | 3,659,944 | 3,834,344 | 4,684,600 |
| A2x4 | persistent B | 373,440 | 41,900 | 45,568 | 3,696,924 | 3,787,392 | 4,658,000 |
| A2x4 | persistent A+B | 367,200 | 100 | 128 | 3,863,884 | 3,864,212 | 4,689,500 |
| FP16-storage | reupload | 1,098,752 | 869,800 | 47,264 | 3,570,524 | 4,476,028 | 6,019,900 |
| FP16-storage | persistent B | 1,096,704 | 509,200 | 26,336 | 3,677,704 | 4,208,212 | 5,795,400 |
| FP16-storage | persistent A+B | 1,101,984 | 100 | 160 | 3,489,508 | 3,489,868 | 5,263,100 |
| cooperative | reupload | 184,928 | 856,700 | 165,760 | 3,428,400 | 4,450,192 | 5,185,000 |
| cooperative | persistent B | 425,984 | 487,100 | 86,336 | 3,294,504 | 3,866,928 | 4,976,400 |
| cooperative | persistent A+B | 417,600 | 0 | 160 | 3,513,716 | 3,513,876 | 4,574,800 |

End-to-end cooperative speedup is 1.16x versus conventional FP16 with
reupload, 1.17x with persistent weights, and 1.15x with both inputs persistent.
Against A2x4 it is 0.90x, 0.94x, and 1.03x respectively. Readback dominates
all three policies, so the large kernel-only gain is mostly hidden. Persistent
weights alone—the more realistic inference assumption for changing
activations—does not beat A2x4 end-to-end in this experiment.

Every benchmark row carries a stable replay identity derived from the
deterministic input generator version, kernel, dimensions, preparation mode,
and shader hash or executed production variant. Full min/median/max data and
identities are committed in
`DevelopmentReport/artifacts/M40A/cooperative_benchmark_rtx3070.json`.

## Evidence for dedicated matrix hardware

Evidence, in descending reliability:

1. The module contains KHR cooperative matrix types, loads, multiply-add, and
   store operations and no scalar accumulation loop.
2. The exact KHR feature/property tuple is active in the logical-device chain.
3. The module creates a Vulkan pipeline, executes on the RTX 3070, passes
   correctness and validation, and has no CPU fallback.
4. Stable 1024 kernel throughput is 2.53x the best conventional FP32 proof and
   8.49x conventional FP16-storage under the stated precision contract.

No Nsight or proprietary instruction disassembly was required or used. These
facts support “cooperative-matrix hardware path,” not a claim of measured
physical tensor-core occupancy.

## Tests and validation

Permanent non-hardware coverage includes probe parsing, deterministic tuple
ordering/selection, missing-feature and unsupported-tuple rejection, intrinsic
type/shape/stage/local-size validation, VD-MIR capability requirements,
toolchain flags and manifest capabilities, extension-absent startup, SPIR-V
opcode and dispatch geometry assertions, half packing through the reused
runtime owner, aligned/tail validation, correctness comparison, benchmark
artifact schema, and replay identity stability.

Hardware-gated coverage includes feature enablement, 16/256/512/1024 aligned
correctness and repeated query timestamps, 257 rejection, validation, the
portfolio comparison, and preparation modes.

The following passed on this Windows RTX 3070 machine:

- all requested Go commands from `go test ./internal/source` through
  `go test ./internal/... ./cmd/oct`;
- `go test ./tools/build_sidecars`;
- `go run ./tools/prometheus_native_manifest -check`;
- `go run ./tools/sdslv_workspace_check`;
- Windows native rebuild;
- standalone capability probe and semantic comparison with the committed JSON;
- two deterministic cooperative shader generations;
- `spirv-val --target-env vulkan1.3` and disassembly assertions;
- `marionette_tests.exe M40a`: 3/3 passed, zero skipped;
- isolated `marionette_benchmarks.exe M40a`: one validated benchmark passed;
- extension-absent simulation;
- `bash -n internal/prometheus/native/build_linux.sh`;
- `git diff --check`.

Linux source compatibility is preserved through extension guards and optional
feature negotiation. Only the Linux build-script syntax was checked here; no
Linux cooperative hardware execution is claimed.

## Limitations and M40b recommendation

Limitations are deliberately explicit:

- one exact f16/f16/f32/f32 subgroup tuple;
- aligned M/N/K multiples of 16 only;
- tails reject rather than pad or mix with conventional work;
- logical f32 inputs are rounded to f16 before cooperative execution;
- the compiler island depends on DXC generic SPIR-V extension attributes;
- end-to-end measurements include an intentionally expensive host readback;
- no profiler proves exact physical unit occupancy.

Production selector integration is **not yet justified**. M40b should not begin
as a rank change. A justified bounded follow-up is:

1. add a device-resident output benchmark and realistic persistent-B/new-A
   workload corpus;
2. implement one explicit padding or cooperative-main/conventional-tail plan;
3. measure tail and residency effects across representative inference shapes;
4. require stable end-to-end benefit over A2x4, not only FP16-storage;
5. only then introduce a capability-, precision-, alignment-, and residency-
   gated experimental selector candidate with telemetry and rollback.

A production M40b should reopen only when persistent-weight end-to-end median
beats the best conventional FP32 path materially across the target corpus and
tail handling remains correct and bounded. Until then, the experimental asset
and optional runtime capability facts are the appropriate permanent result.
