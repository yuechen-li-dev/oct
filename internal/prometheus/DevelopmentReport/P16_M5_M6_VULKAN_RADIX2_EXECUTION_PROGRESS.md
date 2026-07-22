# P16 M5/M6 — Production Vulkan Complex32 Radix-2 FFT

## Status

`SUCCESS` — production-generated shaders, actual Vulkan execution, independent
numerical proof, and bounded RTX 3070 benchmark evidence are present. The
previous M4 host-side calculation is historical and retired.

## Current contract

- `PrometheusComplex32` is two adjacent IEEE-754 `float` values: real, then
  imaginary.
- Forward is unnormalized with `exp(-2*pi*i*k/N)`.
- Inverse is normalized in the final butterfly pass with
  `exp(+2*pi*i*k/N) / N`.
- `N` is nonzero power-of-two, bounded to `2^20`; `batch_count` is nonzero.
- `stride_elements == 0` means tightly packed `N`; otherwise it is the number
  of Complex32 elements between batch starts and must be at least `N`.
- Input/output aliasing is safe because input and output are copied into
  independent host staging buffers before GPU recording. Interior output
  padding is preserved by initializing both ping/pong device buffers from the
  supplied output storage and only writing active transform locations.
- Calls are synchronous: fence completion and readback occur before return.

## Route

The production sources are:

- `shaders/sdslv/production/fft/radix2_bit_reverse.sdslv` (shader id 52)
- `shaders/sdslv/production/fft/radix2_butterfly.sdslv` (shader id 53)

Both use the canonical SDSL-V → generated HLSL → DXC → SPIR-V → generated
header route. DXC uses `-spirv -T cs_6_0 -fspv-target-env=vulkan1.3`; the
manifest declares ProductionVulkan14 / SPIR-V 1.6 / Vulkan 1.4 validation.
The fixed baseline twiddle strategy is ordinary HLSL `sin/cos`; there is no
CORDIC, recurrence, table selector, or adaptive radix policy.

`reactor_vulkan_fft.c` obtains only borrowed generic services through the M4c
seam. It records input/output staging transfer, GPU bit reversal, one GPU
butterfly dispatch for every deterministic plan pass, explicit compute
write-to-read barriers, and final readback. Ping/pong begins at bit-reversal
output `PING`; the final buffer is PONG for odd pass counts and PING for even
pass counts (including N=1).

## Evidence completed locally

- canonical SDSL-V checks and DXC/SPIR-V validation for both shaders;
- generated headers and registry assets 52/53;
- Oct Make `MarionetteTests` SerialCanonical build;
- RTX 3070 native tests with `PROMETHEUS_VK_VALIDATION=1`: 12/12 focused
  tests, including direct double-precision DFT authority through N=1024,
  forward/inverse, impulse, constant, alternating, complex tone, non-symmetric
  real, deterministic pseudo-random, distinct padded batches, and round trip;
- RTX 3070 benchmark: N=16 batch=1 warm median 1,543,870 ns / p90 1,566,825
  ns (10,363 samples/s); N=256 batch=4 warm median 1,400,079 ns / p90
  1,411,821 ns (731,386 samples/s). Both figures include upload, readback,
  and current per-call pipeline setup; checksum validation happens outside the
  measured loop;
- device: NVIDIA GeForce RTX 3070, vendor 0x10de, device 0x2488, Vulkan
  physical-device 1.4.329, NVIDIA proprietary driver 596.36; loader 1.4.350.
- focused Go compiler/interpreter/typechecker checks and Mathematics corpus
  with `FFT(...)` capitalization.

## Identity and limitations

The M4 host-side FFT deception remains historical and retired; this route does
not calculate any transform in C or Go. Shader id 52 has source identity
`55e0a80af7153a7fdf61aeafbe943b982817b09d02ef83299879d27a4da77c2d` and
module identity `d43718d266afa778df6bde761f6653853d23ad0aeebfdb2a03215fbeb4457ed8`.
Shader id 53 has source identity
`90bc28b601ffa331c8587004854f790eceee4d4475d39ca95347dc4b041f54ab` and
module identity `455759aff441ddbff4d34804bf8970c302e8a4f7ef434857f7d2b564f66fc6bc`.
The plan identity comprises N, direction, batch, effective stride, pass count,
fixed native-sin/cos twiddles, final inverse normalization, and the fixed
ProductionVulkan14/vulkan1.3 target. The current implementation intentionally
uses per-call temporary Vulkan objects; bounded persistent cache reuse is the
next performance improvement, not a condition hidden by the benchmark.

## Deferred experiments

| Experiment | Claimed advantage | Assumption | Smallest falsification | Numerical risk | Prerequisite |
|---|---|---|---|---|---|
| radix-4/8 complete plans | fewer dispatches | legal whole-plan decomposition wins | fixed N=1K/4K comparison | ordering/rounding | this radix-2 baseline |
| twiddle tables/recurrence/CORDIC | lower transcendental cost | twiddle generation dominates | fixed N=4K accuracy/timing corpus | phase drift | native sin/cos baseline |
| shared/subgroup FFT | fewer global transfers | one-workgroup constraints fit workload | fixed small-N comparison | synchronization | radix-2 baseline |
