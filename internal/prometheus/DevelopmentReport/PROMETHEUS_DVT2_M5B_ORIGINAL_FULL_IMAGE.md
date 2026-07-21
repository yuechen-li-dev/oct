# DVT2-M5B-ORIGINAL-FULL-IMAGE

Status: original M5b qualified for a separate promotion-review milestone only.
It remains experimental; `SerialCanonical` was rebuilt as the active default
and fallback. GeminiExact and GeminiInPlace, including their source and
evidence, were not modified or executed.

## Bounded stability

The prior ten-sample complete representative-MainTransformer measurements used
the same RTX 3070, Vulkan 1.4/SPIR-V 1.6 route, model, shape, validation, and
warm policy. The boundary includes the complete layer-0 MainTransformer stage,
not attention alone. Serial raw CPU samples were
`440805700,445419300,443262900,443375100,443629000,444113800,442571300,442315300,440592800,441075200`
ns (median 442917100). Original M5b samples were
`433995800,422320700,427048400,407979200,431250200,429232400,427213600,431902300,431936000,426795800`
ns (median 428223000).

The distributions do not overlap: M5b's slowest sample, 433995800 ns, is below
SerialCanonical's fastest, 440592800 ns. M5b's bounded median is 3.32% lower;
the observed Serial and M5b population standard deviations were 1.479 ms and
7.100 ms respectively. Additional bounded samples were not required before
the full-image gate.

## Static and admission invariants

Original M5b is registry id 44 and remains selected only by
`PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT`. Its SPIR-V SHA-256 is
`bf53ad5fe22d83a581a41cc096358762f1766bf5c922ac3f3966880fd4616ebc`
(7,076 bytes). Static disassembly reconfirmed one
`OpGroupNonUniformFMax`, three `OpGroupNonUniformShuffle`, and zero workgroup
variables, barriers, and atomics. The RTX 3070 preflight again admitted the
route with subgroup size 32 and compute/arithmetic/basic/shuffle support.

The final real retained-stream authority rerun passed finite output,
representative final relative L2 `8.38066e-7`, and full 30-layer relative L2
`1.02005e-5`, both under the unchanged `5e-5` limit. Audit validation passed.

## Alternating Prefetch full-image runs

The established Python Prefetch smoke ran in alternating order:
Serial 1, M5b 1, Serial 2, M5b 2. Every run used the same
Tongyi-MAI/Z-Image-Turbo `f332072…564a6` model/checkpoint, lock,
`A lighthouse in fog at dawn` prompt, seed 42, 512×512 image, nine evaluations
of 30 MainTransformer layers, Vulkan validation, and Prefetch policy. All four
generated the exact canonical PNG SHA-256
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.

| Order | Variant | Full wall seconds | PNG authority |
| ---: | --- | ---: | --- |
| 1 | SerialCanonical | 172.2471449 | pass |
| 2 | Original M5b | 159.5279281 | pass |
| 3 | SerialCanonical | 166.2933085 | pass |
| 4 | Original M5b | 161.4066003 | pass |

Serial raw mean was 169.2702267 s (population standard deviation 2.9769182 s);
M5b raw mean was 160.4672642 s (standard deviation 0.9393361 s). The paired
speedups were 7.3843% and 2.9386%, respectively; mean M5b speedup was 5.2005%.
The M5b best, 159.5279281 s, is 7.4643% below the documented post-migration
SerialCanonical best of 172.396 s. The historical pre-migration M4 time was
not used as a promotion comparison.

The local raw PNGs, smoke metadata, and stdout/stderr logs are retained at
`out/prometheus/dvt2_m5b_original_full_image/`. No device loss, timeout,
non-finite output, numerical failure, PNG mismatch, or manifest/audit failure
occurred.

## Decision

Original M5b has enough bounded and full-image evidence to enter a separate
promotion-review milestone. It is not promoted by this task. SerialCanonical
remains active, and all Gemini variants remain preserved negative experiments.
