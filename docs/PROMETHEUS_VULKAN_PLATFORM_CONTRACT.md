# Prometheus Vulkan platform contract

Prometheus production requires Vulkan 1.4 and SPIR-V 1.6.  Its sole shipping
shader profile is `ProductionVulkan14`; Vulkan 1.0 is not a compatibility
fallback and no legacy profile is selected implicitly.

The installed Vulkan SDK is 1.4.350.0, with DXC 1.9.0.5347
(`1.10.5347-fe261573`) and SPIRV-Tools v2026.2.  DXC exposes `vulkan1.3` as
its highest Vulkan target spelling, so production compilation uses that
lowering and validates the resulting SPIR-V 1.6 with
`spirv-val --target-env vulkan1.4`.  This is a compiler spelling limitation,
not a reduction of Prometheus's Vulkan 1.4 runtime contract.

Runtime initialization rejects loaders or selected physical devices below
Vulkan 1.4.  Optional mechanisms remain independently admitted: subgroup
routes need compute-stage subgroup support, the required operation classes,
and their required subgroup size; cooperative matrices additionally require
their extension, feature, tuple, float16, and Vulkan-memory-model facts.
Absence rejects that optimized route rather than silently changing one
execution identity.

For the Mx5 proof, `OpGroupNonUniformFMax` requires subgroup arithmetic and
`OpGroupNonUniformBroadcastFirst` requires basic/broadcast support. The
RTX-specific route separately requires compute-stage support and subgroup size
32; none follows merely from the Vulkan 1.4 API floor.

Artifact and replay identities include `ProductionVulkan14`, DXC lowering
`vulkan1.3`, validator environment `vulkan1.4`, SPIR-V 1.6, and the exact
capability route.  Model and parameter identities do not change merely from
shader legalization.
