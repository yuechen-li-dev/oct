# SDSL-V M37b cross-runtime production timing comparison

Status: **COMPLETE** (Windows RTX 3070, 2026-07-15)

> Historical evidence notice: the original tables are intentionally preserved, but M38a invalidated their Kaiju finite-correctness claim and corrected the adapter. Use the post-fix tables and correction section below for current results.

This closeout uses the existing Prometheus native audit path and the typed Kaiju Octxiliary Vulkan path.  Both consume the registry's current production SPIR-V bytes for the named kernel, derive the same deterministic logical inputs, use the existing packing contract, compare readback against the CPU reference, and use Vulkan query-pool GPU timestamps.  Pipeline creation, upload, and readback are outside the measured kernel region in their existing paths.

Each row used one warmup and five measured iterations.  Kaiju ran with validation requested, available, and enabled; it reported zero warnings, zero errors, and no device loss.  Prometheus was run in its validation-enabled native lane.  `yes` means that both backend outputs passed their CPU-reference comparison.

## Representative workload: M=512, N=512, K=512

| Kernel | Prometheus median ns | Kaiju median ns | Prometheus min/max | Kaiju min/max | Outputs match |
|---|---:|---:|---|---|---|
| scalar | 1,100,544 | 1,125,408 | 1,099,040 / 1,114,976 | 1,118,240 / 1,128,672 | yes |
| tiled | 805,888 | 839,904 | 803,968 / 806,400 | 839,808 / 841,408 | yes |
| memory-conservative | 1,077,472 | 1,083,008 | 1,077,024 / 1,078,112 | 1,081,152 / 1,090,848 | yes |
| scalar-plus | 1,083,104 | 1,139,072 | 1,081,408 / 1,084,800 | 1,134,400 / 1,147,840 | yes |
| tile16 | 1,034,368 | 1,036,736 | 1,032,832 / 1,034,656 | 1,034,848 / 1,036,992 | yes |
| SRT | 1,105,280 | 1,111,808 | 1,097,312 / 1,109,760 | 1,108,736 / 1,114,592 | yes |
| B2x2 | 585,088 | 9,931,008 | 584,160 / 585,952 | 9,642,336 / 10,025,760 | yes |
| A2x4 | 1,141,856 | 8,766,304 | 1,139,680 / 1,144,032 | 8,231,456 / 9,067,776 | yes |
| Packed4 | 3,841,696 | 1,114,144 | 3,801,152 / 3,875,488 | 1,102,304 / 1,125,984 | yes |
| FP16 | 4,082,336 | 1,186,464 | 4,069,056 / 4,084,288 | 1,179,168 / 1,193,600 | yes |

## Odd/tail workload: M=127, N=131, K=129

| Kernel | Prometheus median ns | Kaiju median ns | Prometheus min/max | Kaiju min/max | Outputs match |
|---|---:|---:|---|---|---|
| scalar | 17,504 | 42,208 | 17,280 / 18,080 | 41,536 / 42,848 | yes |
| tiled | 19,424 | 31,168 | 19,040 / 19,936 | 30,400 / 31,296 | yes |
| memory-conservative | 15,776 | 28,992 | 15,104 / 16,192 | 28,032 / 31,232 | yes |
| scalar-plus | 19,168 | 44,128 | 18,720 / 19,872 | 42,592 / 45,248 | yes |
| tile16 | 29,984 | 31,040 | 29,600 / 30,272 | 30,720 / 31,264 | yes |
| SRT | 14,144 | 29,824 | 13,568 / 14,880 | 28,864 / 30,208 | yes |
| B2x2 | 20,576 | 541,952 | 20,224 / 20,736 | 509,600 / 568,960 | yes |
| A2x4 | 179,328 | 441,504 | 178,880 / 179,584 | 436,832 / 453,376 | yes |
| Packed4 | 41,984 | 23,808 | 41,344 / 42,464 | 23,104 / 24,224 | yes |
| FP16 | 66,208 | 45,760 | 66,080 / 81,792 | 44,288 / 48,032 | yes |

