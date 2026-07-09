## Px16 M16

Px16 M16 adds missing host-side SGEMM timing visibility around command-buffer recording and descriptor-set updates. This milestone is instrumentation only.

Non-goals for M16:

- no descriptor-update optimization;
- no descriptor-write caching;
- no barrier changes;
- no selector retuning;
- no kernel changes;
- no FFT/P16 changes;
- no new SDSL-V language features.

## Hypothesis

The leading hypothesis is H1:

- the production EVT lane already measured upload, submit, wait, readback, and optional GPU kernel timestamps;
- it did not measure the host-side command-recording window;
- `vkUpdateDescriptorSets(...)` is issued on the dispatch-preparation path and was also unmeasured;
- that missing work could explain much of `unaccounted_host_ms`.

## Runtime change

The runtime now exports:

```c
uint64_t px16_m8_last_command_record_wall_ns;
```

The implementation mirrors the existing `px16_m8_last_dispatch_submit_wall_ns` pattern:

1. reset at runtime-timing initialization;
2. bracket the real command-recording window with wall-clock timing;
3. copy the field through `PrometheusSgemmPolicyDiagnostics`.

## What `command_record_ms` means

`command_record_ms` is the wall-clock duration of the command-recording preparation window for the production SGEMM dispatch path.

It intentionally includes:

- `vkResetCommandBuffer`
- `vkBeginCommandBuffer`
- `vkUpdateDescriptorSets(...)` when it is part of that path
- command recording and barriers
- `vkEndCommandBuffer`

Why `vkUpdateDescriptorSets(...)` is included:

- H1 is specifically about command recording plus descriptor rewrites as one unmeasured host-side bucket;
- splitting descriptor timing out in this milestone would add complexity without improving the diagnostic answer;
- if descriptor updates are cheap, the new field will simply come back small, which is still a valid result.

## Accounting rule

EVT timing decomposition now includes:

- `kernel_ms`
- `upload_ms`
- `readback_ms`
- `sync_wait_ms`
- `dispatch_submit_ms`
- `command_record_ms`
- `oracle_ms` / `validation_ms` in validation mode
- `unaccounted_host_ms`

`unaccounted_host_ms` is computed as:

```text
total_wall_ms
  - upload_ms
  - command_record_ms
  - dispatch_submit_ms
  - sync_wait_ms
  - readback_ms
  - kernel_ms   (only when GPU timestamps are valid)
```

## Deep diagnostic lane

Integrated a dedicated Marionette FACT lane:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16DeepDiagnostics
```

Artifacts:

- `out/test-artifacts/prometheus_sgemm_px16_deep_diagnostics.json`
- `out/test-artifacts/prometheus_sgemm_px16_deep_diagnostics.md`

Covered shapes:

- `square_128x128x128`
- `square_256x256x256`
- `square_512x512x512`
- `lowk_1024x1024x64`
- `rect_255x129x65`

Reported fields:

- device name / backend / device type / vendor / device id / driver / API version;
- policy mode;
- requested / selected / executed variant;
- executed path;
- `kernel_ms`, `upload_ms`, `readback_ms`, `sync_wait_ms`, `dispatch_submit_ms`, `command_record_ms`, `unaccounted_host_ms`;
- `VK_INSTANCE_LAYERS`;
- `VK_LOADER_LAYERS_ENABLE`;
- resident loop differential when resident mode is available;
- runtime status plus final stage/detail.

## Environment note

M16 reports the two most common explicit Vulkan layer environment variables:

- `VK_INSTANCE_LAYERS`
- `VK_LOADER_LAYERS_ENABLE`

This does not detect every implicit layer. If `command_record_ms` does not explain the slowdown, the next recommended manual check is:

```bat
vulkaninfo --summary
```

and inspect unexpected `Layers:` entries from overlays, capture tools, or validation injection.

## Subgroup note

The known subgroup-size reporting gap is intentionally left unchanged in M16. That follow-up is isolated and should stay separate from this instrumentation milestone.
