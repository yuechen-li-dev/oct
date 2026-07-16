# Prometheus M40b device-resident cooperative inference path

## Outcome

M40b is complete as an **experimental selector candidate**. The bounded path is
correct, validation-clean, lifecycle-safe, and materially faster on the GPU,
but it does not replace A2x4 or enter the default production selector.

The implemented path is:

```text
persistent packed B + host-packed/uploaded A, or resident packed A+B
    -> cooperative SGEMM
    -> one device-local strided FP32 C
    -> compute-write/compute-read barrier
    -> M39b fused or staged stable row softmax
    -> one final correctness readback
```

No generic graph executor, public Vulkan-handle API, convolution,
normalization, model import, Direct3D, PTX, CUDA interop, or distributed work
was introduced.

## Audit of the previous ownership assumptions

The M40a audit executor in `reactor_vulkan_sgemm.c` starts with host FP32 A and
B. Its benchmark owner converts and packs both matrices, writes mapped upload
buffers, copies them to device-local buffers for each preparation policy,
dispatches SGEMM, copies C into a mapped readback buffer, waits on a CPU-visible
fence, and performs the CPU correctness comparison. Even its persistent-A+B
mode retained the final output copy and CPU synchronization. That was the
reason the 2.53x 1024-cube kernel win over A2x4 was mostly hidden.

The M39b reduction reactor already had the right GPU execution structure but
the ordinary entry point began from a host pointer. Each persistent ring slot
owned input, output, scratch, row-max, and row-sum buffers, descriptor sets, a
command buffer, and a fence. Input was uploaded into the slot; the fused path
or staged path wrote a device-local output; only the final result was copied
back. Logical failure, physical slot state, quarantine, generation, and reap
were already distinct.

The required changes were therefore narrow:

- let a reduction dispatch bind a producer-owned device-local input view;
- carry logical width independently from padded physical row stride;
- keep producer storage alive through consumer completion;
- place the SGEMM and reduction commands under one bounded execution owner;
- move the only readback after softmax.

## Internal handoff contract

`prom_device_buffer_view` is internal to the Prometheus Vulkan reactor. It
contains the `VkBuffer`, byte offset/range, element type, logical rows and
columns, physical row stride, row-major layout identity, producer access,
required consumer access, owning `VkDevice`, lifetime ID, slot ID, and slot
generation. Validation rejects null/undersized/misaligned views, wrong element
or layout identity, shape mismatch, incorrect access transition, and
cross-device use.

The view has no host pointer, performs no copy or migration, and is not exposed
through an Oct API. Shape is never inferred from byte count alone.

## Ownership and lifecycle model

The smallest compatible model is a **bounded shared execution slot**. The
existing reduction-family persistent ring owns the entire composed operation:

- a slot owns host-A upload storage when needed, one device-local A, exactly
  one device-local C, the softmax output/readback, reduction temporaries, two
  reusable command buffers, a fence, and the bounded two-submit semaphore;
- family state owns persistent packed B and the optional resident packed A;
- pipelines, descriptor layout/pool, query pool, and command pool are family
  state, not per-dispatch objects;
- the slot cannot return to `PHYSICAL_READY` until the final consumer and
  readback complete;
- uncertain submitted work is quarantined; replacement waits and reaps before
  persistent storage can change;
- B and resident-A generations are strictly increasing and shape/kernel
  identities must match the requested padding plan.

There is one physical owner and no reference-counted object graph. The
fault-injection test submits SGEMM+softmax, forces completion observation to
fail, proves the slot is not recyclable, replaces B only after the fence is
reaped, and then executes with the fresh generation. It ends with one
quarantine, at least one reap, no quarantined slots, and no stale use.

## Command and synchronization plans

The preferred one-command-buffer trace is deterministic:

1. reset eight slot-owned timestamp queries;
2. for host A, host-write to transfer-read barrier, copy A upload to
   device-local A, then transfer-write to compute-read barrier;
3. timestamp SGEMM start, bind the exact SGEMM pipeline/descriptors, push
   M/N/K, dispatch, timestamp SGEMM end;
4. expose C through the device view;
5. issue a C-only `COMPUTE_SHADER/SHADER_WRITE` to
   `COMPUTE_SHADER/SHADER_READ` buffer barrier with both queue-family indices
   `VK_QUEUE_FAMILY_IGNORED`;
6. timestamp softmax start, bind/dispatch the M39b fused plan for logical width
   at most 1024 or its staged plan above 1024, including existing temporary
   barriers, then timestamp softmax end;
7. issue `COMPUTE_SHADER/SHADER_WRITE` to `TRANSFER/TRANSFER_READ` for the final
   softmax output, copy it to the mapped readback buffer, and make the transfer
   visible to host access;
8. timestamp readback completion and signal the slot fence.

The alternative plan records producer and consumer into two slot-owned command
buffers. Two `VkSubmitInfo` records are passed in one bounded `vkQueueSubmit`;
the producer signals a binary semaphore and the consumer waits at compute
stage. It uses the same explicit C barrier and no queue ownership transfer.

