# EVT-2 M2D — MainTransformer numerical closure and streamed chain

## Current state

M2D absorbs M2C.  The former representative `layers.0` mismatch was localized
to the Q/K RoPE kernel: the pinned canonical implementation updates the even
lane before evaluating the odd lane, so the odd expression observes the updated
even lane.  The resident SDSL-V kernel now preserves that source order.

The real RTX representative lane is finite and numerically closed at the
accepted `5e-5` relative-L2 threshold.  Its final joint relative L2 is
`8.38066e-7`; image and context regions are `1.20583e-6` and `4.73599e-7`.
Ten warm executions have zero buffer allocation, weight-upload, pipeline, and
descriptor-pool churn.

The closed lock now resolves all 30 immutable `layers.0` through `layers.29`
parameter sets.  Every local deterministic FP16 cache was validated before the
lock expansion; the generated native descriptor contains the same 30 entries.
The native create path accepts only a descriptor-selected parameter set and
verifies the generated aggregate identity.

## Closeout evidence

The transactional `layers.0 -> layers.29` rebind and resident ping/pong chain
are implemented. The full RTX chain matches the independently generated CUDA
FP32 `layers.29` authority at `1.02005e-05` relative L2, under the unchanged
`5e-5` threshold. The authority was regenerated independently with the same
final joint SHA-256. The closeout run measured 24.630219 s host elapsed,
16.3835963 s summed layer execution, 1.2494448 s summed rebind/upload time,
and 10,492,799,488 streamed parameter bytes (8,397,969,632 B/s effective
rebind bandwidth). The lock-resolved allocation ceiling is exactly 643,587,076
bytes: 361,820,672 persistent, 234,579,972 reusable execution, and 47,186,432
audit bytes; the components sum exactly to the ceiling.

The layer-0 CUDA laboratory corpus was regenerated with all 34 persistent
stage payloads and selected attention witnesses. The native static replay
exercises the same resident input and final gated-residual output at
`8.38066e-7` relative L2. It is deliberately recorded as a final-boundary
replay, not as a per-stage native capture.

M2D is **not yet SUCCESS**: the lock-defined 29-entry MainTransformer static
profile has not been wired to per-stage native summary records, and therefore
the selected later-layer checkpoint audit policy and the requested 30-layer
lifecycle/fault matrix have not yet been demonstrated. The final authority and
model arithmetic remain closed and are not to be reopened.

## Identities

- M2C baseline lock: `f67b31bdd1e54945d9dd66f6371f0f3ca8e99595b702aa9a3da310529f9ffa6a`
- M2D current lock: `71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e`

The next implementation seam is deliberately bounded: reuse the existing
transactional weight-rebind protocol for the 13-role MainTransformer package,
then retain the resulting joint output on device for the next descriptor.  No
host activation copy or activation BF16 conversion is permitted.
