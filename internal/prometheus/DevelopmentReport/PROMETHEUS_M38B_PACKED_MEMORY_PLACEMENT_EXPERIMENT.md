# Prometheus M38b packed-memory placement experiment

Status: **COMPLETE — evidence-based rejection of a production placement preference** on NVIDIA GeForce RTX 3070, vendor `0x10de`, device `0x2488`, driver `596.36` (`driverVersion=2500395008`), Vulkan 1.4.329, 2026-07-15.

M38b tested whether the M38a Packed4/FP16 memory-class effect could become a safe Prometheus production decision. It cannot on the collected evidence. Exact memory-class runs across seven bounded workloads show no persistent kernel advantage for host-visible coherent device-local memory. The final 512³ A/B/C snapshot is flat at about 1.10 ms for both kernels. Across workloads the all-mapped kernel ratio ranges from 0.89x to 1.01x for Packed4 and 0.95x to 1.25x for FP16 relative to pure local, with no stable advantage. Independent cache-perturbed rounds also fail to establish a durable class ordering.

Mapped output is actively unattractive end to end: reading `C` from host-visible coherent device-local memory makes the representative reactor interval about 4.2-4.7x slower. Keeping `C` pure device-local avoids that cost, but mapped inputs still do not produce a repeatable kernel or end-to-end win.

This result does not claim that the controlled M38a observations were fabricated or explain an undocumented NVIDIA mechanism. It shows that the effect did not survive the production-oriented repeatability, workload, role-attribution, and end-to-end tests required to promote it.

## Hypothesis and bounded experiment

Memory placement was treated as part of the execution configuration:

```text
KernelConfiguration {
    ShaderVariant
    WorkloadShape
    APlacement
    BPlacement
    CPlacement
    ReuseMode
}
```

The audit executor consumes unchanged production SPIR-V, entry points, bindings, push constants, layouts, and dispatch metadata. It adds exact-class allocation for:

- pure device-local;
- host-visible coherent system memory;
- host-visible coherent device-local memory.

It does not silently substitute memory types. Unsupported configurations fail as unsupported. Production shader IDs, registry policy, selector ownership, and the default staged path remain unchanged.

The full eight-row placement matrix was run at 512³ for both kernels. The workload matrix used 64³, 256³, 512³, 1024x256x512, 256x1024x512, 127x131x129, and 511x509x513. These cover small, square, tall, wide, odd/tail, Packed4-aligned/hostile K, and FP16 even/odd K. 1024³ was omitted because the seven-shape matrix already bounded the production question while avoiding an unnecessary billion-FMA CPU reference case.

Each important row used a private four-point Vulkan query pool around input transfer, one dispatch, and output transfer. GPU kernel time is the interval immediately around dispatch. Transfer/preparation combines host packing and memory writes, required GPU copies/barriers outside that dispatch interval, and the final host result copy. End-to-end wall time includes submission and synchronization as well. Correctness used the CPU reference, finite-value rejection, Packed4 padding, and the corrected production FP16 flat-half stream.

## Exact memory exposure and capacity

The selected RTX 3070 exposes six memory types. Relevant types are:

| Class | Type | Heap | Property flags | Heap size | Observed budget | Observed usage |
|---|---:|---:|---:|---:|---:|---:|
| pure device-local | 1 | 0 | `DEVICE_LOCAL` (`1`) | 8,406,433,792 | 7,601,127,424 | 6,930,432 |
| host-visible coherent system memory | 3 | 1 | `HOST_VISIBLE|HOST_COHERENT` (`6`) | 16,706,387,968 | 15,901,081,600 | 20,881,408 |
| host-visible coherent device-local memory | 5 | 0 | `DEVICE_LOCAL|HOST_VISIBLE|HOST_COHERENT` (`7`) | 8,406,433,792 | 7,601,127,424 | 6,930,432 |