Across the 14 cooperative workloads, two-submit GPU time was slower for 13 of
14 rows; its median ratio was 1.033x and median CPU submission ratio was
1.448x. At 128x1024x1024, one-submit was 190.240 us combined GPU and 24.1 us
CPU submission versus 203.872 us and 44.1 us for two-submit. One command buffer
is therefore the preferred plan. The bounded two-submit path remains as an
explicit lifecycle comparison, not a scheduler.

## Padding and logical-width softmax

M40b uses exactly one tail policy: zero padding. M, N, and K are rounded up to
16 with overflow-safe arithmetic. Packed A and B are zero-filled outside their
logical extents. Cooperative SGEMM writes padded row-major C, while the device
view reports logical M/N and the padded N row stride. Fused and normalize
softmax shaders now read `InputRowStride` but dispatch and normalize only
`ElementsPerRow`, so padded columns cannot enter the denominator or escape in
the output.

Zero logical dimensions and padded byte-size overflow reject before resource
or pipeline work. Measured arithmetic padding overhead for the awkward corpus
was 1.59% for 127x1001x1023, 9.48% for 257x769x1025, and 2.11% for
511x1281x2049. All three passed final softmax correctness.

## Precision and correctness contract

The cooperative and conventional-FP16 paths consume f16-rounded packed A/B,
accumulate in f32, and write f32. A2x4 consumes and accumulates f32. Comparisons
against conventional FP16 are same-logical-input comparisons; comparisons
against A2x4 are product comparisons and are not described as bit-equivalent.

The CPU oracle performs SGEMM under the matching input contract and stable
row-wise softmax. Acceptance is finite and nonnegative output, row sum within
`2e-4` of one, and each element satisfying absolute error at most `2e-5` or
relative error at most `2e-4`. A failure reports row, column, expected, actual,
absolute/relative error, logical and padded shape, the cooperative shader hash,
and reduction replay ID. No benchmark row produced NaN, Inf, a negative
probability, row-sum failure, or element mismatch.

## Corpus and measured evidence

Notation is M x N x K. The bounded corpus contains six aligned inference
shapes, five additional diffusion-like widths, and three awkward shapes. Each
shape runs cooperative, A2x4, and conventional FP16, for 42 deterministic
records. The following medians are from the RTX 3070 artifact. `GPU` is the
resident SGEMM+softmax interval; `host E2E` is persistent-B/new-host-A with the
one final readback. Speedups use the already-device-resident combined interval.

| M x N x K | Padded | Coop SGEMM (us) | Coop GPU (us) | vs A2x4 | vs FP16 | Coop host E2E (ms) | Host vs A2x4 | Coop device E2E (ms) | Readback (ms) |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 128x320x1024 | same | 127.0 | 127.0 | 4.31x | 2.90x | 1.470 | 1.18x | 1.031 | 0.512 |
| 128x640x1024 | same | 174.1 | 179.6 | 3.75x | 3.96x | 2.032 | 1.15x | 1.607 | 1.046 |
| 128x768x1024 | same | 178.2 | 190.0 | 3.56x | 4.60x | 2.171 | 1.15x | 1.782 | 1.208 |
| 128x1024x1024 | same | 179.2 | 190.5 | 3.57x | 5.97x | 2.731 | 1.02x | 2.422 | 1.713 |
| 128x1280x1024 | same | 181.2 | 215.1 | 3.28x | 6.52x | 3.041 | 1.06x | 2.635 | 2.024 |
| 128x2048x1024 | same | 342.0 | 381.1 | 2.07x | 5.81x | 4.708 | 1.06x | 4.246 | 3.457 |
| 256x1024x1024 | same | 345.1 | 363.3 | 2.04x | 6.05x | 4.836 | 1.04x | 4.484 | 3.306 |
| 512x1024x1024 | same | 523.3 | 548.9 | 2.78x | 7.93x | 10.855 | 0.98x | 9.425 | 8.343 |
| 1024x1024x1024 | same | 1030.1 | 1077.4 | 2.42x | 8.03x | 20.951 | 0.99x | 17.677 | 16.249 |
| 256x4096x1024 | same | 1025.0 | 1112.9 | 2.37x | 10.61x | 18.360 | 1.07x | 17.918 | 15.974 |
| 1024x4096x1024 | same | 9575.4 | 11419.2 | 0.98x | 3.30x | 84.749 | 1.19x | 78.385 | 66.895 |
| 127x1001x1023 | 128x1008x1024 | 126.0 | 141.6 | 4.85x | 4.94x | 2.433 | 1.05x | 1.808 | 1.519 |
| 257x769x1025 | 272x784x1040 | 343.0 | 319.0 | 2.16x | 3.41x | 3.726 | 1.01x | 2.983 | 2.352 |
| 511x1281x2049 | 512x1296x2064 | 1387.5 | 1467.9 | 1.95x | 5.23x | 16.033 | 0.92x | 12.100 | 10.531 |

