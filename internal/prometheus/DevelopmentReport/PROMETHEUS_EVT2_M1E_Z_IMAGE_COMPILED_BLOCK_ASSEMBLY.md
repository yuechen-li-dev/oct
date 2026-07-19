# EVT-2 M1E Z-Image compiled `noise_refiner.0` block assembly

## Outcome

**SUCCESS — assembly, callable facade, and complete-block warm evidence are
frozen; remaining-family reuse is intentionally not certified without the
pinned full-model source inventory.** The completed M1B, M1C, and M1D resident
Vulkan implementation is represented as one deterministic compiled-block
metadata package. This does not alter model arithmetic or its accepted M1D
result.

- Compiled-model status: **FIRST REAL COMPILED BLOCK ASSEMBLY COMPLETE**
- Final output: FP32 `ModelEmbedding`, `[1,1024,3840]`, relative L2
  `1.30438e-6` (limit `5e-5`)
- Peak resident device memory: `654,891,776` bytes
- Existing fixed execution-plan identity: `3212572515069549270`
- Existing final-output-audit replay identity: `6445635600144955590`

The local payload and O19 authority remain
[EVT2_LOCAL_PAYLOADS.md](../../../docs/EVT2_LOCAL_PAYLOADS.md). The validation
gate is `go run ./tools/evt2_payload_check`; it verified the pinned cache,
oracle, all 34 O19 stage payloads, manifest, projections, and final diagnostic.

## Frozen assembly

The complete production portfolio is 13 fixed pipelines: BF16 ingress, AdaLN,
attention norm/modulation, QKV, Q RoPE, K RoPE, streaming attention,
projection, attention residual, FFN norm/modulation, W1/W3, SiLU gate, and
W2/norm/final residual. It is not a graph and has no runtime topology or
arbitrary-shader lookup. The native resident owner retains its established
create/upload/evidence/destroy boundary and now exposes
`prometheus_reactor_runtime_noise_refiner0_execute` for the fixed M1B/M1C/M1D
resident sequence. The facade has no host intermediate output; bounded audits
remain explicit replays.

The immutable numerical policy is unchanged: FP16 cache weights expand only at
FP32 arithmetic use, ingress BF16 widens exactly to FP32, all activations and
reductions are FP32, and there is no TF32, activation FP16, clamp, saturation,
CPU fallback, host intermediate bounce, or adaptive precision path. In
particular, the assembly retains the four no-cast seams requested for
`attention_norm2`, attention residual, W2-to-FFN-norm2, and final residual.

The resource plan continues to use the accepted `361,820,672` persistent,
`245,884,928` reusable, and `47,186,176` audit bytes. Q/K/V are live through
attention; the C/QKV allocation is reused only after attention; W1 becomes
gated hidden only after audit; and the released attention allocation is the
resident FP32 final output. Failed work is quarantined and stale output is
rejected.

## Canonical evidence, timing, and lifecycle

All 34 canonical O19 witnesses are accounted for in one audit map. Persistent
boundaries retain the measured M1B/M1C/M1D results; streaming score and
probability state remains selected-row evidence with positive finite
denominators and normalized probabilities, never a false claim of fully
materialized payload equality. The frozen threshold table is stage-specific;
there is no model-wide fallback tolerance.

Ten prior complete-prefix warm samples remain the timing authority. They show
zero warm buffer/device-memory allocation, upload, pipeline creation,
descriptor growth, local payload reads, host tensor computation, or
intermediate host bounce. The native evidence API still does not expose M1D
per-pipeline timestamp splits, nor does it have per-pipeline fault injection;
the package records those limits without treating them as correctness failures.
Shared lifecycle coverage proves partial initialization cleanup, pipeline and
descriptor failure, dispatch/audit failure, uncertain-completion quarantine and
reap, stale-output rejection, recreate, and repeated destroy protection.

The route is validated on an RTX 3070 with Vulkan validation. Semantics are
portable and SPIR-V is validated, but the resident plan is only proven on that
device: other NVIDIA GPUs and AMD have not been certified.

## Deterministic package and next seam

The reproducible generator is
[`tools/evt2_m1e_assembly/main.go`](../../../tools/evt2_m1e_assembly/main.go).
It generates every artifact twice and rejects a hash mismatch. The package is
payload-free and references immutable cache/canonical identities rather than
placing large weights or witnesses in Git. See
[`artifacts/Evt2M1e`](artifacts/Evt2M1e) for the manifest, ABI, portfolio,
execution, memory/alias, audit, thresholds, timing, faults, replay, route,
family, streaming, and M2 handoff contracts.

The first full-model streaming baseline is deliberately synchronous: validate
the block weights, upload current block, execute and establish certain
completion, then reuse the weight arena. Under uncertain completion, do not
evict or reuse; quarantine and reap first. Asynchronous prefetch is excluded.

The exact next milestone is **EVT-2 M2A — compile `noise_refiner.1` by
assembly reuse**, after acquiring the pinned full-model source/tensor inventory
and establishing actual topology and oracle differences. M1F closure is in
[the M1F report](PROMETHEUS_EVT2_M1F_Z_IMAGE_COMPILED_BLOCK_FACADE.md).
