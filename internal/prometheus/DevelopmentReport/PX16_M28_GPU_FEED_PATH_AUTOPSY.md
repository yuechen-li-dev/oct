# Prometheus Px16 M28 — GPU feed-path autopsy

## Result

Classification: **B, submission/synchronization-bound (with kernel cost still material).** The resident benchmark’s normal depth-one path is architecturally serial: a reusable command buffer and fence are reset, one command buffer is submitted, the CPU waits for it, and only then obtains the timestamp result and starts the next iteration. The diagnostic batch removes that host round-trip without touching production dispatch; on the RTX 3070 it preserves about 0.34 ms kernel time per 512³ DERIVE dispatch while the depth-one wall time is about 0.414 ms per dispatch.

## Code-order map and synchronization points

Production staged API path (`prom_reactor_runtime_sgemm_impl`) performs selector/P15 feed-forward preparation, buffer/descriptor preparation, records transfer/compute/readback work, `vkQueueSubmit`, then `vkWaitForFences` at `reactor_vulkan_sgemm.c:6894`. After completion it reads timestamp results and updates P14/P15 (`:6906–:7000`), copies host-visible C, publishes diagnostics, and returns. The dedicated-transfer path additionally submits its transfer command buffer and waits for `transfer_submit_fence` (`:6851`, `:6885`).

Resident benchmark setup uploads once through the normal staged path. Each timed depth-one iteration calls `prom_sgemm_resident_dispatch_once` (`:7189`): descriptor update → `vkResetCommandBuffer`/record → reset its single `submit_fence` → `vkQueueSubmit` (`:7333`) → `vkWaitForFences` (`:7344`) → `vkGetQueryPoolResults` (`:7355`, no WAIT flag because the fence already completed). Optional validation is a separate command buffer, submit, fence wait, and host memcpy (`:7385`). No `vkQueueWaitIdle` or `vkDeviceWaitIdle` occurs in these hot SGEMM paths.

## Runtime architecture

| Queue family/count | command/fence strategy | descriptor/query strategy | residency/readback |
|---|---|---|---|
| one selected compute queue; optional dedicated transfer queue | one reusable compute command buffer and one reusable fence; reset only after completion | one reusable descriptor set, written every resident dispatch; one two-slot timestamp pool, reset every dispatch | A/B/C device-local after setup; readback only when requested, after timing |

## Per-dispatch operation counts

| mode | submits | waits | query reads | recordings/resets | descriptor updates | readbacks during timing |
|---|---:|---:|---:|---:|---:|---:|
| resident serial depth 1 | 1 | 1 | 1 | 1/1 | 1 | 0 |
| M28 diagnostic batch depth D | 1 per D dispatches | 1 per D | 1 aggregate | 1/1 per D | 1 per D | 0 |
| staged API | compute submit + optional transfer submit | compute fence + optional transfer fence | 1 | 1/1 | 1 | 1 |

## Measured batch scaling

Device: NVIDIA GeForce RTX 3070 (as provided). Shape: 512×512×512. Explicit source-backed `SDSL_REG2X2_TILE16X16_DERIVE_FP32`. Ten timed batches/depth, two warmups. Raw machine-readable data: `out/test-artifacts/prometheus_sgemm_px16_m28_feed_path.json`.

| depth | dispatches | submissions/waits | wall ns | aggregate GPU ns | wall ns/dispatch | GPU ns/dispatch | correctness |
|---:|---:|---:|---:|---:|---:|---:|---|
| 1 | 10 | 10/10 | 4,144,500 | 367,168 | 414,450 | 367,168 | pass |
| 2 | 20 | 10/10 | 7,488,400 | 700,160 | 374,420 | 350,080 | pass |
| 4 | 40 | 10/10 | 14,338,400 | 1,366,240 | 358,460 | 341,560 | pass |
| 8 | 80 | 10/10 | 28,858,300 | 2,721,248 | 360,729 | 340,156 | pass |
| 16 | 160 | 10/10 | 55,629,100 | 5,408,192 | 347,682 | 338,012 | pass |

