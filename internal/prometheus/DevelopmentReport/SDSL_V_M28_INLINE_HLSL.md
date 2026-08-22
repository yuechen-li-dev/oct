# SDSL-V M28 — Bounded Inline HLSL Blocks

## Milestone verification

The SDSL-V development reports contain M25 (ordered `derive`), M26 (derive SGEMM), and M27 (guarded-read value semantics), with no M28 report. **M28** is therefore the next unused canonical SDSL-V milestone number.

## Design

M28 introduces target-generic `ForeignShaderStmt` and `ForeignShaderExpr` AST/VD-MIR nodes. Their first registered target is `HLSL`; adding a later target is a target registration/lowering task rather than an HLSL-shaped compiler rewrite. The lexer enters a raw foreign-block scanner after `HLSL` and balances braces while recognizing quoted strings plus line and block comments. Raw source is never tokenized as SDSL-V.

Syntax follows existing SDSL-V scalar names:

```sdslv
HLSL { GroupMemoryBarrierWithGroupSync(); }
let lane: u32 = HLSL<u32> { return asuint(1.0); };
let value: f32 = HLSL<f32>(a, b) { return mad(a, b, 1.0); };
```

Captures are explicit names. Scope and duplicate checks are performed at the boundary; supported values are `bool`, `i32`, `u32`, `f32`/`float`, and existing fixed scalar vectors. Boards, records, resources, arrays, enums, flow state, and opaque compiler types are rejected with a diagnostic directing users to extract a scalar/vector first. The result annotation is authoritative SDSL-V typing and requires a raw `return` path. DXC remains authoritative for HLSL semantics.

## Emission and restrictions

Statement blocks are emitted in place. Expression blocks lower to a compiler-owned local result plus a bounded local HLSL block, so the raw HLSL `return` assigns once to that result rather than relying on unsupported HLSL lambdas. Generated HLSL includes `BEGIN/END INLINE HLSL <file>:<line>` markers for DXC context. Conservative lexical rejection prevents interface-shaping source (`#`, `register`, `cbuffer`, texture/sampler/buffer declarations, `numthreads`, structs, namespaces). This is a correctness boundary and portability signal, not a security sandbox.

Inline HLSL is target-constrained. The current pipeline supports HLSL only; a non-HLSL lowering must reject it until it explicitly registers an alternate foreign target. It cannot introduce entry points, bindings, resources, includes, or pipeline state, and generated resource names are not a public contract.

## Proof and roadmap

`Examples/SDSL-V/M28/InlineHlslBitCast.sdslv` and the manifest-owned `internal/prometheus/shaders/sdslv/production/sgemm/inline_hlsl_bitcast_proof.sdslv` demonstrate `asuint`, a precise target bit-cast with no current SDSL-V equivalent, and a barrier statement. Registry asset ID 15 records SDSL-V as its primary language, two inline blocks, and foreign target `HLSL`; existing asset and implementation IDs are unchanged. It has no compute implementation and is non-dispatchable, non-benchmark, and selector-ineligible.

## Validation and limitations

Focused lexer/parser/type/emitter coverage verifies nested braces, strings/comments, unterminated blocks, capture scope/duplicates/types, forbidden interface patterns, source markers, and unsupported target rejection. `go test ./internal/sdslv/...`, `go test ./internal/... ./cmd/oct`, DXC/SPIR-V generation, native manifest checking, and the Windows Marionette suite pass (340 run, 335 pass, 5 expected skips). Vulkan preflight identified an NVIDIA GeForce RTX 3070.

## Acceptance status

**SDSL-V M28 ACCEPTED — compiler and SPIR-V toolchain scope.**

M28 proves the language/compiler/backend path: generic `ForeignShaderStmt` / `ForeignShaderExpr` representation; bounded raw HLSL parsing; typed explicit captures; restrictions; target requirement tracking; HLSL emission; DXC/SPIR-V generation; manifest ownership; registry provenance; SDSL-V regressions; and the native Prometheus/registry validation path.

Hardware execution is explicitly out of scope. The current Prometheus runtime dispatches only SGEMM implementations through its fixed A/B/C storage-buffer and push-constant contract. M28 intentionally adds neither a one-off test path nor a fake SGEMM implementation, because either would distort the runtime architecture merely to exercise a language feature.

### Future runtime follow-up

A future, separately scoped milestone may provide a generic **registered compute-shader dispatch/readback harness**. It must be reusable rather than M28-specific: the same contract should serve SDSL-V feature proofs, FFT, reductions, subgroup operations, and other non-SGEMM compute shaders. It must own explicit binding schemas, pipeline creation, dispatch geometry, readback, and device evidence without changing selector policy. That work is not started by M28.

### M29 handoff update

M29 now owns the test-specific alternative to that broad generic-runtime idea:
`.sdslvtest` discovery, stable case identity, and a fixed assertion result ABI.
Its fixed native host now executes the `InlineHlslAsUint` test artifact through
DXC/SPIR-V/Vulkan result readback on the RTX-capable Windows environment. This
closes M28's runtime-proof handoff for the bounded `asuint` foreign block only;
it does not expand M28 into a generic compute runtime.
