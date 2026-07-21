# DVT2 M5b builtin-topology full image

Status: **ACCEPTED AS SUCCESS (2026-07-21).** Identity 49 is production-owned
for Auto and forced `SubgroupOwned32`; identity 41 is the production
`SerialCanonical` fallback. Identity 47 and its historical evidence are
unchanged and are not selected by the completed production route.

Identity 49 replaces subgroup ownership derived from `SV_GroupIndex` with the SPIR-V `SubgroupId` and `SubgroupLocalInvocationId` built-ins. It preserves identity 47 unchanged: identity 47 is historical fast/numerically-valid evidence, but its ownership mapping is implementation-defined and unproved. Identity 41 remains the canonical fallback.

The same-session layer-0 alternating run used 20 samples per route after four alternating warm-up pairs. The full Prefetch sequence was SerialCanonical, BuiltinTopology, SerialCanonical, BuiltinTopology: 166.5402801, 165.7057764, 167.0417911, and 165.5166871 seconds. Both paired full-image differences favor identity 49 (0.8345037 s and 1.5251040 s), and every PNG matched `7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.

Identity 49 is recommended for a separate promotion-validation milestone, not promoted here. Static admission remains local size 256, subgroup size 32, native subgroup built-ins, and no workgroup variables, barriers, or atomics.
