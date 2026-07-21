# DVT-2 reviewer handoff

Date: 2026-07-21

## Closed production state

M5B is accepted as SUCCESS. Production attention authority is identity 49 for
Auto/forced `SubgroupOwned32` and identity 41 for `SerialCanonical` fallback.
Identity 47 remains unchanged historical evidence. Review should reject any
diff that changes those facts as part of M6A.

The forced-route closeout is hardened: selected attention authority is checked
before owner/pipeline allocation, rejection is detail `-6927`, and the bridge
returns route/shader/fallback evidence without dispatch or absent-pointer
access. A child-process regression exits with ordinary status 7 and signal 0.
Registry and manifest validation pin identities 41 and 49.

## M6A disposition

The isolated identities 50/51 prove real layer-0 cooperative W1/W3 execution.
Median complete-boundary improvement is 15.66% with 19/20 paired wins,
bitwise-deterministic replay, finite output, and a 694,950,916-byte measured
Prefetch footprint. The completed layer relative L2 is 4.65441e-6.

Do not approve M6B or promotion without an owner decision: raw W1
(`6.78747e-5`), raw W3 (`1.02317e-4`), and gated (`7.57058e-5`) exceed the
existing `5e-5` authority. The valid choices are canonical-only production, a
separately named mixed-precision fast profile, or route rejection.

Primary review artifacts:

- `artifacts/Dvt2M6a/cooperative_matrix_contract_rtx3070.json`
- `artifacts/Dvt2M6a/cooperative_matrix_compiler_proof.json`
- `artifacts/Dvt2M6a/bf16_fp16_w1_w3_audit_rtx3070.json`
- `artifacts/Dvt2M6a/m6a_layer0_evaluation_rtx3070.json`
- `PROMETHEUS_DVT2_M6A_COOPERATIVE_MATRIX_FEASIBILITY.md`