`VK_EXT_memory_budget` was available. Type 5 exposes the device-local heap rather than a small aperture on this machine, so the tested tensors (at most about 3.7 MiB across A/B/C) fit easily. That capacity fact is device-specific. The prototype still rejects absent types, unknown devices/drivers, over-envelope requests, insufficient budget/headroom, and allocation failure.

## Required table 1 — Packed4 placement results

Representative 512x512x512 final validation snapshot, re-upload mode, one warmup plus nine measured samples:

| A placement | B placement | C placement | Median GPU ns | Transfer/prep ns | End-to-end ns | Relative to pure local |
|---|---|---|---:|---:|---:|---:|
| pure device-local | pure device-local | pure device-local | 1,106,624 | 3,662,716 | 5,180,200 | 1.000x |
| mapped device-local | mapped device-local | mapped device-local | 1,108,128 | 22,358,536 | 23,992,500 | 0.999x |
| mapped device-local | mapped device-local | pure device-local | 1,109,152 | 3,980,612 | 5,496,100 | 0.998x |
| pure device-local | pure device-local | mapped device-local | 1,107,328 | 22,527,224 | 24,161,200 | 0.999x |
| mapped device-local | pure device-local | pure device-local | 1,107,072 | 3,846,732 | 5,351,800 | 1.000x |
| pure device-local | mapped device-local | pure device-local | 1,107,808 | 3,884,760 | 5,431,700 | 0.999x |
| pure device-local | pure device-local | mapped device-local | 1,106,624 | 21,989,864 | 23,597,500 | 1.000x |
| coherent system control | coherent system control | coherent system control | 1,110,560 | 3,646,156 | 5,121,800 | 0.996x |

No A- or B-specific kernel sensitivity is present at representative scale. Mapping `C` does not improve dispatch time and dominates end-to-end cost.

## Required table 2 — FP16 placement results

| A placement | B placement | C placement | Median GPU ns | Transfer/prep ns | End-to-end ns | Relative to pure local |
|---|---|---|---:|---:|---:|---:|
| pure device-local | pure device-local | pure device-local | 1,100,256 | 4,104,152 | 5,600,400 | 1.000x |
| mapped device-local | mapped device-local | mapped device-local | 1,100,832 | 22,386,044 | 24,046,200 | 0.999x |
| mapped device-local | mapped device-local | pure device-local | 1,100,928 | 4,303,172 | 5,806,600 | 0.999x |
| pure device-local | pure device-local | mapped device-local | 1,099,232 | 21,843,924 | 23,424,400 | 1.001x |
| mapped device-local | pure device-local | pure device-local | 1,100,224 | 4,103,648 | 5,591,800 | 1.000x |
| pure device-local | mapped device-local | pure device-local | 1,100,224 | 4,119,200 | 5,599,200 | 1.000x |
| pure device-local | pure device-local | mapped device-local | 1,100,160 | 22,116,600 | 23,758,200 | 1.000x |
| coherent system control | coherent system control | coherent system control | 1,186,816 | 4,426,740 | 6,018,800 | 0.927x |

The corrected flat-half layout remains correct. No operand-specific mapped-device-local benefit appears. The two equivalent C-only rows agree, and coherent system memory remains a diagnostic control only.

## Workload behavior and production competitors

The fastest placement for each packed kernel was ranked once per kernel against scalar, tiled, memory-conservative, B2x2, A2x4, and the unchanged production-selected path. Ranks are among unique kernels, not duplicate placement rows.

### Required table 3 — Best-kernel ranking by workload

