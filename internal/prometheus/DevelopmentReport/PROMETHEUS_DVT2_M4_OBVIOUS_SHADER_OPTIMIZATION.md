# Prometheus DVT-2 M4: obvious shader optimization

Date: 2026-07-19

## Result

This pass reaches **meaningful progression**, not the requested 2x W1/W3 target.
The preferred RTX 3070 route no longer assigns one invocation to one QKV,
projection, W1, or W3 output and loops over 3,840 channels. Those contractions
now reuse the production `sgemm_reg2x2_tile16x16_fp32` structure: one 8x8
workgroup produces a 16x16 output tile, reuses FP32 activation and expanded-FP16
weight tiles in workgroup memory, and keeps 2x2 FP32 register accumulators.
W1/W3 shares the activation tile across both independent contractions.

The final fixed Prefetch smoke is 165.439 s versus the M3 196.302 s baseline,
a 15.7% wall reduction, with the accepted PNG identity unchanged. W1/W3 falls
29.7%, QKV 31.6%, and projection/residual 31.7%. W1/W3 remains 29.560 s, above
the 21.012 s target. Attention and fused W2/residual are now the isolated
algorithmic problems; attempting the same simple split/tile treatment for W2
was numerically accepted but slower and was completely reverted.

## Complete production inventory

Classification keys: **CE** compiler-expression issue; **PR** missing reusable
SDSL-V primitive; **RU** failure to reuse an existing mechanism; **GC**
generated-code defect; **KA** genuine kernel-algorithm problem; **OK** no
material obvious issue.

