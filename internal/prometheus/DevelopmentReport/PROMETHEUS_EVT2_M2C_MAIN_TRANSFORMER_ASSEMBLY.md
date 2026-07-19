# EVT-2 M2C — MainTransformer representative assembly

## Status

**Meaningful progression; not M2C closeout.**

This change preserves the source, package, canonical-authority,
semantic-space, shader-portfolio, session-ownership, lock-transition, and
device-to-device joint-composition work for the single representative
`layers.0` block, and adds the first closed native MainTransformer owner and
execution recorder. It does not claim representative RTX numerical closure,
MainTransformer static-audit closure, ten-run warm zero-churn closure, or the
complete retained-refiner-chain comparison. Those claims remain open and must
be completed before M2C can become `SUCCESS`.

## Frozen authority

- Source: `Tongyi-MAI/Z-Image`, revision
  `26f23eda626ffadda020b04ff79488e1d72004cd`,
  `src/zimage/transformer.py`.
- Model revision: `f332072aa78be7aecdf3ee76d5c247082da564a6`.
- Checkpoint: `sha256:2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6`.
- Current compiled-model lock:
  `f67b31bdd1e54945d9dd66f6371f0f3ca8e99595b702aa9a3da310529f9ffa6a`.

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

The previous single `state->model_block` ownership boundary has been split by
the compiled-model session path. The session now owns fixed lock-derived
resident slots:

- `PreparedImage = 1`;
- `PreparedContext = 2`;
- `JointWorking = 3`.

`JointWorking` is physically composed on device in the exact `Image | Context`
order with the accepted byte boundary:

- image bytes: `15,728,640`;
- context bytes: `491,520`;
- joint bytes: `16,220,160`;
- image byte offset: `0`;
- context byte offset: `15,728,640`.

This pass adds a distinct closed owner/program path for
`ZImageTurbo.MainTransformer`, `MainTransformer0 / layers.0`. Creation resolves
the family, parameter set, tensor-role inventory, shader portfolio, stream
roles, token counts, joint extent, and memory plan from the generated lock
descriptor. The execution request accepts only the compiled-model session, lock
identity, model-local block id, validated `layers.0` payload bundle, timestep
conditioning payload, and execution options. Wrong lock, wrong block id,
missing or stale joint/source generations, malformed payloads, and descriptor
drift are rejected before execution mutation. A final-output audit egress now
copies only the completed representative joint stream for oracle comparison; it
does not participate in the block-to-block resident path.

The clean retained-chain RTX lane now runs
`NoiseRefiner0 -> NoiseRefiner1 -> PreparedImage`,
`ContextRefiner0 -> ContextRefiner1 -> PreparedContext`,
device-composes `JointWorking`, executes `MainTransformer0 / layers.0`, reads
back the final joint output, and performs ten warm representative executions
with no buffer allocation, memory allocation, weight upload, pipeline creation,
or descriptor-pool growth. The output is finite, but numerical closure fails
against the deterministic FP32 joint oracle:

| region | relative L2 | Linf |
| --- | ---: | ---: |
| joint | `0.0234765` | `2.24572` |
| image | `0.0375236` | `2.24572` |
| context | `0.00248398` | `1.10574` |
| last image token | `0.0425776` | not recorded |
| first context token | `0.00202765` | not recorded |

The first joint mismatch is coordinate `0`. This is a MainTransformer
execution-arithmetic/stage-wiring seam, not a refiner-stream or session
ownership contradiction: the accepted clean-process M1F/M2A real lane still
passes with block-1 final relative L2 `1.27829e-6` and Linf `1.52588e-4`.

The MainTransformer static audit schedule, one-batch stage-local readback, and
full lifecycle fault corpus must still be completed before M2C can close. No
host activation bounce, canonical tensor substitution, or tolerance widening
has been claimed.

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

SDSL-V checks for all four joint kernels pass. The native Windows build was
performed inside the configured Visual Studio developer environment. The
generated lock check passes for
`f67b31bdd1e54945d9dd66f6371f0f3ca8e99595b702aa9a3da310529f9ffa6a`, and the
full default Marionette run completed with **440 tests, 405 passed, 35 skipped,
and 0 failed**. The opt-in M2C real lane executed on the local RTX path and
proved finite output plus zero warm churn, but failed the final joint numerical
assertion with the metrics above. That evidence validates the closed owner,
retained stream ingestion, final-output audit egress, and warm reuse; it is not
a substitute for the still-open generated static audit or numerical closure.
