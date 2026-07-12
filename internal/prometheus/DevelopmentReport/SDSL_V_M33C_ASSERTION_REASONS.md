# SDSL-V M33c: Mandatory assertion reasons

Every SDSL-V assertion ends with a nonempty string-literal reason. The reason
describes the invariant being proved and is compiler-owned test metadata; it
is never a shader runtime string.

Supported signatures are `Assert.True(condition, reason)`,
`Assert.False(condition, reason)`, `Assert.Equal(expected, actual, reason)`,
`Assert.NotEqual(unexpected, actual, reason)`, and
`Assert.Near(expected, actual, tolerance, reason)`.

The validator rejects missing, nonliteral, empty, and ASCII-whitespace-only
reasons. Literal contents and their exact spans are retained by the canonical
assertion plan and serialized in the test manifest. The fixed GPU result ABI
continues to report assertion ordinals and payload words only; the native host
joins a failure to manifest metadata and emits `reason` in its JSON result.

Case identity remains source/function/kind/row based, so changing assertion
prose does not change a stable case ID. Authors should name the semantic
invariant (for example, “guarded reads must return the fallback above the
input length”), rather than restating a comparison such as “value equals 1”.

Invalid SDSL-V fixtures use the established `.octfail` header grammar,
`expect error: "..."`. The SDSL-V fixture adapter reuses the shared parser and
then performs its existing SDSL-V phase, diagnostic-code, and exact-span
checks. M33c adds ten invalid fixtures covering missing reasons on every
assertion form, empty and whitespace-only literals, nonliterals, wrong types,
and extra arguments.

The native host was rebuilt with Visual Studio 2026 Developer Command Prompt
on Windows. The rebuilt executable preserves result ABI version 1 and emits a
manifest-joined JSON `reason` for a deliberately failing assertion.

Migration and proof summary: 517 reasonless assertion call sites were migrated
across 10 committed `.sdslvtest` files (521 supported assertion calls now exist
in the migrated corpus and valid fixtures). The pragmatic skim found none of
the prohibited generic reason forms. Manifest schema is version 4; stable case
IDs are unchanged because they exclude assertion prose.

Expected-failure replay was exercised by the native host contract using stable
case `sdslv-364333778db4567d859e051d`: assertion `0` reports reason
`FirstFailureWins must preserve its declared invariant`, expected bits `1`,
actual bits `2`, and the original assertion source coordinates. Replaying the
same stable case produces the same manifest-joined reason. The proof ran on
2026-07-11 using an NVIDIA GeForce RTX 3070 (driver 596.36, Vulkan device API
1.4.329; loader 1.4.350).