The cooperative combined GPU interval beat A2x4 on 13 of 14 shapes by
1.95x-4.85x; the 1024x4096x1024 repeat was 0.98x and is a clear large-shape
rollback signal. It beat conventional FP16 on all 14 by 2.90x-10.61x.
Device-A+B end-to-end beat A2x4 on 12 of 14 by up to 1.41x and FP16 on all 14
by 1.24x-1.65x. Persistent-B/new-A end-to-end beat A2x4 on 11 of 14, ranging
from 0.92x to 1.19x, and beat conventional FP16 on all shapes by 1.17x-1.69x.
Readback is still the largest application cost for the large rows and is
reported separately.

The first 128x1024x1024 cooperative invocation was 3.754 ms end-to-end;
one-time B conversion/upload was 2.507/0.653 ms and resident-A
conversion/upload was 0.357/0.325 ms. Warm resident 10-operation and
100-operation median end-to-end times were 2.396 ms and 2.088 ms. No
per-dispatch pipeline, descriptor-layout, or Vulkan memory allocation occurs
after capacity is established.

## Reuse, identity, and authority

The ring depth is two. Buffers grow geometrically and are retained per slot;
packed B and resident A are family-owned. Descriptor sets, command buffers,
query pool, reduction pipelines, and each of the three comparison SGEMM
pipelines are reused. The artifact records buffer grows/reuses, retained bytes,
descriptor updates, pipeline count, command-buffer reuse, both replay IDs, and
every logical/padded workload identity. Each row additionally records complete
machine-readable host-one, resident-one, and resident-two command traces:
binds, push constants, dispatches, barriers, copies, timestamp placements,
access masks, queue-family identities, and replay IDs. The workspace checker
rejects a second C, an intermediate host copy, missing required operations, or
an accidental queue-family transfer.

Identity evidence:

- cooperative source SHA-256:
  `872ef19abeb1d9a0f894fa238bd5e6b8ec1d9b8762a3e47ef5c3756aecb0b3b4`;
- cooperative SPIR-V SHA-256:
  `247e410eb526f25c2276d127a732bb4def0c7949bca0ad0fdc5434ea95d17fea`;
- compiler: Vulkan SDK DXC 1.9.0.5347 / 1.10.5347-fe261573,
  `-fspv-target-env=vulkan1.3`;
- tuple: subgroup scope, 16x16x16, f16 A/B, f32 accumulator/result,
  subgroup size 32;
- M40b artifact SHA-256:
  `1e767199715e0bac007fad93ee232b2a7361e8be06434d9be87f5a3756323c63`.

The exact cooperative source/header remains under the experimental SDSL-V
tree. It is imported only by the bounded internal composition/audit path, is
not present in the production shader registry, has no production numeric ID,
and remains selector-ineligible by default. Production reduction shader IDs
19 and 20 retain production authority; their strided-input module hashes are
recorded in the M39b report.

## Experimental selector predicate and classification

The deterministic predicate reports a rejection reason for disabled state,
capability/compiler-feature mismatch, tuple mismatch, precision rejection,
shape envelope, unsupported padding, absent persistent B, missing residency,
and rollback. Eligibility requires the executable cooperative capability,
16x16x16 tuple, shader f16 and Vulkan memory model, a precision policy allowing
f16-rounded inputs, nonzero in-envelope M/N/K, supported padding, persistent B,
device-resident composition, and no rollback. Even an eligible result sets
`selected=0`: the default production policy is unchanged.

The bounded runtime envelope is M <= 1024, N <= 1,048,576,
M*N <= 16,777,216, nonzero overflow-safe padded sizes, an executable selected
tuple on the owning device/subgroup, matching persistent generations, and
available Vulkan buffer capacity. The measured evidence envelope is the 14
shapes above, up to 1024x4096x1024 aligned and 511x1281x2049 awkward.

Classification is **2. Experimental selector candidate**. The combined GPU
question is strongly positive for the stated envelope but not stable enough for
a global default: the repeat corpus includes one 1024x4096x1024 A2x4 win and
persistent-B/new-host-A does not beat A2x4 across the whole corpus. Padding is
bounded and fully resident end-to-end wins are credible. The path also still
assumes reduced-precision input permission and an exact NVIDIA-tested tuple.
Therefore no source move, stable production ID, registry entry, or production
selector promotion occurs.

## Exact next workload

M42 completed the real-producer workload as a full one-head forward attention
operator rather than another synthetic SGEMM-softmax proof. Three Vulkan SGEMMs
produce Q/K/V, an explicit GPU layout stage creates K-transpose, cooperative
QK-transpose feeds M39b softmax, and GPU-packed P and V feed cooperative P-by-V.
The normal path has no intermediate readback and excludes its one final Output
readback from the central GPU interval.

On the 128/1024/128 attention convention, cooperative full attention measured
418.0 us versus 1.478 ms for A2x4 and 586.4 us for conventional FP16. The
six-shape corpus beat A2x4 on all rows, but tiny attention was 0.98x versus
conventional FP16. M40b's global SGEMM selector and experimental authority are
unchanged. Complete ownership, precision, lifecycle, and performance evidence
is in `PROMETHEUS_M42_DEVICE_RESIDENT_ATTENTION_OPERATOR.md`.
