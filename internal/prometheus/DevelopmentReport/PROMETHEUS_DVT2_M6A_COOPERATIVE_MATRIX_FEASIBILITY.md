# DVT2-M6A cooperative-matrix feasibility

Date: 2026-07-21

Status: **MEANINGFUL PROGRESSION — OWNER DECISION REQUIRED; DO NOT START M6B.**

M6A proves that the real layer-0 W1/W3 contraction can execute through an
isolated F16 × F16 → F32 cooperative-matrix route and materially reduce the
measured layer boundary. It does not promote that route. Raw W1, raw W3, and
the gated intermediate exceed the existing `5e-5` numerical authority, even
though the completed layer output is below it. The required next action is an
explicit owner choice among canonical-accuracy production only, a separately
named fast mixed-precision profile, or rejection.

## Preserved production state and forced-route hardening

M5B is accepted as SUCCESS. Production attention remains exactly:

- identity 49 for Auto and forced `SubgroupOwned32`;
- identity 41 for `SerialCanonical` fallback;
- identity 47 unchanged as historical evidence, not the selected route.

No attention shader was edited or benchmarked. Admission now preflights the
selected attention asset before owner or pipeline allocation. Rejection records
detail `-6927`, requested/selected route, shader identity, and fallback reason;
the bridge returns that structured error before dispatch. Native tests cover an
experimental identity-41 authority mutation and an absent asset pointer.
The subprocess regression records the precise reason and exits normally with
status 7 and signal 0, ruling out the prior access-violation path.
Registry validation and the workspace manifest checker pin identities 41 and
49 to production SDSL-V authority and their exact source paths.

## RTX 3070 cooperative contract

The live Vulkan probe recorded 11 `VK_KHR_cooperative_matrix` configurations,
subgroup size 32, storage-buffer offset alignment 16 bytes, and the device
limits in `cooperative_matrix_contract_rtx3070.json`. The required subgroup
tuple exists: A=`float16`, B=`float16`, accumulator=`float32`,
result=`float32`, scope=subgroup, M/N/K=`16/16/16`. One BF16/BF16/F32/F32
16×16×16 tuple is exposed, but the current SDSL-V route does not claim BF16
lowering. Vulkan property enumeration does not enumerate layouts; cooperative
load/store operands permit row/column layout and M6A validates row-major only.

## Compiler proof

The compile-only SDSL-V fixture passed generated-HLSL inspection, DXC SPIR-V
1.6 lowering for Vulkan 1.3, `spirv-val`, and disassembly. Evidence records
`OpTypeCooperativeMatrixKHR` for F16 operands and F32 accumulation/result,
`OpCooperativeMatrixMulAddKHR`, subgroup scope, 16×16×16 dimensions, three
bindings, row-major loads/stores, required capabilities, zero atomics, and two
intentional workgroup barriers used solely for tile staging. SPIR-V was not
hand edited.

## Real checkpoint BF16 → FP16 audit

All 60 W1/W3 tensors across all 30 layers were read directly from the pinned
12.3 GB safetensors checkpoint (`240761...574a6`):

| Measure | Result |
|---|---:|
| Elements | 2,359,296,000 |
| Non-finite | 0 |
| FP16 overflow | 0 |
| Underflow to zero | 384 |
| Exact conversions | 2,359,234,575 (99.9974%) |
| Maximum absolute error | 2.9802322387695312e-8 |
| Maximum relative error | 1.0 (underflow cases) |

All 60 tensor names, shapes, source ranges, hashes, ranges, and per-tensor
counts are in `bf16_fp16_w1_w3_audit_rtx3070.json`. Conversion is therefore
extremely close but not literally exact for this family.

The established Prefetch path streams checkpoint-derived FP16 cache payloads;
BF16 conversion occurs when that cache is generated. M6A adds no second model
weight cache, and runtime weight-conversion cost is consequently zero at this
boundary. This repository fact is explicit in the evidence rather than being
misreported as an in-dispatch BF16 conversion.

## Isolated candidate

