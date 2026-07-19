# SDSL-V compiled model manifest PoC

## Result

**MEANINGFUL PROGRESSION / IN PROGRESS.** The PoC uses the established Oct
package-manifest syntax (`package Manifest`, records, and a returned manifest
record) and the existing data-only Octagon convention. `CompiledModel()` is a
second compiler-parsed declaration in the same `manifest.oct`, not a JSON DSL.

`tools/compiled_model_lock` resolves the real Z-Image Turbo two-block subset to
the canonical `lock-tagon.octagon`. It validates the closed names, aggregates,
FP32 ABI, and order, then writes deterministic Octagon. Re-running produces
byte-identical output; `-check` detects a missing or changed lock.

## Resolved model

- family: `ZImageTurbo.NoiseRefiner`;
- instances: `NoiseRefiner0` / `noise_refiner.0` and `NoiseRefiner1` /
  `noise_refiner.1`;
- aggregates: `a1ba5268…ca5d5e` and `80c0cd75…4bc7c8`;
- successor edge: block 0 to block 1 with resident FP32 `ModelEmbedding`,
  atomic weight rebind, no BF16 boundary cast, and no host activation bounce.

The native rebind request now accepts only the lock identity and a model-local
block ID plus the validated payload bundle. It resolves the immutable block-1
entry internally, so callers cannot supply a family tag or cache aggregate.
The resulting RTX chain completed block-1 final audit at relative L2
`1.27829e-6` against its canonical FP32 authority. Context-refiner topology is
explicitly not included.

The current C descriptor table is a bounded projection of the resolved lock;
the remaining manifest milestone work is to generate that projection directly
from the lock and add the full negative linker/runtime corpus.