| Workload | Fastest kernel/configuration | Packed4 rank / best ns | Packed4 speedup P/S/T/selected | FP16 rank / best ns | FP16 speedup P/S/T/selected | Unchanged selected ns |
|---|---|---:|---:|---:|---:|---:|
| 64x64x64 | Packed4, pure local | 1 / 8,096 | 1.00/1.39/1.19/1.91x | 5 / 11,328 | 1.00/0.99/0.85/1.36x | 15,456 |
| 127x131x129 | Packed4, all mapped | 1 / 30,080 | 1.01/1.25/1.55/1.91x | 5 / 45,440 | 1.00/0.83/1.02/1.27x | 57,568 |
| 256x256x256 | B2x2, pure local | 4 / 148,448 | 1.00/1.03/0.77/1.05x | 7 / 154,912 | 1.00/0.99/0.73/1.00x | 155,616 |
| 511x509x513 | B2x2, pure local | 2 / 1,127,680 | 1.00/1.57/2.29/1.72x | 6 / 2,231,040 | 1.19/0.79/1.16/0.87x | 1,934,752 |
| 512x512x512 | A2x4, pure local | 7 / 2,342,272 | 1.00/0.47/0.34/0.48x | 6 / 1,101,376 | 2.54/1.00/0.73/1.02x | 1,128,096 |
| 1024x256x512 | A2x4, pure local | 6 / 1,107,680 | 1.00/1.00/0.72/0.97x | 5 / 1,090,272 | 1.00/1.02/0.74/0.99x | 1,078,368 |
| 256x1024x512 | A2x4, pure local | 4 / 2,305,632 | 1.00/1.17/0.87/1.21x | 5 / 2,469,088 | 1.00/1.10/0.81/1.13x | 2,796,256 |

`P/S/T/selected` means speedup from best placement versus that kernel's pure-local form, then versus scalar, tiled, and the current production-selected execution. Greater than 1 is favorable. This is a snapshot, not a stable promotion ranking: the competitor and packed rows also move with the temporal drift demonstrated below. Packed4 is attractive for the two smallest cases and second on the large odd/tail case, but placement is not the reason. FP16 does not become a leading production kernel after placement.

### Required table 4 — Kernel-only versus end-to-end speedup

Speedup is pure-device-local time divided by alternate-placement time; greater than 1 is favorable.

| Kernel / comparison | 512³ GPU speedup | 512³ end-to-end speedup | Seven-workload GPU range | Seven-workload end-to-end range |
|---|---:|---:|---:|---:|
| Packed4 all mapped | 0.999x | 0.216x | 0.89x-1.01x | 0.15x-0.32x |
| Packed4 mapped inputs, local C | 0.998x | 0.943x | 0.89x-1.00x | 0.55x-1.09x |
| FP16 all mapped | 0.999x | 0.233x | 0.95x-1.25x | 0.16x-0.40x |
| FP16 mapped inputs, local C | 0.999x | 0.964x | 0.95x-2.54x | 0.50x-1.50x |

There is no bounded envelope with a stable positive placement speedup. The all-mapped end-to-end regression is decisive. Mapped-input/local-output rows sometimes save a few percent and sometimes lose substantially; that is not a production profile.

## Reuse, cache perturbation, and repeatability

Cold allocation, warm reuse, re-upload every iteration, and output-only turnover all remained correct. All-mapped 512³ warm runs spent roughly 39-44 ms in recurring transfer/preparation and 44-49 ms end to end per dispatch because result availability includes reading mapped `C`. One/ten/one-hundred-dispatch runs did not amortize that recurring cost; one-time setup was only about 0.4-0.7 ms.

Five independent 512³ rounds rotated input values, reversed pure/mapped order on alternating rounds, and issued a 32 MiB unrelated device-local fill before each timed dispatch. Round-median GPU time was:

| Kernel | Placement | Round min | Round median | Round max | Cross-round CV |
|---|---|---:|---:|---:|---:|
| Packed4 | pure device-local | 2,415,424 | 3,397,728 | 4,123,840 | 0.18 |
| Packed4 | mapped device-local | 2,407,584 | 2,959,648 | 3,630,848 | 0.17 |
| FP16 | pure device-local | 2,920,352 | 3,414,848 | 3,802,976 | 0.10 |
| FP16 | mapped device-local | 2,699,616 | 3,396,672 | 3,641,888 | 0.10 |

