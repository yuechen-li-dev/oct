# DVT-2 Mx5 Vulkan 1.4 shader migration

## Outcome

**MEANINGFUL PROGRESSION — IN PROGRESS — READY WITH REQUIRED ADAPTATIONS.**

The complete registered SDSL-V production portfolio (40 assets) now compiles
with the compiler-owned `ProductionVulkan14` contract: DXC's highest supported
`-fspv-target-env=vulkan1.3`, SPIR-V 1.6, and `spirv-val --target-env
vulkan1.4`.  Every regenerated module passed that validator.  Native runtime
initialization now requests and requires Vulkan 1.4 for both the loader and
selected physical device.

DXC from Vulkan SDK 1.4.350.0 has no `vulkan1.4` spelling; its help lists
`vulkan1.3` as the maximum.  SPIRV-Tools v2026.2 explicitly supports SPIR-V
1.6 under Vulkan 1.4 semantics.  The production contract is therefore not
weakened to a DXC spelling.

The accepted M4 arithmetic was not edited.  Existing authority remains final
relative L2 `1.02005e-5` (threshold `5e-5`) and PNG SHA-256
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.
The production-path subgroup proof is green.  It is a bounded SDSL-V test
source using the established typed HLSL escape hatch, compiled by the SDSL-V
test compiler with the production DXC lowering, validated as Vulkan 1.4, and
executed through the native Vulkan assertion host on the RTX 3070.  It launches
two 32-lane workgroups and asserts deterministic maximum `31` and broadcast
first `0`.  Its SPIR-V 1.6 SHA-256 is
`4d54625f9b5349e7d5acc252fe8c231ca05aca692d6fc41838a27a02266f024e` and
contains `OpGroupNonUniformFMax` and `OpGroupNonUniformBroadcastFirst`.

The accepted nine-evaluation Prefetch replay completed with the exact accepted
PNG SHA-256, unchanged Prefetch allocation ceiling (`1,005,407,748` bytes),
and unchanged serial M4 route.  Two post-migration runs measured 174.946 s and
172.396 s rather than the accepted 165.439 s.  GPU accounting on the first run
localizes 2.495 s of additional GPU time, including 2.297 s in unchanged
attention shaders.  This is a material unexplained target-legalization
regression, so Mx5 cannot close successfully or promote M5b.

## Controlled attention artifact experiment

A diagnostic-only control compiled the identical generated HLSL for the three
attention modules with DXC `vulkan1.0`, while leaving the runtime Vulkan-1.4
admission, every other regenerated module, payload, and Prefetch profile
unchanged.  The control modules differ only in SPIR-V version and entry-point
interface listing: SPIR-V 1.0 names built-ins; SPIR-V 1.6 additionally names
the descriptor resources and push constants.  Opcode counts and source HLSL
are identical.

The control replay was exact and measured **167.947 s**, recovering most of
the SPIR-V-1.6 attention regression.  It proves the regression belongs to
driver handling of that artifact form, not the attention algorithm.  The
control is gated by the explicit build-only macro
`PROMETHEUS_DVT2_MX5_VULKAN10_CONTROL`, is never the default binary, and is
not a compatibility route or a production solution.  The normal
ProductionVulkan14 binary was rebuilt after the experiment.

## Rejected SPIR-V 1.6 encoding alternative

The next control retained DXC's `vulkan1.3` spelling and SPIR-V 1.6, then
removed descriptor and push-constant IDs from the three attention
`OpEntryPoint` interface lists, retaining only the built-in compute inputs.
This would have matched the compact Vulkan-1.0 control encoding without
changing the production target contract or shader instructions.

It is not legal SPIR-V 1.6 for Vulkan 1.4. `spirv-val --target-env vulkan1.4`
rejects it: `Interface variable ... StorageBuffer ... is used by entry point
... but is not listed as an interface`. The installed DXC exposes no option
which removes required interfaces: `-fspv-preserve-interface` expands the
set, `-fspv-reflect` adds reflection instructions, and
`-fspv-reduce-load-size` changes the binary but retains the same interface.
All tested compute profiles (`cs_6_0`, `cs_6_1`, `cs_6_6`, and `cs_6_9`)
produce that same required interface form under `vulkan1.3`.

Therefore this compacting transformation is rejected rather than promoted.
The next bounded optimization seam is a driver/toolchain artifact encoding
that remains valid SPIR-V 1.6, or a newer pinned DXC that changes this legal
encoding. Neither permits restoring Vulkan 1.0 as a production target.

## SPIRV-Tools canonical optimization control

`spirv-opt -O` was also tested on the three DXC-produced SPIR-V 1.6 attention
modules. Each optimized module validated with `spirv-val --target-env
vulkan1.4`, preserved the required entry-point interfaces, and reduced binary
size (NR0 from 10,280 to 9,208 bytes). The RTX 3070 preflight passed with
validation enabled, and the full Prefetch replay produced the exact canonical
PNG and unchanged model-owned ceiling.

Its 170.710 s wall time is better than the best unoptimized Mx5 sample, but
the stage measurement is decisive: total attention time was 34.136 s versus
34.112 s for the unoptimized Mx5 sample. The apparent wall-time gain therefore
does not recover the localized attention regression and is within the
unexplained run-level variance. This valid encoding is not promoted; the
ordinary DXC-produced ProductionVulkan14 artifact remains the production
binary.

## Supplied newer DXC control

The supplied `microsoft.direct3d.dxc.1.9.2602.24.nupkg` was unpacked only to
the ignored Mx5 control workspace. Its package SHA-256 is
`4e4cef12283f7875a3602b9f5dc04f153c77cfa216559f58881305f59f8f7e2f` and
its DXC identity is `1.9.2602.24 (d355aa836)`. It still exposes `vulkan1.3`
as its highest target spelling. All three production attention shaders were
compiled through `oct sdslv compile-spv` with that executable and validated by
SPIRV-Tools as Vulkan 1.4.

The emitted SPIR-V binaries are byte-identical to the current pinned DXC
outputs (NR0 SHA-256
`be8edeb10e2d7774e8647525332f153cd7649228d393fc0664bd5609fcd451b`, with
the corresponding ContextRefiner and MainTransformer hashes identical too).
It cannot change the driver artifact, so no replay or toolchain migration is
warranted. The production toolchain remains pinned as previously recorded.

## M5b handoff

After the target-legalization performance regression is isolated, resume
subgroup attention using `ProductionVulkan14`, an RTX 3070 subgroup
size of 32, and the already established operations
`OpGroupNonUniformFMax` and `OpGroupNonUniformBroadcastFirst`.  Preserve the
M4 serial route as the explicit fallback.  Do not reuse M5a's rejected
256-thread topology.  First add the production SDSL-V subgroup proof and its
capability admission test, then evaluate one 32-thread subgroup-row topology
and the separately admitted multi-row-256 candidate.
