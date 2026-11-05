# SDSL-V frontend and scientific math modernization

Date: 2026-11-04

The compiler now shares binary-expression parsing mechanics with ordinary Oct,
accepts declaration-first shader templates, and replaces 42 production inline
HLSL blocks with typed math, conversion, and bit operations. The 26 sources in
the math/template pass emit exactly the same SPIR-V bytes as before. Three
additional BF16 sources use ordinary typed code and pass GPU boundary checks;
the two roundtrip helpers now preserve infinities that the old foreign emitter
incorrectly converted to NaNs.

## Comparison and implementation boundary

The audit used Oct baseline `b30d64bb` and read-only Copeland checkout
`e7859357257af1d942000ee98c65396fd63d563a`. The current Aurelian guides
`docs/Aurelian/visual-typescript-language-port.md` and
`docs/Aurelian/visual-typescript-compute-values.md` describe sharing Copeland's
parser, certifying a bounded GPU profile, and expressing typed vector/resource
programs before backend emission. Aurelian's implementation remains unchanged.

These are concrete architectural lessons. This pass does not claim complete
feature parity between Visual TypeScript and SDSL-V.

| Concern | Result |
| --- | --- |
| Parser duplication | `internal/syntax.ParseBinary` owns the precedence-climbing algorithm used by both frontends. Each supplies its own operands, operator precedence, delimiter rules, AST construction, and diagnostics. |
| Template spelling | `template shader Tile<C: TileConfig>` puts parameters after the name, matching Oct's declaration order. Six production templates and the M5 template example use it. |
| Existing shader contracts | The old template spelling remains accepted. Pinned benchmark and graphics fixtures retain their paths and identities. Concept-config specialization still takes exactly one explicit parameter. |
| Scientific math | `Exp`, `Sqrt`, `Rsqrt`, `Sin`, `Cos`, `Tanh`, and `Pow` validate scalar/vector types and lower to explicit VD-MIR intrinsics. The HLSL backend owns the operation spellings. |
| Numeric conversion | Existing `Convert<f32>` replaces raw `float(...)` escapes. Existing production helper-call boundaries remain stable. |
| Integer words | Scalar `i32`/`u32` bit operators, defined shift counts, and hexadecimal masks replace the three BF16 escapes. The existing bounded constant evaluator implements the same 32-bit rules. |

This shares actual parsing code without translating shader source into host
Oct or admitting host-only runtime features. Shader declarations, static config
constraints, memory permissions, semantic spaces, stages, and GPU legality stay
with their existing compiler owners. The math additions are runtime operations,
not another compile-time evaluator.

## Production result

Counting actual `HLSL` expression/statement introducers gives **48 blocks before,
6 after**. A preliminary word search counted 50 because two comments also
mentioned HLSL. The 42 removed blocks occur across 23 sources:

- FFT radix-2 twiddles and conversion;
- softmax exponential/sum, normalization, and packed-short stages;
- Z-Image activation, normalization, attention, positional, and audit helpers.
- packed BF16 ingress and Gemma BF16 roundtrip/RoPE precision boundaries.

Six production SGEMM sources also adopt declaration-first templates. The
comparison therefore covers 29 sources: 28 manifest assets and the existing
unregistered `nr0_attention_streaming_parallel.sdslv` source. This pass does not
register that extra source or change selection policy.

Every before/after source passed DXC compilation and `spirv-val`. SHA-256
comparison found **26 byte-identical SPIR-V modules** in the math/template pass.
Regenerating those 25 manifest headers with repository-relative commands left
their tracked bytes unchanged. The three BF16 modules differ and their headers
were regenerated, along with the tracked Gemma HLSL/SPIR-V side products.
Bindings, entry names, workgroups, and shader IDs retain their previous values.
Manifest inline-HLSL provenance now reflects the source changes.

## Integer and BF16 qualification

The new bit operators require matching scalar integer types, preserve the left
type for shifts, and require an unsigned shift count. Lowering explicitly masks
the count with 31 in VD-MIR and preserves the left integer type before HLSL
emission. This prevents backend literal promotion from turning a signed right
shift into an unsigned one. Hexadecimal literals retain exact source spans and
normalize to decimal AST values so existing attributes and layout consumers
share the same decoded values.

The [integer GPU fact](../../../Language/SDSL-V/Modernization/BitwiseOperations.sdslvtest)
checks runtime high bits, masks, complement, signed/unsigned shifts, counts
31/32/33, and precedence. The
[constant config specimen](../../../Language/SDSL-V/conformance/compute/BitwiseConstants.sdslvvalid)
checks the same 32-bit rules through config specialization, static assertions,
and workgroup extents. Nested angle closers have a parser boundary test that
also verifies source spans and that lexer input is not mutated.

