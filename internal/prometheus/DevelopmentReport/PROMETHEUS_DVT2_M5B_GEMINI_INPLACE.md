# DVT2-M5B-GEMINI-INPLACE

Status: measured, experimental only, not promoted. `GeminiExact` remains
unchanged external authorship evidence. `SerialCanonical` remains the active
production default and fallback.

## Hypothesis and provenance

GeminiInPlace tests exactly one reviewer-derived hypothesis: GeminiExact's
three private `float[33]` arrays may mask the benefit of its two subgroup
reductions. It is a separate HLSL source at
`internal/prometheus/shaders/hlsl/experimental/m5b-gemini-inplace.hlsl` with
provenance **Gemini algorithm, reviewer-derived in-place private storage
transformation**. It retains bindings, push constants, entry point,
256-thread dispatch, eight-subgroup row ownership, QK/PV order,
`WaveActiveMax`, both `WaveActiveSum` reductions, validity test, output and
audit behavior.

The sole transformation replaces `scores[33]`, `exponentials[33]`, and
`probabilities[33]` with `values[33]`: scores are written first, `scoreZero`
preserves the audit value, the array is overwritten with exponentials and then
probabilities, and PV shuffles those probabilities. It does not add reciprocal
multiplication, fast math, loop restructuring, or any other optimization.

## Static comparison

DXC 1.9.0.5347 compiled GeminiInPlace with `-spirv -T cs_6_0 -E
MainTransformerJointAttentionSubgroupOwned_CS -fspv-target-env=vulkan1.3 -O3`.
`spirv-val --target-env vulkan1.4` passed. GeminiInPlace source SHA-256 is
`02bc4eeba55f99d18b631df0611c13d010897f7f2f3e21cf1a4622492bd67f9e`;
SPIR-V is 6,404 bytes, SHA-256
`d7b5e2deda2d1a901ed2f741d066ae71915d6a3b3247e60cc3dc7f250f743aa8`.

| Variant | SPIR-V bytes / instructions | FMax / FAdd / shuffle | Function arrays | Workgroup vars / barriers / atomics |
| --- | --- | --- | --- | --- |
| Original M5b | 7,076 / 434 | 1 / 0 / 3 | 2×`float[33]`, 1×`float[4]` | 0 / 0 / 0 |
| GeminiExact | 6,508 / 378 | 1 / 2 / 1 | 3×`float[33]` | 0 / 0 / 0 |
| GeminiInPlace | 6,404 / 372 | 1 / 2 / 1 | 1×`float[33]` | 0 / 0 / 0 |

SPIR-V exposes no register count or local-memory byte metric. The available
storage evidence is the Function-storage `OpVariable`/`OpTypeArray` count:
GeminiInPlace emits one private array where GeminiExact emits three.

## Bounded admission, authority, and timing

The existing RTX 3070 preflight passed: subgroup size 32, subgroup compute,
arithmetic, basic, and shuffle support, Vulkan 1.4/SPIR-V 1.6 contract, and
subgroup-owned admission. GeminiInPlace then ran the same retained-stream
representative layer-0 MainTransformer route as GeminiExact. It passed finite
output, bounded audit, static audit, and final-transformer relative L2
`8.39396e-7` under the unchanged `5e-5` authority limit. The resident-chain
authority was `9.66693e-6`. No device loss, timeout, non-finite output,
admission failure, or audit/bounds failure occurred.

Ten warm samples measure the whole representative layer-0 MainTransformer
stage, not the attention dispatch alone. CPU wall samples were
`477647500,480393800,478923500,462692400,463774000,466765200,462939600,477924400,478658100,476820100`
ns: best 462692400, median 477233800, mean 472653860, population variance
51246430588400 ns², standard deviation 7158661.2 ns. The Vulkan timestamp
compute-boundary aggregate was GPU median 476628528 ns and mean 472053000 ns;
raw GPU samples were not emitted by this existing harness. RTX 3070
`timestampPeriod` is 1 ns with 64 valid queue timestamp bits.

Against the disclosed existing medians, GeminiInPlace is about 7.75% slower
than SerialCanonical and 8.51% slower than GeminiExact. This is well outside
the observed single-run sample variation but remains stage-level evidence, not
an isolated attention-kernel benchmark. It does not justify a full Z-Image
run. No full image, image hash, promotion, SDSL-V translation, Make work, or
unrelated optimization was performed.
