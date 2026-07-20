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

## M5b handoff

After the target-legalization performance regression is isolated, resume
subgroup attention using `ProductionVulkan14`, an RTX 3070 subgroup
size of 32, and the already established operations
`OpGroupNonUniformFMax` and `OpGroupNonUniformBroadcastFirst`.  Preserve the
M4 serial route as the explicit fallback.  Do not reuse M5a's rejected
256-thread topology.  First add the production SDSL-V subgroup proof and its
capability admission test, then evaluate one 32-thread subgroup-row topology
and the separately admitted multi-row-256 candidate.
