# EVT-2 M2A noise-refiner.1 assembly reuse

## Current result

**MEANINGFUL PROGRESSION / IN PROGRESS.** The source and tensor evidence establishes `noise_refiner.1`
as an **identical** instance of the modulated noise-refiner assembly; only its
immutable parameters differ. The pinned source constructs both entries in one
`noise_refiner` `ModuleList` from the same `ZImageTransformerBlock` class.
The exact checkpoint inventory confirms the same 13 roles, shapes, BF16
source dtype, cache orientations, and `361,820,672`-byte FP16 package.

The block-1 cache aggregate is
`80c0cd75f44cc434d9306c0fd9a8f02e48b593ecc254de01c1f8fcc29f4bc7c8`.
The focused 34-stage FP32 authority uses the accepted block-0 FP32 final
payload as its input (no BF16 internal-boundary cast); its final payload is
`9b133c9ed3772f782e1bd77ff5b89732dc28406eec2078f1692d1899e2eb39e7`.

## Proven reuse

- Topology, operation order, AdaLN, Q/K norm, RoPE, non-causal attention,
  QKV order, residual order, bias policy, widths, and epsilons are identical.
- Every block-1 weight is finite and FP16 storage has zero overflow. The small
  conversion underflows are recorded per tensor; they do not alter the
  accepted FP16-weight/FP32-arithmetic policy.
- The 13 production shader binaries, descriptor layout, push constants,
  semantic spaces, internal ABI, and 654,891,776-byte one-block memory plan
  are structurally reusable. Only immutable bindings and replay identity must
  change.

## M2A-R resident binding seam

The native owner now has the closed `ZImageTurbo.NoiseRefiner` family and the
two only legal parameter tags, `NoiseRefiner0` and `NoiseRefiner1`. A resident
binding reports its parameter-set aggregate, binding generation, output
generation, descriptor generation, lifecycle state, and replay identity.
`noise_refiner0_execute` and `noise_refiner1_execute` verify that tag before
they can run; neither spelling can relabel a handle.

`noise_refiner_rebind` validates all 13 declarations before state mutation,
uploads a complete candidate package into a separate device-local arena,
updates all weight descriptors in one bounded write batch only after certain
completion, then swaps ownership and increments the binding generation. A
pre-commit failure destroys only the candidate and retains block 0; uncertain
completion quarantines. Commit invalidates output, audit, and replay state.

The internal continuation copies the resident FP32 final `ModelEmbedding` to
the next block input buffer on-device, retains the resident FP32 timestep, and
starts at AdaLN. It does not invoke the BF16 ingress pipeline or touch host
activation memory.

## Validation completed so far

`go run ./tools/evt2_payload_check` validates both caches, both 34-stage
authorities, and the FP32 two-block boundary authority. The full Windows
native build and the clean RTX block-0 hardware witness continue to pass.
The generated block-1 oracle is independent from the native implementation;
The clean RTX 3070 chain witness passed: 13 block-0 uploads, 13 staged
block-1 uploads, binding generation `2`, descriptor generation `1`, with no
pipeline or descriptor-pool growth. The measured baseline was 46,350,800 ns
for rebind and 1,102,526,700 ns for the first block-1 resident execution.

This remains **in progress** because the new resident output audit accessor
and block-1 full-stage numerical closure have not yet been implemented. The
canonical final identity is present and validated independently, but it has
not been copied out through a bounded post-execution audit path for comparison.

## M2B direction

After M2A closes the safe weight-window and resident two-block path, M2B is
the context-refiner family. It can reuse the basic attention/FFN machinery but
must introduce its non-modulated block contract, text-token boundary,
different tensor package, and corresponding witnesses; it is excluded here.
