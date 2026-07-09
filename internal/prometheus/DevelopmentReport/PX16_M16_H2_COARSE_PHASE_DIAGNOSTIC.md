## Px16 M16 H2 Coarse-Phase Diagnostic

This milestone localizes the remaining host-side SGEMM gap after H1.

H1 added `command_record_ms` for command-buffer recording plus descriptor-set update. Real hardware data ruled H1 out: command recording stayed flat and tiny, while the remaining `unaccounted_host_ms` scaled with M*N*K.

H2 adds three coarse timing buckets around the unbracketed regions inside `prom_reactor_runtime_sgemm_impl_with_variant`:

- `pre_dispatch_ms`
- `post_sync_ms`
- `post_readback_ms`

This is diagnostics only. It does not optimize P14/P15, change kernels, retune selectors, change production dispatch authority, change SDSL-V, or touch FFT/P16.

## Runtime Fields

The runtime and `PrometheusSgemmPolicyDiagnostics` now expose:

```c
uint64_t px16_m8_last_pre_dispatch_wall_ns;
uint64_t px16_m8_last_post_sync_wall_ns;
uint64_t px16_m8_last_post_readback_wall_ns;
```

They follow the same pattern as `px16_m8_last_command_record_wall_ns`:

1. reset with the runtime timing decomposition;
2. bracketed at the real SGEMM call site;
3. copied through the policy diagnostics getter.

## Window Definitions

`pre_dispatch_ms` covers:

- occupancy facts;
- judgment-engine selection;
- P15 forecast/feedforward and reservation reconciliation before dispatch;
- path/compute-mode decision;
- buffering and layout/precision facts;
- current upload preparation/copies before command-record begin.

`post_sync_ms` covers:

- after `vkWaitForFences(...)` completes;
- `vkGetQueryPoolResults(...)`;
- GPU timestamp interpretation;
- `prom_dominatus_measurement_filter_update(...)`;
- P14 update logic;
- P15 maturation/correction/reservation/shadow update logic.

`post_readback_ms` covers:

- after output readback completes;
- slot/controller/bookkeeping;
- return from the SGEMM function.

The leading suspect is `post_sync_ms`, but the diagnostic does not assert that it must dominate.

## Artifacts

Run:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Deep_CoarsePhaseLocalization
```

Artifacts:

- `out/test-artifacts/prometheus_sgemm_px16_deep_coarse_phase.md`
- `out/test-artifacts/prometheus_sgemm_px16_deep_coarse_phase.json`

The broader deep diagnostic also includes the same H2 fields:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16DeepDiagnostics
```

Artifacts:

- `out/test-artifacts/prometheus_sgemm_px16_deep_diagnostics.md`
- `out/test-artifacts/prometheus_sgemm_px16_deep_diagnostics.json`

## Accounting

`unaccounted_host_ms` now subtracts:

```text
pre_dispatch_ms
command_record_ms
dispatch_submit_ms
sync_wait_ms
post_sync_ms
readback_ms
post_readback_ms
kernel_ms   (only when GPU timestamps are valid)
```

`upload_ms` remains reported in EVT/deep artifacts as an informational sub-bucket, but it is not subtracted separately because the current runtime performs upload before command-record begin, inside `pre_dispatch_ms`.

The correctness lane's CPU oracle and validation timing remain reported separately because they are outside the production measured SGEMM call.

## CPU Work Audit

Found CPU-side work in or adjacent to the measured SGEMM path:

| Path | Complexity | Timed bucket | Gating | Status |
| --- | ---: | --- | --- | --- |
| `prom_fp16_evaluate_tolerance(...)` | O(M*N*K) | `pre_dispatch_ms` | Not debug-gated | Still suspect. It evaluates FP16 tolerance by recomputing reference and quantized products over every output and K element. |
| `prom_apply_debug_row_major_oracle(...)` | O(M*N*K) plus O(M*N) | `readback_ms` | `PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK` and packed4 mode | Ruled out for default EVT unless the test flag is enabled. |
| `prom_pack_a_packed4_rowmajor(...)` | O(M*K) | `upload_ms` | packed4 mode | Accounted by upload timing; not M*N*K. |
| `prom_pack_b_packed4_colmajor(...)` | O(K*N) | `upload_ms` | packed4 mode | Accounted by upload timing; not M*N*K. |
| `prom_pack_fp16_pairs(...)` | O(M*K + K*N) | `upload_ms` | FP16 mode | Accounted by upload timing; not M*N*K. |
| output readback `memcpy(...)` | O(M*N) | `readback_ms` | readback path | Accounted by readback timing. |
| `prom_dominatus_measurement_filter_update(...)` and filter policy | O(1), fixed windows <= 16 and <= 9 | `post_sync_ms` | production P14 | Still measured by H2, but no matrix-size loop found. |
| P15 reservation/maturation/shadow update functions | O(1), fixed rings/caps of 16 | `post_sync_ms` and `pre_dispatch_ms` | production P15 | Still measured by H2, but no matrix-size loop found. |
| EVT CPU oracle/validation helpers | O(M*N*K) or O(M*N) | outside default benchmark timing | correctness lane only | Ruled out for default production timing; reported separately when used. |

No loop was removed in this milestone. The main actionable result is that H2 can now distinguish the confirmed O(M*N*K) pre-dispatch suspect from post-sync P14/P15 work and from readback/bookkeeping.

## Interpretation

- If `pre_dispatch_ms` scales with M*N*K, inspect `prom_fp16_evaluate_tolerance(...)` and any selector facts feeding it next.
- If `post_sync_ms` scales with M*N*K, sub-bracket P14 filter update versus P15 maturation/correction/reservation/shadow updates.
- If all three H2 buckets are small and remaining unaccounted time persists, widen the search to code between benchmark iterations and external Vulkan/tooling layers.
