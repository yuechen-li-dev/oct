# EVT-2 M1C Z-Image attention residual

## Current conclusion

**SUCCESS — M1C is complete and EVT-2 is ready for M1D.** The real resident M1B-to-M1C path passes the accepted O19 full-boundary witnesses and the bounded selected-coordinate transient audit under Vulkan validation.

The canonical authority is `o19-fp32-reference` under the local relative root recorded in [m1c_canonical_stage_authority.json](artifacts/Evt2M1c/m1c_canonical_stage_authority.json). Historical `capture_04` is forensic-only; payload setup and validation are governed by [EVT2_LOCAL_PAYLOADS.md](../../../docs/EVT2_LOCAL_PAYLOADS.md).

## Implemented resident path

`prometheus_reactor_runtime_model_block_execute_m1c` requires the exact resident M1B replay identity. It accepts no host Q/K/V tensors. It dispatches:

1. `Nr0AttentionStreaming`: non-causal, 30-head attention; one workgroup owns one `(query token, head)` row. Its 1024 FP32 scores and 1024 FP32 probabilities are workgroup-local, so there is no persistent `[1024,1024]` logits/probability allocation.
2. `Nr0AttentionProjection`: FP32 attention-output projection.
3. `Nr0AttentionResidual`: FP32 RMSNorm2, FP16-weight expansion at use, gate broadcast, and addition of the original M1B residual.

The detailed schedule and precision contract are frozen in [m1c_execution_plan.json](artifacts/Evt2M1c/m1c_execution_plan.json). M1C replay identity includes M1B replay identity, requested output identity, and audit boundary, preventing a boundary replay collision. The execution-plan identity includes the 64-FP32/256-byte third audit binding.

## Measured O19 boundary audit

The real payload command is:

```powershell
$env:OCT_EVT2_M1B_REAL='1'; $env:OCT_EVT2_M1C_REAL='1'; out\prometheus\native\marionette_tests.exe PrometheusM1BRealPayloadReachesTheFirstCanonicalModelWitness
```

All values below are FP32 `[1,1024,3840]` row-major resident boundaries compared with the canonical O19 payload. Full metrics and payload identities are in [m1c_projection_norm_residual_audit.json](artifacts/Evt2M1c/m1c_projection_norm_residual_audit.json).

| Boundary | Finite | Relative L2 | Linf | First mismatch |
| --- | --- | ---: | ---: | ---: |
| aggregation | yes | 2.41332e-7 | 0.000274658 | 1 |
| projection | yes | 2.30291e-7 | 0.0546875 | 0 |
| gated residual | yes | 1.45129e-6 | 0.000154495 | 35 |

`attention_norm2` and the gated product are deliberately not materialized as separate buffers; the accepted final residual is their downstream full-boundary witness. Projection and residual full-boundary comparison therefore proves the FP32 norm/gate/residual path without creating FP16 projection or residual storage.

## Warm memory and lifecycle evidence

Ten real chained M1B-to-M1C executions made zero new buffer allocations, weight uploads, pipeline creations, or descriptor-set growth. Median chained time was 364,794,650 ns and p95 was 372,042,100 ns. Memory measurements are 361,820,672 persistent bytes, 245,884,928 reusable bytes, 47,186,176 audit bytes, and 654,891,776 peak planned device bytes. See [m1c_memory.json](artifacts/Evt2M1c/m1c_memory.json) and [m1c_timing.json](artifacts/Evt2M1c/m1c_timing.json).

The current shared model-block fault suite covers creation, dispatch, audit copy, uncertain completion quarantine/reap, stale output, partial initialization, and destroy/recreate. Its M1C-specific coverage is recorded candidly in [m1c_faults.json](artifacts/Evt2M1c/m1c_faults.json).

## Streaming transient evidence

The selected O19 rows are hash-addressed in [m1c_transient_attention_audit.json](artifacts/Evt2M1c/m1c_transient_attention_audit.json), including first/last token and head. The 64-float device-local audit record captures raw/scaled logits, max, shifted values, exponentials, denominator, probabilities, row-normalization, probability-times-V, and per-head output for keys `0, 1, 512, 1023`. The real witness compares selected logits/probabilities with O19, rejects non-finite/non-positive denominators, and confirms probability rows sum to one. Full aggregation, projection, and gated residual are independently compared as resident full boundaries.

Full transient payload hashes remain intentionally unavailable because score/probability rows are workgroup-local. Correctness is instead established through canonical selected witnesses, softmax invariants, probability-times-V evidence, downstream full-boundary agreement, and the final resident residual. Do not substitute persistent full score/probability tensors. The exact M1D ABI is recorded in [evt2_m1d_handoff.json](artifacts/Evt2M1c/evt2_m1d_handoff.json).
