# P8d.1 Report — Tiled Path Reachability Stabilization

## 1) Original reachability bug

P8d introduced a tiled SGEMM compute pipeline, but runtime compute-mode selection only allowed tiled mode when `selected_path == PROM_VK_PATH_DIRECT`.

At the same time, path selection routes large workloads to staged execution on device-local-capable runtimes (`can_stage != 0 && small_shape == 0`), so real discrete-GPU-sized workloads were typically staged and therefore never eligible for tiled compute.

Result: the tiled path was effectively dead for the workloads where it should matter most.

## 2) Exact fix applied

`reactor_vulkan.c` was changed so tiled compute is selected independent of direct/staged path:

- `PROM_TESTCFG_FORCE_TILED_PATH` now forces tiled mode on both direct and staged paths.
- auto tiled policy (`tiled_shape`) now applies on both direct and staged paths.

No path-routing heuristic was redesigned; only the compute-mode gating condition was corrected.

## 3) Why staged+tiled is the chosen design

Large workloads are already routed to staged execution on device-local-capable hardware by existing policy. Enabling tiled compute there:

- makes tiled path reachable in the structurally correct high-workload path,
- avoids threshold hacks that force a contrived direct+tiled window,
- keeps staging policy intact while fixing architectural reachability.

## 4) Output semantics verification

Tiled shader semantics are verified as overwrite (not accumulation onto prior C):

- P8d shader contract documents accumulation from `sum = 0` and final `C[row * n + col]` write.
- Runtime binds `shader_c` as output storage; no read-from-C path exists in SGEMM setup.

Therefore staged device-local C does not require pre-zeroing for correctness under current kernel semantics, and repeated calls do not compound stale C contents.

## 5) Tests proving reachability and correctness

Marionette SGEMM tests now cover:

1. staged-capable large-shape auto-selection to `staged + tiled` with CPU-oracle validation,
2. forced `staged + tiled` exact-multiple shape correctness,
3. forced `staged + tiled` non-multiple shape correctness,
4. forced `staged + tiled` rectangular shape correctness,
5. regression that small shapes still avoid auto tiled selection,
6. staged+tiled repeated-call overwrite safety (no stale output compounding).

## 6) Diagnostics / observability outcome

Success detail codes now preserve both path and compute mode:

- `PROM_DETAIL_PATH_DIRECT`
- `PROM_DETAIL_PATH_STAGED_UPLOAD`
- `PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK`
- `PROM_DETAIL_PATH_FALLBACK_TO_DIRECT`
- `PROM_DETAIL_PATH_DIRECT_TILED`
- `PROM_DETAIL_PATH_STAGED_UPLOAD_TILED`
- `PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED`

Backward alias `PROM_DETAIL_PATH_TILED` is preserved as `PROM_DETAIL_PATH_DIRECT_TILED`.

## 7) Intentionally deferred

Still deferred (by design):

- tile-size retuning/auto-tuning,
- shader algorithm changes,
- staging policy redesign,
- async/overlap/perf benchmarking work.

## Inconsistency surfaced

P8d report described tiled path as “direct-buffer mode only”. P8d.1 intentionally resolves that now-outdated policy statement by enabling staged+tiled reachability without broad runtime redesign.
