# EVT-2 M1D Z-Image 10240-wide gated FFN and final residual

## Outcome

**SUCCESS — M1D completes the resident FP32 `noise_refiner.0` block and EVT-2 is ready for M1E.** The real Vulkan witness consumes the resident M1C attention residual, adjusted MLP scale, and tanh MLP gate without host tensor ingress; it produces the canonical O19 final FP32 `ModelEmbedding` in the accepted memory envelope.

The authoritative local payload setup remains [EVT2_LOCAL_PAYLOADS.md](../../../docs/EVT2_LOCAL_PAYLOADS.md). The canonical final diagnostic identity is `4aff8bf19cfbfc9aebf2e8aa78ef91fb7bb5c117f98504080ed1bc3b206e0c43`; historical capture data and the diagnostic Go executor are not semantic authority.

## Fixed resident program

`prometheus_reactor_runtime_model_block_execute_m1d` rejects a missing or stale M1C replay identity and executes the closed, model-specific shader portfolio:

1. `Nr0FfnNormModulate` performs uncentered FP32 RMSNorm1 and applies the resident adjusted MLP scale.
2. `Nr0FfnW1W3` contracts the `[1024,3840]` FP32 input against the cached FP16 W1 and W3 `[3840,10240]` tensors, expanding pairs to FP32 at arithmetic use.
3. `Nr0FfnGate` computes exact FP32 `x / (1 + exp(-x)) * w3` and stores gated hidden activation in place of W1 after its witness boundary.
4. `Nr0FfnW2Residual` contracts the `[1024,10240]` FP32 hidden activation against cached FP16 W2, applies FP32 RMSNorm2, broadcasts the resident tanh MLP gate, and adds the preserved attention residual in FP32.

The SDSL-V sources use grouped semantic-space declarations and ordinary function signatures for legal transitions. The complete portfolio, bindings, dispatches, and source identities are frozen in [m1d_shader_portfolio.json](artifacts/Evt2M1d/m1d_shader_portfolio.json); the static sequence and synchronization boundaries are in [m1d_execution_plan.json](artifacts/Evt2M1d/m1d_execution_plan.json).

## Precision and range result

All activations and reductions are FP32. The only FP16 use is immutable package-weight unpack at arithmetic use. There is no activation FP16 storage, TF32/BF16 runtime route, clamp, saturation, CPU fallback, or host intermediate tensor bounce. The observed W2 range is `[-62296.4, 142582]`, confirming that the post-W2 path cannot safely be FP16 and is not cast there.

## Real O19 witness

On an NVIDIA GeForce RTX 3070 with Vulkan validation enabled, `PrometheusM1BRealPayloadReachesTheFirstCanonicalModelWitness` passed the entire M1B → M1C → M1D resident chain. Every audited M1D full boundary is finite and under the strict `5e-5` relative-L2 threshold.

| Boundary | Relative L2 | Linf | Range |
| --- | ---: | ---: | --- |
| FFN norm | 8.29172e-7 | 0.000118256 | [-8.40769, 67.646] |
| FFN modulation | 9.33449e-7 | 8.01086e-5 | [-14.4328, 46.8637] |
| W1 | 7.41145e-7 | 0.000175476 | [-75.2414, 103.681] |
| W3 | 8.77522e-7 | 0.000564575 | [-356.355, 333.029] |
| SiLU(W1) × W3 | 2.09487e-6 | 0.0146484 | [-4194.26, 3668.63] |
| W2 | 1.94789e-6 | 0.40625 | [-62296.4, 142582] |
| Final output | 1.30438e-6 | 0.000152588 | [-6.84376, 40.048] |

The direct W1/W3 and gated-hidden boundaries establish the exact SiLU route; FFN norm2 is intentionally fused into the fixed FP32 W2/final-residual workgroup, so W2 and final output are its bracketing resident full-boundary witnesses. The complete audit, including first mismatch coordinates and canonical final projection, is recorded in [m1d_ffn_audit.json](artifacts/Evt2M1d/m1d_ffn_audit.json) and [m1d_final_output_audit.json](artifacts/Evt2M1d/m1d_final_output_audit.json).

## Memory, replay, and lifecycle

The implementation preserves the accepted `654,891,776`-byte resident plan: `361,820,672` persistent bytes, `245,884,928` reusable bytes, and `47,186,176` audit bytes. W3 is partitioned across released M1C buffers only until the gate consumes it; no extra 10240-wide persistent W3 resource is allocated. The final FP32 output is retained in the released attention buffer for M1E. See [m1d_memory.json](artifacts/Evt2M1d/m1d_memory.json).

Execution-plan identity now includes M1B, M1C, and M1D shader IDs, bindings, and push-constant sizes. M1D replay identity hashes its required M1C replay identity, requested output identity, and audit stage; no transient process state participates. Shared model-block lifecycle coverage remains in force. The current bounded fault granularity and the deliberate absence of invented per-pipeline non-finite instrumentation are documented in [m1d_faults.json](artifacts/Evt2M1d/m1d_faults.json).

## Timing and next seam

The ten M1C-prefix warm samples and zero-churn contract are retained in [m1d_timing.json](artifacts/Evt2M1d/m1d_timing.json). The existing evidence API does not expose timestamp-query splits for the four M1D pipelines, so this report records no fabricated per-stage GPU timing. The resident M1D execution itself completed in the real witness with zero post-warm allocations/uploads/pipeline creation/descriptor growth.

M1E inherits a resident `[1,1024,3840]` FP32 row-major `ModelEmbedding`, the fixed four-shader M1D portfolio, the exact final authority identity, and the accepted memory plan. Its narrow scope is frozen in [evt2_m1e_handoff.json](artifacts/Evt2M1d/evt2_m1e_handoff.json).
