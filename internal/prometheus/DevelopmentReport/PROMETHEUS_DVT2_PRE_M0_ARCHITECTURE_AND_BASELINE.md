# PROMETHEUS DVT-2 Pre-M0 — Architecture and Performance Baseline

## Outcome

This audit freezes the post-bootstrap strangler architecture without reopening accepted transformer arithmetic. The canonical smoke stays Python-led at the outside boundaries and Prometheus-led for the compiled model core.

The authoritative, deterministic audit set is `artifacts/Dvt2PreM0/`. It contains baseline identity, seams, actual lifecycle, cold/warm timing slots, per-evaluation and per-block timing, transport limits, exact model-owned accounting, scaffold inventory, Python-removal map, candidates, reproduction command, and the single M0 decision.

## Measurement discipline

Bridge ABI v2 adds bounded per-block host-visible probes: reactor execution, rebind window, payload-read window, and immutable uploaded bytes. These probes do not alter arithmetic. They are not GPU copy timestamps and must not be represented as PCIe bandwidth.

## M0 decision

M0 is **persistent warm model owner/session across scheduler evaluations**. It targets measured repeated payload reconstruction and owner rebind cost while retaining lock authority, FP32 resident activation, 30-layer execution, and the 643,587,076-byte model-owned ceiling. Prefetch, full typed transport, native scheduler, Qwen, and VAE remain deferred.

## Canonical command

```powershell
& "$env:USERPROFILE\ComfyUI\.venv\Scripts\python.exe" tools\zimage_prometheus_smoke.py --metadata internal\prometheus\DevelopmentReport\artifacts\Dvt2PreM0\dvt2_profile_cold.json
```

The command validates its PNG, records all nine native evaluations, validates the 30-layer count and allocation ceiling, and writes evidence atomically.