The per-dispatch wall cost falls 16% from depth 1 to 16. Timestamp results show the kernel itself remains roughly 0.34 ms/dispatch; the removed fixed host/queue round-trip is roughly 76 µs at depth one. This is evidence of real feed-path overhead, not proof that it is the only SGEMM limitation.

## Dominatus/P14/P15 granularity

P15 selection/reservation probing occurs before staged dispatch construction (`:5666+`). P14 filtering and P15 predictor/correction/maturation execute only after the completed timestamp is available (`:6928+`). Therefore the staged production API has this dependency: `submit N → fence completion N → timestamp → P14 update → P15 update → API return → caller may submit N+1`. P14/P15 do not call a Vulkan wait themselves, but current API control flow makes their completed-measurement update synchronous and closed-loop. Resident benchmark dispatches intentionally bypass that update, so its observed serialization is directly the command-buffer/fence/query topology rather than Dominatus.

| step | completed GPU measurement required? | blocks next API submit? | future batchable? |
|---|---|---|---|
| selector/P15 feed-forward | no | no within an API call | yes, epoch policy possible |
| fence wait | yes | yes | diagnostic proof exists |
| timestamp/P14/P15 update | yes | yes through API return | yes, semantics need explicit design |
| diagnostics/readback | readback only | yes for staged path | yes when correctness is external |

## Serialization audit

1. Per-dispatch fence wait: present in resident and staged paths; required by current one-fence/one-command-buffer reuse and synchronous API result.
2. Query-result serialization: present after each fence; the query call itself does not use `WAIT_BIT`, but it cannot run early because of the prior fence wait.
3. Queue/device idle calls: absent from the hot SGEMM path.
4. Single reusable fence, query pair, and command buffer: present; each blocks multiple in-flight submits.
5. Descriptor writes and command reset: present per resident dispatch; batched diagnostic reduces both by depth.
6. Host invalidate/readback: excluded from resident timed batches; staged path includes readback.

## Nsight discovery and reproducibility

Installed: `C:\Program Files\NVIDIA Corporation\Nsight Systems 2025.6.3\target-windows-x64\nsys.exe`; `ncu.bat` from Nsight Compute 2026.1.0. Nsight Systems is the appropriate Vulkan CPU/API/queue timeline tool; Nsight Compute was not used as a CUDA profiler. Run `Run-Px16M28NsightSystems.ps1`; it traces `vulkan,wddm` and leaves `.nsys-rep` captures under `out/profiling/px16_m28/` (ignored output). Inspect queue submission spacing, fence waits, and GPU idle gaps. No raw capture is committed. Capture was attempted in this checkout, but Nsight Systems failed to register its Vulkan extension JSON because registry writing permission was unavailable; WDDM/context-switch tracing also requires elevation. Run the launcher from an elevated PowerShell to obtain the timeline.

## Ranked causes and M29

Rank 1: serial submit/fence/reuse cadence. Evidence: one wait and one command recording per depth-one dispatch; depth-16 lowers wall/dispatch 16%. Estimated impact: roughly 76 µs/dispatch at this shape. M29: design a bounded multi-frame/in-flight submission experiment with distinct fence/query/command resources, keeping P14/P15 semantics explicit.

Rank 2: kernel duration. Evidence: even batched D ERIVE remains about 0.338 ms/dispatch. M29 should first retain this feed-path baseline before any kernel work.

Rank 3: staged transfers/readback. Evidence: staged path explicitly owns upload/readback/fences; resident data isolates them but does not eliminate serial compute synchronization.

Limitations: this M28 batch uses repeated writes to the same C because each SGEMM overwrites all output elements; it diagnoses queue feeding but is not production throughput. It provides aggregate rather than per-dispatch timestamps within a batch and does not claim an Nsight capture was taken in this checkout.