Prometheus has lower medians for every representative row except Packed4 and FP16.  On the odd/tail workload it is lower for scalar, tiled, memory-conservative, scalar-plus, tile16, SRT, B2x2, and A2x4; Kaiju is lower for Packed4 and FP16.  The complete per-backend ordering does not broadly agree: B2x2 is Prometheus's representative leader but Kaiju's slowest representative row, while Kaiju's Packed4 behavior differs sharply from Prometheus.

A2x4 remains suspicious: it is 8,766,304 ns on Kaiju for the representative workload and 179,328 ns / 441,504 ns for the odd/tail workload on Prometheus/Kaiju, respectively.  Packed4 and FP16 have no correctness failure, but their timing trends differ by runtime; these kernel-only, runtime-specific timestamp regions are not an application-speed claim.  Kaiju remains an independent runtime witness, not the production performance authority.  Production shader sources, artifact ownership, and selection policy were unchanged.

Convergence outcome: **SUCCESS**

Milestone state: **COMPLETE**

## M38a correction and post-fix rerun

The tables above are retained as the original M37b evidence. M38a found that the original Kaiju adapter generated intended negative values through unsigned subtraction, used Packed4's layout for FP16, and did not reject non-finite comparisons. Therefore the original `Outputs match` column is historical rather than a valid finite-value proof. The corrected adapter uses signed inputs, the production FP16 flat-half stream, and a finite comparator; all twenty rerun rows pass CPU reference and validation.

M38a also proved that the large timing disparities track buffer memory type. Kaiju formerly chose coherent system memory; it now prefers coherent host-visible device-local memory with a tested fallback. Production Prometheus remains on its staged pure-device-local path. Exact causes, controlled same-memory experiments, identity/dispatch tables, and timestamp traces are in `PROMETHEUS_M38A_CROSS_RUNTIME_ROOT_CAUSE_ANALYSIS.md`.

### Post-fix representative workload: M=512, N=512, K=512

| Kernel | Prometheus median ns | Kaiju median ns | Outputs match |
|---|---:|---:|---|
| scalar | 1,101,120 | 1,108,384 | yes |
| tiled | 806,368 | 805,696 | yes |
| memory-conservative | 1,077,600 | 1,076,896 | yes |
| scalar-plus | 1,083,392 | 1,083,200 | yes |
| tile16 | 1,033,888 | 1,033,952 | yes |
| SRT | 1,105,472 | 1,108,224 | yes |
| B2x2 | 585,152 | 583,840 | yes |
| A2x4 | 1,159,104 | 369,888 | yes |
| Packed4 | 4,121,376 | 1,107,840 | yes |
| FP16 | 4,080,352 | 1,101,184 | yes |

### Post-fix odd/tail workload: M=127, N=131, K=129

| Kernel | Prometheus median ns | Kaiju median ns | Outputs match |
|---|---:|---:|---|
| scalar | 17,760 | 17,664 | yes |
| tiled | 19,456 | 19,616 | yes |
| memory-conservative | 15,616 | 15,936 | yes |
| scalar-plus | 19,392 | 19,552 | yes |
| tile16 | 29,824 | 30,208 | yes |
| SRT | 15,008 | 15,232 | yes |
| B2x2 | 20,544 | 20,736 | yes |
| A2x4 | 186,880 | 64,160 | yes |
| Packed4 | 47,040 | 13,472 | yes |
| FP16 | 65,920 | 19,968 | yes |

## M38b production disposition

M38b tested the packed-kernel placement effect across exact A/B/C classes, seven shapes, cold/warm/re-upload/output-turnover modes, cache perturbation, production competitors, and end-to-end result availability. The historical M37b/M38a timing gaps did not become a repeatable production rule: current exact-class representative rows were approximately 1.10 ms regardless of pure versus mapped device-local placement, while independent rounds showed large shared drift and no stable ordering. Mapping output `C` caused a decisive end-to-end regression.

Use `PROMETHEUS_M38B_PACKED_MEMORY_PLACEMENT_EXPERIMENT.md` for the production decision. Packed4 and FP16 receive research-only placement classifications; no memory profile is activated and the default selector/staged path remains unchanged.