| Shader | Semantic operation | Runtime loops / known runtime facts / variants | Manual work | Existing mechanism and boring disposition | Class | Expected or measured payoff |
|---|---|---|---|---|---|---|
| `context_refiner_attention_streaming` | 32-token attention | key/channel/softmax/PV loops; fixed 32 tokens, 30 heads, width 128 | dot and reductions | Existing attention implementation is already streaming; leave redesign to attention pass | KA | Small in full image |
| `context_refiner_qk_norm_rope` | Q/K RMSNorm + text RoPE | channel loop; fixed width/head/RoPE partitions | RMS reduction/index transforms | Fixed constants already dominate generated path; no material branch ladder | CE/PR | Negligible M3 share |
| `main_transformer_ffn_gate` | SiLU(W1) * W3 | none beyond linear dispatch | elementwise arithmetic | Already semantic fixed gate stage | OK | None |
| `main_transformer_ffn_w1_w3` | two `[M,3840] x [3840,10240]` contractions | one 240-tile K loop; 3,840/10,240 and tile count now literals | no one-output scalar dot | Reused reg2x2/shared SGEMM structure; retained independent W1/W3 witnesses and existing gate stage | RU/CE fixed | 42.024 -> 29.560 s aggregate W1/W3 |
| `main_transformer_joint_attention_streaming` | 1,056-token joint attention | score, stable-softmax, PV loops; fixed 1,056/30/128 | dot and reductions | No attention redesign in M4 | KA | 32.317 -> 31.815 s is noise-level cleanup |
| `main_transformer_joint_qk_norm_rope` | joint Q/K RMSNorm + image/text RoPE | channel loop; fixed 1,024+32 domain and `[32,48,48]` split | RMS reduction/index transforms | Preserve; per-element domain cleanup requires an attention/RoPE-specific pass | CE/PR | Negligible M3 share |
| `nr0_adaln` | timestep projection and AdaLN split | projection/channel loops; fixed 3,840 split offsets | small contraction | Not a top stage; current fixed projection is retained | CE | Negligible |
| `nr0_attention_norm_modulate` | RMSNorm + modulation | channel/reduction loops; fixed width | RMS reduction | Needs reusable in-shader row reduction, not another local rewrite | PR | Negligible |
| `nr0_attention_projection` | `[M,3840] x [3840,3840]` | one 240-tile K loop; strides/extents now literals | no one-output scalar dot | Reused reg2x2/shared SGEMM structure | RU/CE fixed | projection/residual 13.556 -> 9.258 s |
| `nr0_attention_residual` | post-projection norm/gated residual | channel/reduction loops | RMS reduction | Keep fused elementwise boundary; reduction primitive remains compiler/library work | PR | Small |
| `nr0_attention_streaming` | 1,024-token attention + forensic sample | score, softmax, PV loops; selected keys are constants | dot/reductions; audit serialization | Removed four-iteration sample loop and integer ladder; added bounded shader-member audit sample/probability writers | CE/PR fixed; KA remains | Audit work disappears from hot variant selection; attention remains algorithmic |
| `nr0_bf16_ingress` | boundary cast | linear dispatch | none | Required ABI conversion | OK | None |
| `nr0_ffn_gate` | SiLU(W1) * W3 | linear dispatch | none | Already semantic fixed gate stage | OK | None |
| `nr0_ffn_norm_modulate` | RMSNorm + modulation | channel/reduction loops; fixed width | RMS reduction | Needs reusable in-shader row reduction | PR | Negligible |
| `nr0_ffn_w1_w3` | two `[M,3840] x [3840,10240]` contractions | one 240-tile K loop; strides/extents now literals | no one-output scalar dot | Same reg2x2/shared mechanism as main transformer | RU/CE fixed | representative W1/W3 accepted; complete NR0 about 100 ms warm boundary |
| `nr0_ffn_w2_residual` | `[M,10240] x [10240,3840]`, RMSNorm, gate, residual | 10,240-channel contraction plus 3,840 RMS loop | scalar contraction/reduction | A two-dispatch reg2x2 W2 experiment regressed representative main GPU 437.781 -> 452.129 ms and was rejected | KA | Final route 18.951 s; next focused kernel problem |
| `nr0_fused_qkv` | `[M,3840] x [3840,11520]` | one 240-tile K loop; strides/extents now literals | no one-output scalar dot | Reused reg2x2/shared SGEMM structure; main transformer and refiners share this physical kernel | RU/CE fixed | 41.628 -> 28.459 s |
| `nr0_k_norm_rope` | K RMSNorm + RoPE | width loop; fixed head width/partition | RMS reduction | Retain until reusable row reduction/RoPE primitive exists | PR | Negligible |
| `nr0_persistent_audit_summary` | typed summary/projection serialization | bounded source and tree-reduction loops | reduction/record stores | Already uses `AuditValueClass` payload enum and exhaustive `match`; no loose variant ladder found | OK | Audit-only |
| `nr0_q_norm_rope` | Q RMSNorm + RoPE | width loop; fixed head width/partition | RMS reduction | Retain until reusable row reduction/RoPE primitive exists | PR | Negligible |

MainTransformer QKV and projection are not separate SDSL-V files: the runtime
intentionally reuses shader IDs 26 and 31. Context-refiner projection does the
same. All normal, static-audit, context, and main dispatch geometries were
updated from one-output 8x8 geometry to 16x16 output-tile geometry.

## Cooperative SGEMM audit

`sgemm_cooperative_f16_f32_m16n16k16` has this exact contract:

- A storage: packed `F16x2` in `u32`, row-major `[M,K]`;
- B storage: packed `F16x2` in `u32`, row-major `[K,N]`;
- accumulator and C: FP32;
- M/N/K: multiples of 16, no tails;
- one 32-lane subgroup per 16x16 tile;
- Vulkan requirements: `VK_KHR_cooperative_matrix`,
  `SPV_KHR_cooperative_matrix`, `Float16`, `VulkanMemoryModel`, subgroup 32.