The candidate is available only under
`PROMETHEUS_DVT2_M6A_COOPERATIVE_W1W3_EXPERIMENT` and runtime route
`OCT_DVT2_M6A_W1W3_ROUTE=cooperative`:

- identity 50 packs the authoritative FP32 activation once to FP16;
- identity 51 owns the cooperative W1 and W3 payload;
- separate pack, W1, and W3 pipeline objects are used;
- W1 and W3 reuse the same packed activation;
- W1/W3 outputs, W3 segmentation, SiLU/gating, W2, residuals, and persistent
  output remain FP32;
- canonical identity 42 remains present and is the default even in the
  experimental build when the route variable is absent;
- the ordinary production build allocates none of the M6A buffers.

The candidate adds 8,110,080 bytes for packed activation and 43,253,760 bytes
for contiguous FP32 W3 before copying into the established three-view gate
layout. Measured model-owned peak is 694,950,916 bytes with Prefetch preserved.
The 824,712,196-byte numerical run includes a temporary, opt-in three-stage
host-visible capture and is not the performance-route footprint.

## Bounded real layer-0 evaluation

The test alternated canonical and candidate on one retained layer-0 session,
after four warm-up pairs, for 20 measured pairs. The retained evidence is
layer-0-only and no image workload ran. During final validation, one invocation
omitted the existing bounded flag and entered the legacy M2D full-transformer
branch; that output was discarded, was not used to evaluate M6A, and the lane
was rerun successfully with `OCT_EVT2_M5B_BOUNDED=1`.

| Boundary | Canonical median | Candidate median |
|---|---:|---:|
| Complete layer-0 execute | 440.529 ms | 371.525 ms |
| W1/W3 route (all pack/repack included) | 109.631 ms | 40.236 ms |
| FP32 gate epilogue | 0.297 ms | 0.302 ms |

Candidate median total improvement is 15.66%; it wins 19/20 pairs. Total means
are 441.423 ms canonical and 378.652 ms candidate. Population standard
deviations are 3.741 ms and 31.318 ms (CV 0.00847 and 0.08271). The candidate
variance is dominated by one 133.852 ms W3 segmentation-copy outlier; the
cooperative execution median remains 39.971 ms. Activation packing median is
0.059 ms and W3 segmentation/repack median is 0.204 ms. Raw samples, all
summary statistics, paired differences, bytes, VRAM, and identities are in
`m6a_layer0_evaluation_rtx3070.json`.

## Numerical characterization

The existing threshold is tested unchanged with a `1e-6` near-zero guard for
maximum relative error:

| Boundary | Relative L2 | Linf | Max relative | Finite | 5e-5 |
|---|---:|---:|---:|---:|---:|
| Raw W1 | 6.78747e-5 | 0.016983 | 150.2 | yes | fail |
| Raw W3 | 1.02317e-4 | 0.0238037 | 186.624 | yes | fail |
| Post-SiLU/gate | 7.57058e-5 | 4.80078 | 86.7001 | yes | fail |
| Completed layer-0 | 4.65441e-6 | 0.00247192 | 13.6171 | yes | pass |

Two completed candidate replays are bitwise identical. No non-finite values
were observed. The large maximum-relative values are guarded near-zero
coordinates; they do not replace relative L2 or justify a looser contract.

## Decision

Do not promote and do not proceed to M6B yet. Compiler/static audits, finiteness,
determinism, performance, and Prefetch VRAM compatibility pass. The raw
numerical authority does not. Preserve the evidence and obtain an explicit
owner decision among:

1. canonical-accuracy production only;
2. a separately named fast mixed-precision profile;
3. rejection of the cooperative route.

## Evidence and validation

Machine-readable evidence is under `DevelopmentReport/artifacts/Dvt2M6a/`.
The affected Go tools, SDSL-V generation, DXC/`spirv-val`, SPIR-V disassembly,
native production and M6A builds, registry tests, workspace parity, JSON
parsing, payload hashes, macro isolation, and real bounded tests were run.
No `.octmake` content is part of the change.