Round medians drift together from roughly 2.4 to 4.1 ms. Alternating order prevents one class from always running first, yet neither kernel retains one winning class: the pure/mapped ranges overlap heavily and cross-round CV is comparable. Separate full-matrix invocations also moved later rows while leaving the early matrix near 1.10 ms. This sequence sensitivity is why no buffer role or placement class is promoted. Per-row min/median/max/p10/p90/CV are preserved in the machine artifact.

The existing persistent submission-ring tests remain the lifecycle authority. M38b did not wire an unproven placement into ring slots. Ring recycling, logical-versus-physical failure, and quarantine/reap behavior were rerun separately and remained clean.

## Required table 5 — Capacity and policy envelope

| Item | Result | Production implication |
|---|---|---|
| mapped device-local type | type 5, flags 7, heap 0 | available on this device only |
| heap size / observed budget | 7.83 GiB / 7.08 GiB | representative tensors fit |
| largest tested A+B+C | approximately 3.7 MiB | evidence does not cover model-scale long-lived tensors |
| allocation count / storage-buffer range limits | 4,294,967,295 / 4,294,967,295 bytes | not the limiting factor in the tested envelope |
| tested shape envelope | through 1024x256x512 / 256x1024x512 and 511x509x513 | no 1024³ production claim |
| prototype maximum | 4 MiB in policy tests | deliberately bounded to evidence |
| required headroom | explicit profile field; 128 MiB in tests | reject before allocation when budget is tight |
| missing type / unknown device or driver | reject | pure-device-local staged fallback |
| allocation failure | explicit failure transition | pure-device-local staged fallback |
| approved kernels | none after evidence review | no runtime activation |

## Allocator/profile prototype and fallback

M38b adds a deterministic, non-autotuning profile decision with stable device identity, driver range, kernel compute mode, minimum shape, maximum bytes, mapped-type availability, budget/usage, and required headroom. It independently represents input and output placement and always names pure device-local as fallback.

Permanent tests cover exact memory-type selection, mapped-device-local preference, mixed A/B/C selection, the Packed4/FP16-only allowlist, absent type, unknown device and driver, disabled profile, shape/capacity/budget rejection, allocation-failure fallback, Packed4 padded sizes, FP16 odd flat-half sizes, finite comparison, and deterministic percentile aggregation.

The prototype is intentionally not connected to the production selector or ring. Evidence did not justify approving Packed4 or FP16, so adding an operational feature flag would create a dormant production path with no supported profile. The existing production staged path is the only runtime behavior.

## Correctness and validation

All placement, workload, reuse, amortization, competitor, and production-selected rows passed their CPU reference. NaN/Inf are rejected. Packed4 padding and FP16 odd-lane sizes passed permanent tests. The validation-enabled run reported:

- validation requested, available, enabled;
- `VK_LAYER_KHRONOS_validation` active with debug utils;
- zero warnings and zero errors;
- no device loss.

Only coherent mapped classes are eligible in this experiment, so no flush/invalidate operation is required. The selector names the Vulkan properties precisely and never calls coherent mapped memory generically “system memory.”

## Decision

**Packed4: Research-only memory-placement phenomenon.** Packed4 itself is competitive on small and odd/tail workloads, but alternate placement provides no stable kernel or end-to-end benefit. Keep placement unchanged; kernel ranking may be considered separately using ordinary production evidence.

**FP16: Research-only memory-placement phenomenon.** Correct placement does not make FP16 consistently competitive. The all-mapped path is end-to-end harmful and mapped-input/local-output behavior is unstable.

Recommended production action: retain the current pure-device-local staged path and unchanged selector. Preserve the bounded selector and exact-class audit seam as diagnostics. Reopen placement only if a future driver/device produces a repeatable, order-controlled, end-to-end win across the same acceptance matrix.

Convergence outcome: **SUCCESS**

Milestone state: **COMPLETE**

The success is a clear production rejection, not a promoted optimization: M38b converts the surprising M38a observation into an actionable decision with deterministic fallback and no speculative hardware claim.
