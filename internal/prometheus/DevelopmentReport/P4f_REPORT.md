# P4f Report — Performance Characterization + Low-Risk Optimization

## What P4f changed

1. Added explicit SGEMM timing/reporting fields:
   - `CPUTimeNs`
   - `VulkanTimeNs`
   - `VulkanEnv`
2. Kept correctness gate behavior unchanged (CPU oracle remains authoritative).
3. Reduced per-call Vulkan overhead by moving reusable objects into `prometheus_runtime`:
   - descriptor pool + descriptor set
   - command buffer
   - submit fence
4. Applied push-constant safety fix by making host push-constant payload 16 bytes (`m,n,k,reserved0`) to keep alignment/layout conservative across vendors.
5. Added explicit software-Vulkan detection path (CPU device type and/or `llvmpipe` name match), surfaced through probe backend type.

## Measurement corpus

As required, P4f uses exactly this shape set:
- `(1,1,1)`
- `(4,4,4)`
- `(16,16,16)`
- `(32,32,32)`
- `(3,5,7)`

## Measured output (this environment)

Command:

```bash
go run ./cmd/oct prometheus-sgemm cpu
```

Observed:

- `M=1,N=1,K=1`: `cpu=694ns`, `vulkan=0ns`, `vulkan_env=not_applicable`
- `M=4,N=4,K=4`: `cpu=345ns`, `vulkan=0ns`, `vulkan_env=not_applicable`
- `M=16,N=16,K=16`: `cpu=5878ns`, `vulkan=0ns`, `vulkan_env=not_applicable`
- `M=32,N=32,K=32`: `cpu=55434ns`, `vulkan=0ns`, `vulkan_env=not_applicable`
- `M=3,N=5,K=7`: `cpu=432ns`, `vulkan=0ns`, `vulkan_env=not_applicable`

Command:

```bash
OCT_PROMETHEUS_REACTOR=/tmp/missing-reactor.so go run ./cmd/oct prometheus-sgemm prometheus
```

Observed:

- all runs reported `status=fallback(prometheus_unavailable)`
- `backend_requested=prometheus backend_used=cpu`
- `vulkan=0ns`
- `vulkan_env=unavailable`

## Environment honesty

In this container run, no Prometheus reactor shared library was available at the configured path, so Vulkan execution could not be measured end-to-end here.

P4f reporting now explicitly distinguishes:
- `hardware_vulkan`
- `software_vulkan_llvmpipe_or_cpu`
- `unavailable`
- `not_applicable`

When software Vulkan is detected, run notes explicitly warn that timings are not representative of hardware GPU acceleration.

## Bottlenecks observed

Before this patch, SGEMM paid repeated setup costs per call:
- descriptor pool/set allocation
- command buffer allocation
- fence creation/destruction

These are now amortized by runtime-level reuse. Remaining per-call work is primarily:
- buffer allocate/map/copy
- command recording + submit/wait
- output copy

## Correctness status

Correctness remains unchanged:
- CPU oracle still gates all paths.
- fallback status remains explicit and truthful.
- Prometheus execution failures still report explicit stage/detail status (not fallback).
