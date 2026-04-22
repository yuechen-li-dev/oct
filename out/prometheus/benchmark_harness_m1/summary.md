# Prometheus benchmark harness M1 summary

Generated at: 2026-04-22T21:52:00Z
Warmup runs: 0
Measured runs: 1

Validated reactor stack components:
- P8c staged memory path
- P8d tiled compute path
- P8d.1 staged+tiled reachability
- P8f judgment seam observability
- P8e/P8e.1 async lifecycle on real hardware where allowed

## Sync cases

| Group | Case | Detail | Status | Correctness | Env | Median outer | Median inner wall |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A | Tiny direct baseline correctness | direct (6101) | ok | True | windows_native_vulkan | 664.487ms | 577.600us |
| A | Forced direct+tiled reachability | direct_tiled (6105) | ok | True | windows_native_vulkan | 662.135ms | 1.012ms |
| A | Forced staged baseline reachability | staged_upload_readback (6103) | ok | True | windows_native_vulkan | 687.802ms | 1.491ms |
| A | Forced staged+tiled reachability | staged_upload_readback_tiled (6107) | ok | True | windows_native_vulkan | 656.200ms | 1.013ms |
| B | Non-multiple staged+tiled tails | staged_upload_readback_tiled (6107) | ok | True | windows_native_vulkan | 654.746ms | 502.100us |
| B | Rectangular staged+tiled correctness | staged_upload_readback_tiled (6107) | ok | True | windows_native_vulkan | 644.984ms | 1.119ms |
| B | Natural auto-selected staged+tiled | staged_upload_readback_tiled (6107) | ok | True | windows_native_vulkan | 650.238ms | 1.025ms |

## Async

Outcome: ok
Environment: windows_native_vulkan
Query lifecycle: ready
Submit detail: staged_upload_readback_tiled
Consume detail: staged_upload_readback_tiled
Median async wall: 502.200us

## Conclusions

Staged+tiled live on real hardware: True
Natural auto-selection reached staged+tiled: True
Async path outcome on real hardware: ok

Scope intentionally left for later:
- threshold retuning
- new kernels
- judgment-engine redesign
- broad async redesign
