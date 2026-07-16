# Prometheus M38a cross-runtime root-cause analysis

Status: **COMPLETE** on NVIDIA GeForce RTX 3070, Vulkan 1.4.329, 2026-07-15.

M38a explains the four large M37b disparities without changing shader source, shader IDs, registry policy, or selection rankings. The dominant variable was storage-buffer memory type. Kaiju allocated the first host-visible coherent type (system memory, flags `6`); Prometheus production used staged device-local buffers (flags `1`). B2x2 and A2x4 are extremely slow against system memory, while Packed4 and FP16 are unusually faster there than against Prometheus's pure device-local heap. Controlled same-memory runs converge across runtimes.

M38a also found three defects in the original Kaiju benchmark adapter: signed input generation underflowed through `uint32`, FP16 reused Packed4's row-padded/column-packed payload instead of the production flat half stream, and the result comparator could accept NaN/Inf. These defects are fixed and all twenty corrected paired rows pass CPU-reference comparison and validation.

## Artifact identity

Both adapters extract the exact arrays compiled into the production Prometheus registry/native runtime. The permanent SPIR-V reflection test verifies entry point, `LocalSize`, descriptor set/bindings, a 12-byte push-constant interface, and absence of specialization constants. The machine table is emitted as `out/test-artifacts/m38a_identity_table.json`.

| Kernel | Bytes | SHA-256 (both runtimes) | Entry | Local size |
|---|---:|---|---|---|
| scalar | 2668 | `e5610fc9a63daea8e83188a1cb9c63856225f85b2e6d61bd1e3fc7dcf560a5d9` | `main` | 8x8x1 |
| tiled | 3540 | `936a8ee97960e3ab208ec23e3a1ed4b4816188d5662f3c00adb310a4ac3dc287` | `main` | 8x8x1 |
| memory-conservative | 4356 | `be67a1de9e74081ff3aaec6952a920d7315736a2e00058787a2593c0e11d28b2` | `main` | 8x8x1 |
| scalar-plus | 2956 | `06e583fbc000929a9321b6559f2e3f943f618ffeadeedc95d48c90ff919631dd` | `SgemmScalarBaselinePlus8x8_CS` | 8x8x1 |
| tile16 | 5824 | `950b4148cdac20351352dbaa5d6fc822d802975decdba9b52995759f7ff460a4` | `SgemmTile16x16SharedFp32_CS` | 16x16x1 |
| SRT | 2268 | `78ee174ba47fa508b1fc4baa41f150726f5fc56cc2b28a7d564b882a0cdcd802` | `SgemmSrt2AccumK_CS` | 8x8x1 |
| B2x2 | 3776 | `900fda8225422ae85fd211bb41e639a9b3dbbc10118c111f711554b46403042e` | `SgemmB2x2_CS` | 8x8x1 |
| A2x4 | 5980 | `be00306ed7e0ad8f3684c3c60b2f6de37d64b9cc651fcf1ad4653a682b8c9f15` | `SgemmA2x4_CS` | 8x8x1 |
| Packed4 | 2016 | `91b080bac53b744295bffdbc096669f225d3e611eff3a8b10fc0a33aafcf89e1` | `SgemmPacked4_CS` | 8x8x1 |
| FP16 | 2468 | `a4c175c49c339fdc01e667db9eee0d3b7d186d7d4a6b36ff658ce8eaa3a8eb6d` | `SgemmFp16StorageFp32Accum_CS` | 8x8x1 |

Every module has descriptors `(set=0,binding=0..2)`, storage-buffer descriptor type, one 12-byte `M,N,K` push range, entry point shown above, and no specialization constants. The requests contain byte-identical push constants, logical dimensions, packed payloads, zero-initialized `C`, and one dispatch per sample. Pipeline creation occurs once before warmup/measured dispatches in both bounded adapters; neither uses a pipeline cache or hidden substitute module.

## Dispatch contracts

The complete 40-row runtime table is emitted as `out/test-artifacts/m38a_dispatch_contracts.json`. The two runtime rows are identical for every kernel/workload. Key rows are:

