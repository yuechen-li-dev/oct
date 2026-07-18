# EVT-2 M0.75 — Z-Image FP16 Cache and M1 Baseline

Convergence outcome: **SUCCESS**  
Milestone state: **COMPLETE**  
EVT-2 state: **READY FOR M1**

The dependency-free Go cache writer reads only the thirteen `noise_refiner.0`
BF16 ranges from the verified source checkpoint, validates its full SHA-256 and
each tensor contract/raw hash, then writes atomic little-endian FP16 files below
`$OCT_EVT2_CACHE/layers/<source-hash>/noise_refiner.0/`.  Matrix files are
reversible `[out,in]` → `[in,out]` transposes; vectors are preserved; fused QKV
remains one `[3840,11520]` file with logical Q/K/V views in that order. No full
checkpoint copy is created.

Transform `zimage-noise-refiner.0-bf16-fp16-transpose-v1` produced aggregate
SHA-256 `a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e`
and 361,820,672 bytes. Two independent generations were byte-identical; a
generation took 8.5 seconds and peaks at one source plus one destination tensor
(88 MiB each). Conversion is IEEE BF16 bits → exact Float32 semantic value →
FP16 nearest-even. Signed zero/infinity are preserved; FP16 subnormal rules
apply; NaNs canonicalize to signed quiet `0x7e00`.

The real isolated Comfy block witness restored all cache values into the exact
M1 boundary. An activation-fixed Float32 control versus cache-value execution
has output L2 `1.9148956e-05`, L∞ `7.6293945e-06`, RMS `9.6567172e-09`, and
relative L2 `1.2938293e-08`, with all finite values. This is the first M1
authority. The original BF16 oracle remains distinct: its delta to the
cache-value/FP32-compute witness is relative L2 `0.0037042543`, dominated by
execution arithmetic rather than converted weights. M1 policy is **FP16 cache
with FP32 norm/reduction/softmax/RoPE/scheduler**, and it must report these two
comparisons separately.

Clean-process, validation-enabled required-hardware Vulkan preflight passes on
the RTX 3070 (vendor 4318, `caps_available=1`, `vk_result=0`), as do reduction
correctness, M42–M49b, replay, fault/quarantine/reap, and warm allocation lanes.
The 429-test monolith still later reaches `vk_result=-3` / `reason_code=2` and
skips its preflight, while a fresh process immediately passes. This is frozen as
a process-global initialization-lifetime limitation; M1 requires the clean
process lane. No runtime or shader behavior changed.

M1 now implements only `noise_refiner.0`: captured `[1,1024,3840]` input,
`[1,256]` timestep, 30×128 heads, fused QKV views, three-axis RoPE, and
AdaLN/tanh maps. Python/Comfy remains reference-only; product M1 consumes the
Go cache and Prometheus runtime paths.
