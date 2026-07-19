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

## Still open

The transactional `layers.0 -> layers.29` rebind and resident ping/pong chain
are implemented. The full RTX chain matches the independently generated CUDA
FP32 `layers.29` authority at `1.02005e-05` relative L2, under the unchanged
`5e-5` threshold. The authority was regenerated independently with the same
final joint SHA-256.

Stage-local MainTransformer capture remains to be expanded from the existing
representative audit schedule into the full requested M2D audit profile.

## Identities

- M2C baseline lock: `f67b31bdd1e54945d9dd66f6371f0f3ca8e99595b702aa9a3da310529f9ffa6a`
- M2D current lock: `71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e`

The next implementation seam is deliberately bounded: reuse the existing
transactional weight-rebind protocol for the 13-role MainTransformer package,
then retain the resulting joint output on device for the next descriptor.  No
host activation copy or activation BF16 conversion is permitted.