| Kernel | Runtime | Footprint | Local size | Groups 512³ / odd | Total invocations 512³ / odd | Useful invocations 512³ / odd | Over-dispatch 512³ / odd | Dispatches/sample |
|---|---|---|---|---|---:|---:|---:|---:|
| B2x2 | Prometheus | 2x2 | 8x8x1 | 32x32x1 / 8x9x1 | 65,536 / 4,608 | 65,536 / 4,224 | 1.000 / 1.091 | 1 |
| B2x2 | Kaiju | 2x2 | 8x8x1 | 32x32x1 / 8x9x1 | 65,536 / 4,608 | 65,536 / 4,224 | 1.000 / 1.091 | 1 |
| A2x4 | Prometheus | 2x4 | 8x8x1 | 32x16x1 / 8x5x1 | 32,768 / 2,560 | 32,768 / 2,112 | 1.000 / 1.212 | 1 |
| A2x4 | Kaiju | 2x4 | 8x8x1 | 32x16x1 / 8x5x1 | 32,768 / 2,560 | 32,768 / 2,112 | 1.000 / 1.212 | 1 |
| Packed4 | Prometheus | 1x1 | 8x8x1 | 64x64x1 / 16x17x1 | 262,144 / 17,408 | 262,144 / 16,637 | 1.000 / 1.046 | 1 |
| Packed4 | Kaiju | 1x1 | 8x8x1 | 64x64x1 / 16x17x1 | 262,144 / 17,408 | 262,144 / 16,637 | 1.000 / 1.046 | 1 |
| FP16 | Prometheus | 1x1 | 8x8x1 | 64x64x1 / 16x17x1 | 262,144 / 17,408 | 262,144 / 16,637 | 1.000 / 1.046 | 1 |
| FP16 | Kaiju | 1x1 | 8x8x1 | 64x64x1 / 16x17x1 | 262,144 / 17,408 | 262,144 / 16,637 | 1.000 / 1.046 | 1 |

B2x2 independently applies `ceil(M/2)` and `ceil(N/2)`, then divides each by local size 8. Tests cover dimensions 1, 2, 3, 16, 17, and 512. A2x4 independently applies `ceil(M/2)` and `ceil(N/4)`, then local size; `(M,N,K)=(3,17,7)` records 1x1x1 and `(512,512,8|512)` records 32x16x1. Neither footprint nor local size is applied twice, X maps rows, Y maps columns, and no dispatch repeats.

At 512³, A2x4 canonical 2x4 produces 32,768 invocations. Historical 2x2 would produce 65,536 (2x), and generic 1x1 would produce 262,144 (8x). The actual command arguments are canonical in both paths.

Buffer contracts are also equal. At 512³, f32/Packed4 A/B/C are 1,048,576 bytes each and FP16 A/B/C are 524,288/524,288/1,048,576. At 127x131x129, f32 is 65,532/67,596/66,548; Packed4 is 67,056/69,168/66,548 with padded K=132; FP16 is 32,768/33,800/66,548 with two half lanes per `u32` and unpadded logical K=129.

## Timestamp and command traces

Prometheus normalized sequence:

1. Record upload barriers/copies and transfer-to-compute barrier (staged path), then bind pipeline/descriptors and push constants.
2. `vkCmdResetQueryPool`; `vkCmdWriteTimestamp(COMPUTE_SHADER)`.
3. One `vkCmdDispatch`.
4. `vkCmdWriteTimestamp(COMPUTE_SHADER)`.
5. Record compute-to-transfer barrier, C readback copy, and transfer-to-host barrier.
6. End/submit command buffer; wait fence on host; retrieve both query results on host.

Only dispatch is between Prometheus timestamps. Upload, output handling, command recording, queue submission, fence wait, result retrieval, predictor accounting, packing, and CPU conversion are outside the query interval. C is not cleared because every valid output element is overwritten.

Kaiju normalized sequence:

1. Reset/begin its reusable command buffer.
2. `vkCmdResetQueryPool`; `vkCmdWriteTimestamp(TOP_OF_PIPE)`.
3. Bind pipeline/descriptors, push constants, and issue one `vkCmdDispatch`.
4. `vkCmdWriteTimestamp(BOTTOM_OF_PIPE)`; end/submit.
5. Wait fence on host; retrieve query results with 64-bit/wait flags; read mapped output after completion.

Kaiju has no barriers, copies, clears, or fills in the command buffer. Its wider timestamp stages include the bind/push command region, unlike Prometheus's dispatch-only compute-stage interval. This boundary difference is real but not causal: same-memory kernel medians converge to 0.3-5.4% across runtimes, including the anomalous shaders.

## Queue, memory, and pipeline comparison

