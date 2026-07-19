# EVT-2 M2C — MainTransformer representative assembly

## Status

**Meaningful progression; not M2C closeout.**

This change closes the source, package, canonical-authority, semantic-space,
and shader-portfolio work for the single representative `layers.0` block. It
does not claim a compiled MainTransformer execution, static audit, warm-path
measurement, or a resident NoiseRefiner/ContextRefiner handoff. Those claims
remain blocked by the current native owner topology described below.

## Frozen authority

- Source: `Tongyi-MAI/Z-Image`, revision
  `26f23eda626ffadda020b04ff79488e1d72004cd`,
  `src/zimage/transformer.py`.
- Model revision: `f332072aa78be7aecdf3ee76d5c247082da564a6`.
- Checkpoint: `sha256:2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6`.
- M2B lock baseline (not replaced here):
  `4b4aeb0474779325d60a8725d1094796d9c2271b7669f596b11815e7a7a6970b`.

The source constructs all thirty `self.layers` entries with the same
modulation-enabled `ZImageTransformerBlock` class. The checkpoint inventory
confirms the same thirteen role/shape/dtype entries for `layers.0` through
`layers.29`; that is structural equivalence, not permission to substitute
weights. `layers.0` is the selected representative and every future layer
remains a separately aggregate-identified package.

## Representative package and oracle

`internal/prometheus/zimage/main_transformer.go` names exactly one closed
package, with aggregate identity
`48e987811885741ae5f1bf16b28db33ca7f23e09f1e99c1c2fe3d81bdd1caeb6` and
361,820,672 cached FP16 bytes. The cache tool reads only the pinned
checkpoint and writes a conversion/finiteness inventory beside the local
cache; neither payload is versioned in this repository.

The independent local FP32 canonical laboratory composes the accepted M2B
final streams as `Joint = Concat(Image, Context)`: 1024 image tokens at
offset zero followed by 32 context tokens at offset 1024. It records the
full joint stage sequence and selected image/context attention rows. Two
identical runs produced identical files. The final identities are:

| boundary | SHA-256 |
| --- | --- |
| image output | `9c8fee6d4208500b585e118150056b05c984bc14374f3f47a875543d58868555` |
| context output | `0d4b5cf45c2eace2b8cce21124505961df5e05c4a3a469ac7259c2ce060a0173` |

All weights expand from immutable FP16 storage to FP32 at use. Activations,
RMSNorm, RoPE, attention logits/probabilities/accumulation, projections, and
residuals remain FP32. There is no activation FP16 narrowing, tolerance
widening, clamp, or saturation policy.

## Semantic and shader boundaries

The focused Oct experiment supplies compiled, zero-fallback contracts for
joint image-first composition, the two RoPE coordinate domains, and AdaLN
segment order. Its artifact sink currently has no compiled backend support,
so the JSON witness is explicitly an interpreted artifact-lane result rather
than compiled numerical proof.

Four new production SDSL-V kernels are registered:

- 40: joint Q/K RMSNorm + RoPE (24-byte push constants);
- 41: fixed 1056-token one-query/head attention;
- 42: 1056-token W1/W3 projection;
- 43: 1056-token SiLU gate.

The remaining operations reuse the reviewed physical kernels only through
separate MainTransformer bindings. The two new conformance-invalid fixtures
reject cross-stream key use and context-as-image RoPE coordinates with
`SDSL-V4123`. Current semantic spaces intentionally model vector-value
boundaries, not tensor axes.

## Native boundary and next blocker

`prom_reduction_runtime_state` currently owns one `prom_model_block_state` at
`state->model_block`. That makes NoiseRefiner, ContextRefiner, and a new
MainTransformer mutually exclusive resident owners. Feeding the representative
by reading either accepted M2B stream back through the host would violate the
requested device-to-device handoff; it was not implemented or represented as
success.

The narrow next step is a multi-owner resident block registry with explicit
stream handles and ownership/liveness checks. It must allow the two accepted
M2B owners and one MainTransformer owner to coexist, bind image/context
buffers directly into the 1056-token joint ingress, and preserve the fixed
audit arena/no-prefix-replay rules. Only then should the compiled-model lock
transition from the M2B baseline and only then can an RTX execution, ten warm
runs, fault matrix, static audit, and scaling evidence be reported.

## Evidence and commands

The committed review artifacts are under
`internal/prometheus/DevelopmentReport/artifacts/Evt2M2c/`. Recreate local
payload-derived evidence with:

```powershell
$env:OCT_EVT2_CACHE = "$env:LOCALAPPDATA\Oct\evt2-z-image-turbo"
go run ./tools/zimage_main_transformer_cache -source C:\Users\yuech\ComfyUI\models\diffusion_models\z_image_turbo_bf16.safetensors -cache-root $env:OCT_EVT2_CACHE
python tools/zimage_main_transformer_canonical.py --cache-root $env:OCT_EVT2_CACHE --oracle-root $env:OCT_EVT2_CACHE\oracle\f332072aa78be7aecdf3ee76d5c247082da564a6 --out $env:OCT_EVT2_CACHE\canonical\f332072aa78be7aecdf3ee76d5c247082da564a6\m2c-fp32-reference\layers.0
go run ./tools/evt2_m2c_artifacts -cache-root $env:OCT_EVT2_CACHE
go run ./cmd/oct test Experiments/ZImageTurboMainTransformer0 --execution compiled
```

SDSL-V checks for all four new kernels pass. The native Windows build was
performed inside the configured Visual Studio developer environment. The
focused registry filters passed, including the generated-header/manifest
check, and the full Marionette run completed with **437 tests, 403 passed,
34 skipped, and 0 failed**. That build evidence validates the new production
asset records; it is not a substitute for the absent M2C runtime owner or
numerical execution.