The RTX 3070 probe supports F16 x F16 -> F32 for 16x16x16, so all four fixed
shapes are dimensionally compatible. The route is nevertheless unsuitable for
the accepted production contract: A is a live FP32 activation, while the
cooperative primitive requires packed F16 A. M49/O14-O16/O20 established that
rounding activation boundaries changes accepted multi-layer identities. The
kernel cannot consume FP32 A with FP16 B on this device, and using it would
therefore change model semantics rather than optimize them. No runtime
activation pack or package transpose was added.

The immutable package layout was already compatible with the selected
conventional route: packed FP16, low lane first, row-major `[in,out]`, naturally
aligned as `u32`. Package identity and lock layout are unchanged. A package
layout projection is not required.

## Selected contraction mechanism

The semantic request remains a fixed contraction; the production realization
is frozen by shader IDs and generated SPIR-V headers. The implementation is the
repository's `sgemm_reg2x2_tile16x16_fp32` structure adapted only at the operand
load boundary for packed-FP16 immutable weights:

- 8x8 invocations produce a 16x16 output tile;
- every invocation retains a 2x2 FP32 register tile;
- each A/B tile element is loaded once per workgroup tile;
- packed weight lanes expand once on tile load;
- the fixed 16-lane inner product is unrolled;
- the only SPIR-V loop left in each contraction is the 240-tile K loop;
- QKV/projection have one conditional SPIR-V branch; W1/W3 branches are the
  pre-existing deterministic W3 alias-range stores.

The source still accepts the existing 16-byte push-constant ABI, but generated
address arithmetic and loop bounds use literal 3,840, 10,240, 11,520, and 240
facts. Token count remains dispatch-selected because the shared binary serves
the closed 32, 1,024, and 1,056-token variants.

The scalar implementation is no longer selected for QKV, projection, W1, or
W3 on RTX 3070. This selected route uses baseline Vulkan compute features, so
it needs no capability downgrade. The shader ID and generated module remain
part of execution-plan identity; there is no silent scalar fallback.

## Audit and variant cleanup

The sample keys `{0,1,512,1023}` are now literal call sites. The old runtime
`sample` loop and `if sample == ...` ladder are absent from generated HLSL and
SPIR-V. `WriteAuditSample` and `WriteAuditProbability` describe the stable wire
record without changing its offsets. The helper form is intentionally bounded;
no general serializer framework was introduced.

No other material integer/Boolean variant ladder was found in the production
portfolio. Persistent summary classification already uses a closed payload
enum and exhaustive `match`. The attempted scalar-to-enum conversion for sample
indices was correctly rejected because sample index is not a semantic variant;
constant keys plus a typed record writer are the smaller representation.

## Generated-code gate

The durable inspection record is
`artifacts/Dvt2M4/generated_code_and_timing.json`. SDSL-V sources are the source
artifacts; deterministic generated SPIR-V is preserved in the corresponding
`reactor_vulkan_zimage_*_spirv.h` headers. Generated HLSL/SPIR-V hashes, sizes,
loop counts, branch counts, barriers, and remaining-loop explanations are in
the JSON record. Reproduction is:

```powershell
go run ./cmd/oct sdslv emit-hlsl <shader.sdslv> -o out/sdslv/<name>.hlsl
go run ./cmd/oct sdslv compile-spv <shader.sdslv> -o out/sdslv/<name>.spv
& "$env:VULKAN_SDK\Bin\spirv-val.exe" --target-env vulkan1.0 out/sdslv/<name>.spv
& "$env:VULKAN_SDK\Bin\spirv-dis.exe" out/sdslv/<name>.spv
```

The initial one-output shared-tile candidate was rejected despite an unchanged
PNG: W1/W3 regressed to 44.326 s and wall rose to 198.417 s. The accepted
register-tiled modules materially change generated code and performance; this
is not a source-only prettification.

## Numerical validation

No threshold, clamping, or model operation changed. The final real lane reports:

| Witness | Relative L2 | L-infinity | Result |
|---|---:|---:|---|
| QKV | 1.33528e-7 | 1.37329e-4 | accepted |
| attention projection | 2.30291e-7 | 5.46875e-2 | accepted |
| W1 | 7.41145e-7 | 1.75476e-4 | accepted |
| W3 | 8.77522e-7 | 5.64575e-4 | accepted |
| gated hidden | 2.09487e-6 | 1.46484e-2 | accepted |
| representative final block | 1.30438e-6 | 1.52588e-4 | accepted |
| representative MainTransformer joint output | 8.38066e-7 | 6.40869e-4 | accepted |
| 30-layer authority | 1.02005e-5 | 1.17188e-2 | accepted under 5e-5 relative-L2 threshold |

Final Prefetch PNG SHA-256:
`7ba9047ae27ea7060c8358ca25bf704e4169b006e628560b1901518bbb483613`.

## Performance

The table sums the 270 MainTransformer layer traces in the fixed nine-evaluation
canonical run.

| Stage | M3 baseline (s) | M4 final (s) | Change |
|---|---:|---:|---:|
| FFN W1/W3 | 42.024 | 29.560 | -29.7% |
| QKV | 41.628 | 28.459 | -31.6% |
| Attention | 32.317 | 31.815 | -1.6% |
| W2/residual | 19.610 | 18.951 | -3.4% |
| Projection/residual | 13.556 | 9.258 | -31.7% |
| Full-image wall | 196.302 | 165.439 | -15.7% |

The W1/W3 target of <=21.012 s is not met. Reaching it now requires a stronger
mixed FP32-activation/FP16-weight contraction algorithm or a newly justified
precision contract. Manual cooperative fragments, new tile choreography, and
speculative tile searches are deliberately outside this boring-cleanup pass.

## Rejected W2 reuse

The fused W2/RMSNorm/gated-residual shader still contains the portfolio's last
giant scalar contraction. A bounded experiment split it into the same reg2x2
W2 contraction plus a typed `ReductionLane`/`match` postprocess. Numerical
contracts passed, but representative MainTransformer GPU median regressed from
436.342 ms to 452.129 ms. The experiment was fully reverted. This proves that
mechanically applying the current tile is unsuitable; W2 is a genuine
kernel-algorithm problem rather than unattempted obvious reuse.

Attention remains similarly algorithmic after audit cleanup. Its score,
stable-softmax, and probability-times-value loops dominate; M4 did not redesign
them.

## Validation

Passed on the final accepted tree:

- full SDSL-V generation/check and Vulkan 1.0 `spirv-val` for changed modules;
- generated HLSL and SPIR-V loop/branch/barrier inspection;
- `go run ./tools/sdslv_workspace_check`;
- native Windows build (`internal/prometheus/native/build_windows.cmd`);
- full default `marionette_tests.exe` suite;
- `go test ./...` during the pass, followed after specialization by focused
  SDSL-V, Prometheus, bridge, and compiled-lock package tests;
- real M1B/M1C/M1D witness lane, including representative W1/W3/gate/block;
- representative MainTransformer and 30-layer authority lane;
- fixed Prefetch canonical smoke, nine evaluations, exact PNG identity;
- EVT-2 payload and lock check;
- JSON parsing for manifest, canonical metadata, and this artifact;
- `git diff --check`.

Default native execution skips explicitly opt-in local-payload lanes; the real
M1B/M1C/M1D and M2C lanes above were run separately with the validated local
payload roots.

## Reference/documentation gap

`Language/reference/language/16-vectors-and-matrices.md` describes tensor
statement lowering as deferred historical work, while current production SDSL-V
supports `tile`, `reg_tile`, `matrix_view`, and cooperative contraction
intrinsics. The reference remains authoritative for Oct syntax, but it does not
document these current SDSL-V compiler facilities or a reusable resource-bound
dot/reduction/audit-writer abstraction. This pass followed the existing
production shader contracts and records the inconsistency rather than silently
treating older examples as authority.