Both select the same RTX 3070 and queue family 0. Kaiju reports queue flags 15 (graphics, compute, transfer, sparse); Prometheus also submits compute to family 0. Neither uses a dedicated-compute queue. Buffers are exclusive-sharing, bound at offset 0, with Vulkan-reported alignment captured in the machine trace. Kaiju uses storage usage 32. Prometheus device A/B use storage+transfer-dst (34), C uses storage+transfer-src (33).

Memory property bits are `DEVICE_LOCAL=1`, `HOST_VISIBLE=2`, `HOST_COHERENT=4`:

| Path | Memory type | Flags | Behavior |
|---|---:|---:|---|
| Prometheus production staged | 1 | 1 | pure device-local buffers; staging/copies outside timestamps |
| Prometheus controlled direct | 3 | 6 | mapped coherent system memory |
| Kaiju original/default before fix | 3 | 6 | mapped coherent system memory |
| Kaiju fixed default | 5 | 7 | mapped coherent device-local memory; flags-6 fallback retained |

Both build one compute pipeline from the same module/entry, same three storage descriptors, same push range, no specialization data, no derivative/base pipeline, and no material compute-pipeline flags. Pipeline creation is outside timing. Reusing one Prometheus audit pipeline for all samples rejected first-dispatch/pipeline creation as a cause; Packed4 and FP16 remained near 4.1 ms on pure device-local memory.

## Numerical hypothesis tests and causes

| Hypothesis | Numerical prediction | Observation | Result |
|---|---|---|---|
| B2x2 treated as 1x1 | 4x invocation count | Old Kaiju/Prometheus was 16.97x; actual groups were 32x32 | rejected |
| B2x2 system-memory sensitivity | Kaiju corrected flags-6/flags-7 = 9.864/0.584 = 16.88x | Old ratio 9.931/0.585 = 16.97x | confirmed |
| A2x4 historical 2x2 | 2x invocations | Old ratio 7.68x; actual groups 32x16 | rejected |
| A2x4 generic 1x1 | 8x invocations | Superficially near 7.68x, but command trace is canonical and memory-only change removes it | rejected |
| A2x4 system-memory sensitivity | Same flags-6 runtimes should approach 1x | Prometheus/Kaiju flags-6 = 8.551/8.236 = 1.038x | confirmed |
| Packed4 Prometheus interval includes copies | Direct memory alone should not collapse timing | Pure-local/direct = 4.117/1.100 = 3.74x and direct Prometheus/Kaiju = 1.100/1.099 = 1.001x | rejected; memory confirmed |
| FP16 Prometheus interval includes packing/copies | Direct memory alone should not collapse timing | Pure-local/direct = 4.090/1.171 = 3.49x; original Prometheus/Kaiju = 3.44x | rejected; memory confirmed |
| Duplicate dispatch | integer ~2x with dispatch count 2 | both traces record exactly one | rejected |
| Warm mapped-input cache | re-upload before every dispatch should change medians | local/re-upload ratios remained ~1.00 | rejected |

Per-kernel conclusions:

- **B2x2:** original Kaiju allocation of coherent system memory. Canonical 2x2 dispatch was always correct. Changing only Kaiju memory selection gives 583,840 ns, matching Prometheus 585,152 ns.
- **A2x4:** original Kaiju coherent system memory, not historical/generic footprint metadata. Canonical dispatch is 32x16. Fixed Kaiju's mapped device-local type is 369,888 ns, faster than Prometheus pure device-local type at 1,159,104 ns; the remaining 3.13x inverse gap is a proven memory-type/heap sensitivity (system: ~8.2-8.6 ms, pure local: ~1.16 ms, mapped local: ~0.370 ms), not a dispatch or runtime-contract bug.
- **Packed4:** Prometheus pure device-local storage is the slow case. Its controlled flags-6 path is 1,100,288 ns versus Kaiju flags-6 1,099,072 ns. No packing, copy, clear, or duplicate dispatch is inside the timestamp.
- **FP16:** same memory-placement sensitivity. Prometheus flags-6 is 1,170,784 ns versus Kaiju flags-6 1,148,256 ns. The corrected FP16 adapter uses flat row-major packed halves and remains correct.

## Confirmed fixes and production safety