The [BF16 GPU contracts](../../../Language/SDSL-V/Modernization/Bf16BitOperations.sdslvtest)
contain 20 roundtrip rows and eight packed-lane rows using uploaded runtime
words. Expected results cover even/odd ties, neighboring values, signed zero,
subnormals, overflow to infinity, both infinities, and quiet/signalling NaN
payloads. Their helper definitions match the new production helper bodies.

Paired qualification extracted the actual helper body from each original and
rewritten production source, then attached the canonical GPU theory. These
are helper-boundary executions through the native Vulkan host, rather than a
claim to have run the full Gemma or Z-Image model:

| Production helper | Original | Rewritten |
| --- | --- | --- |
| BF16 ingress widening | 8 passed, 0 failed | 8 passed, 0 failed |
| Gemma BF16 roundtrip | 18 passed, 2 failed | 20 passed, 0 failed |
| Gemma RoPE BF16 roundtrip | 18 passed, 2 failed | 20 passed, 0 failed |

The four original failures are positive/negative infinity becoming quiet NaN
(`0x7f800000 → 0x7fc00000`, `0xff800000 → 0xffc00000`). Generated HLSL reveals
the cause: only the first `return` in an inline expression is changed to a
result assignment; it then falls through to the next raw return. The new
ordinary SDSL-V branches preserve the intended infinity result. Original
failures are retained, not reclassified as passing equivalence. The foreign
return-rewriting defect is tracked as [FB-109](../../../FEEDBACK.md#fb-109--inline-hlsl-expressions-rewrite-only-the-first-return).

## Executable coverage

The [canonical template specimen](../../../Language/SDSL-V/conformance/compute/OctStyleTemplate.sdslvvalid)
checks config substitution in workgroup memory, workgroup dimensions, and a
concrete emitted entry. It compiles to validated SPIR-V.

Fourteen registered negative specimens cover missing template parameters, parameters
without `template`, math arity, integer arguments, mismatched vector widths, and
semantic-space erasure, bit operand/count types, and hexadecimal overflow.
The conformance manifest pins their diagnostic codes
and source locations.

The [math GPU contracts](../../../Language/SDSL-V/Modernization/MathIntrinsics.sdslvtest)
use uploaded runtime inputs and independently stated expected values. Two facts
exercise all seven operations on scalars and float4 lanes, plus float2/float3,
composition, unary/binary precedence, and left associativity. Assertions check
individual scalar lanes because the current `Assert.Near` contract is scalar.
Both facts pass through the native Vulkan test host.
Together with the integer fact and 28 BF16 theory rows, the new permanent GPU
corpus contains 31 passing cases.

Relevant validation commands:

The complete fast Go suite passes. The ordinary Oct arithmetic corpus passes
16 facts and the logical corpus passes six facts in each of interpreted and
strict compiled execution, with zero interpreter fallback in compiled runs.
The SDSL-V conformance gate passes the new static/diagnostic cases alongside
the existing pinned artifacts.

```powershell
go test -count=1 -parallel 8 ./...
go run ./cmd/oct sdslv test Language/SDSL-V/Modernization
go run ./cmd/oct sdslv compile-spv Language/SDSL-V/conformance/compute/OctStyleTemplate.sdslvvalid -o out/sdslv/modernization/template.spv --validate --require-spirv-val
go run ./cmd/oct test Language/Expressions/Arithmetic/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/Arithmetic/valid --execution compiled
go run ./cmd/oct test Language/Expressions/Logical/valid --execution interpreted
go run ./cmd/oct test Language/Expressions/Logical/valid --execution compiled
python tools/check_reference.py
git diff --check
```

## Remaining language work

| Remaining blocks | Owning capability |
| --- | --- |
| Four softmax/subgroup attention/topology bodies | Typed subgroup collectives, lane information, broadcasts, participation rules, and existing builtin integration; [FB-106](../../../FEEDBACK.md#fb-106--subgroup-collectives-still-require-inline-shader-bodies) |
| Two foreign-boundary proof blocks | Deliberate conformance input, not missing language functionality; `Bitcast` and `WorkgroupBarrier` already exist |

Removing the four subgroup bodies needs their type, capability, and uniformity
contracts plus real kernel qualification.
The packed bit-operation gap [FB-105](../../../FEEDBACK.md#fb-105--packed-bf16-boundaries-lack-ordinary-typed-bit-operations)
is resolved by this pass.

The full workspace checker currently fails its obsolete native-registry source
scan. The same first failure reproduces against HEAD's manifest/registry and
conformance inputs with the unchanged checker. [FB-104](../../../FEEDBACK.md#fb-104--workspace-ownership-check-still-assumes-a-source-bearing-native-registry)
records that separate defect. Shader compilation, native GPU facts, and binary
equivalence do not imply that this broader checker is green.