1. Kaiju's allocator now deterministically prefers `HOST_VISIBLE|HOST_COHERENT|DEVICE_LOCAL`, falling back to host-visible coherent memory. An audit-only override preserves controlled flags-6/flags-7 experiments. A permanent selector test covers preference, forced choices, and fallback.
2. The M37b adapter now generates signed finite inputs, uses the production FP16 flat-half layout, and rejects non-finite comparisons. Packing/byte-size tests cover aligned and padded workloads.
3. The Prometheus bounded timing adapter now creates one pipeline around warmup plus measured samples, instead of recreating it per sample. It still invokes the unchanged production command path once per sample.
4. Both traces now expose queue, memory, alignment/offset, exact timestamp commands/stages, query reset/retrieval, and dispatch count in typed/machine-readable results.

Prometheus shader source, shader IDs, registry metadata, selection policy, resource bindings, push constants, production async/ring ownership, physical slot recycling, and production path choice are unchanged. Kaiju buffer selection changes; its shader and request policy do not. Diagnostic memory/re-upload controls are audit-only.

## Before and after timings

One warmup and five measured query-pool iterations; same GPU/driver/artifacts and validation enabled.

Representative 512x512x512:

| Kernel | Prom before | Kaiju before | Prom after | Kaiju after |
|---|---:|---:|---:|---:|
| scalar | 1,100,544 | 1,125,408 | 1,101,120 | 1,108,384 |
| tiled | 805,888 | 839,904 | 806,368 | 805,696 |
| memory-conservative | 1,077,472 | 1,083,008 | 1,077,600 | 1,076,896 |
| scalar-plus | 1,083,104 | 1,139,072 | 1,083,392 | 1,083,200 |
| tile16 | 1,034,368 | 1,036,736 | 1,033,888 | 1,033,952 |
| SRT | 1,105,280 | 1,111,808 | 1,105,472 | 1,108,224 |
| B2x2 | 585,088 | 9,931,008 | 585,152 | 583,840 |
| A2x4 | 1,141,856 | 8,766,304 | 1,159,104 | 369,888 |
| Packed4 | 3,841,696 | 1,114,144 | 4,121,376 | 1,107,840 |
| FP16 | 4,082,336 | 1,186,464 | 4,080,352 | 1,101,184 |

Odd/tail 127x131x129:

| Kernel | Prom before | Kaiju before | Prom after | Kaiju after |
|---|---:|---:|---:|---:|
| scalar | 17,504 | 42,208 | 17,760 | 17,664 |
| tiled | 19,424 | 31,168 | 19,456 | 19,616 |
| memory-conservative | 15,776 | 28,992 | 15,616 | 15,936 |
| scalar-plus | 19,168 | 44,128 | 19,392 | 19,552 |
| tile16 | 29,984 | 31,040 | 29,824 | 30,208 |
| SRT | 14,144 | 29,824 | 15,008 | 15,232 |
| B2x2 | 20,576 | 541,952 | 20,544 | 20,736 |
| A2x4 | 179,328 | 441,504 | 186,880 | 64,160 |
| Packed4 | 41,984 | 23,808 | 47,040 | 13,472 |
| FP16 | 66,208 | 45,760 | 65,920 | 19,968 |

All after rows pass CPU reference. Validation was requested, available, enabled, and reports zero warnings/errors and no device loss in both hardware lanes.

## Remaining performance question and recommendation

There is no unexplained host/runtime disparity left: forcing the same flags-6 memory makes all four anomalous kernels agree closely, while changing only memory class reproduces the large ratios. The remaining differences are genuine kernel sensitivity to the RTX 3070 memory type/heap and generated memory-access behavior.

The next optimization target should be **Packed4/FP16 memory access on Prometheus's production pure device-local path**, followed by A2x4's three-tier heap sensitivity. That work should inspect generated device code/access coalescing and controlled heap residency before changing shader ranking or source. M38a makes no such optimization.

Convergence outcome: **SUCCESS**

Milestone state: **COMPLETE**

## M38b production disposition

M38b preserved this root-cause result as historical controlled evidence, then tested whether it could support a production placement preference. It could not. On the same RTX 3070/596.36 system, the exact A/B/C matrix and seven-shape workload matrix did not retain a stable mapped-device-local kernel advantage; representative Packed4 and FP16 rows were approximately 1.10 ms in both pure and mapped device-local memory. Cache-perturbed independent rounds drifted substantially without a durable class ordering, and mapped `C` was 4x or more worse end to end.

Therefore M38a's recommendation to investigate placement is closed by `PROMETHEUS_M38B_PACKED_MEMORY_PLACEMENT_EXPERIMENT.md`: no production profile is approved, production policy remains unchanged, and no undocumented NVIDIA explanation is claimed.
