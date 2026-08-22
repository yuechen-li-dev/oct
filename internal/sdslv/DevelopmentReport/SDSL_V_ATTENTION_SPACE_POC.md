# SDSL-V attention-space proof of concept

## Decision

- Convergence outcome: **SUCCESS**
- Milestone state: **COMPLETE**
- Attention-space verdict: **USEFUL WITH BOUNDED EXTENSION**
- EVT-2 state: **READY FOR M1 WITH ATTENTION SPACES**

The practical question is answered yes. Exact nominal vector-value spaces reject
real Z-Image Q/K/V, RoPE-state, score/probability, coordinate-domain, and
residual mistakes even when every physical value is a `float4`. The annotations
erase completely: the semantic and annotation-erased controls produce
byte-identical SPIR-V.

This is not a claim that learned embeddings are physical coordinates. `@space`
means that a value is represented in one declared semantic basis or domain and
may cross into another only through an explicit typed function.

## Scope and authority

The fixture represents Z-Image-Turbo `noise_refiner.0` at model revision
`f332072aa78be7aecdf3ee76d5c247082da564a6` and source revision
`26f23eda626ffadda020b04ff79488e1d72004cd`. Its declarations retain width 3840,
30 heads, head width 128, FFN width 10240, RoPE partition `[32,48,48]`, and theta
256. The executable arithmetic uses one `float4` slice so this PoC does not
become the M1 GPU block.

No Oct source or semantics, Z-Image runtime block, numerical RoPE kernel,
attention-axis type system, tensor algebra, metaprogramming, importer,
scheduler, VAE, or performance path was added.

## Audit of the existing implementation

The audited path is grammar/parser (`internal/sdslv/parse`), AST
(`internal/sdslv/ast`), validation and graphics rules
(`internal/sdslv/validate`), VD-MIR (`internal/sdslv/vdmir`), lowering
(`internal/sdslv/lower`), HLSL emission (`internal/sdslv/emit/hlsl`), toolchain
SPIR-V generation, the canonical graphics conformance corpus, and the graphics
specification/reconciliation documents.

Before this PoC, the live behavior was:

| Question | Audited behavior |
|---|---|
| Naming | `type Alias = Base @space(dotted.path)`; the parser stores the complete path string. The grouped-declaration follow-up admits reserved tokens as segments only inside `@space(...)`, allowing the exact expanded `zimage.attention.score` identity. |
| Attachment | Direct annotations are valid only on named aliases. Resolved bases were and remain only `float2`, `float3`, or `float4`; arbitrary scalars and records cannot be annotated directly. |
| Identity | Compatibility is the exact pair `(resolved physical base, nominal space string)`. Separate aliases with the same pair are compatible. Different strings are incompatible. |
| Aliases | Alias resolution recursively preserves the established target space. There is no implicit spaced/unspaced conversion. |
| Establishment | A target-typed primitive vector constructor establishes the expected spaced alias. A function return type is the canonical transformation boundary. No automatic object/world/view/clip transform exists. |
| Preservation | Direct identity return, assignment to the same type, fields, parameters, returns, and element access preserve the typed contract. Scalar component extraction is unspaced. Plain-vector intrinsics reject spaced vectors instead of erasing them. |
| Equality requirements | Assignment, returns, function calls, record/payload construction, stream linkage, and fixed-shape element compatibility use exact space equality. |
| Records and payload enums | Fields may reference a spaced alias and retain its semantic type. Direct field annotations remain invalid. |
| Arrays and tensors | `array<Alias>` and `ndarray<Alias, [shape]>` retain the spaced element type. Shapes and axes carry no space. |
| VD-MIR | `vdmir.Type.Space` retains compile-time evidence, including nested element types. |
| HLSL/SPIR-V | HLSL physical types ignore `Space`; alias comments retain useful source evidence. DXC sees ordinary arithmetic and layouts. |
| Diagnostics | Graphics mismatches used established general type codes. Function mismatches did not consistently expose resolved actual/expected space names. |

The important pre-PoC restriction was in `validateCoordinateAlias`: only the
closed graphics roots `object|world|view|clip` and roles
`position|normal|vector` were admitted. The parser, AST, compatibility checker,
VD-MIR, lowering, and backend were already general enough for an exact dotted
nominal string.

## Bounded extension

The validator now admits any dotted non-graphics nominal name on the existing
float-vector alias form. This adds no parameters, generic spaces, inheritance,
relation declarations, operator overloading, or runtime behavior. The four
graphics roots retain their closed vocabulary and `clip.position` special rule,
so existing graphics semantics do not loosen.

`SDSL-V4123` is added at ordinary function argument boundaries when a
non-graphics semantic space differs. It reports the function, expected space,
actual space, source span, and the explicit establishment action. Existing
graphics diagnostic codes remain stable.

No syntax was added. In particular, there is no `space`, `role`, or `pairing`
declaration.

## Z-Image vocabulary

The mandatory production vocabulary is:

- `zimage.noise_refiner.embedding`
- `zimage.attention.query_head`
- `zimage.attention.key_head`
- `zimage.attention.value_head`
- `zimage.attention.positioned_query_head`
- `zimage.attention.positioned_key_head`
- `zimage.attention.score`
- `zimage.attention.probability`

The PoC additionally uses `zimage.attention.output`,
`zimage.position.frame`, `zimage.position.row`, and
`zimage.position.column`. Negative contracts use `qwen.hidden`,
`qwen.position.text_token`, a foreign value-head space, and an incompatible
position convention.

The grouped-declaration follow-up uses `Score` and deterministically expands it
to `zimage.attention.score`; explicit `@space(zimage.attention.score)` is also
accepted even though lowercase `score` is a judgment token elsewhere.

## Legal transformations and pairing

| Operation | Input spaces | Output space |
|---|---|---|
| `ProjectQuery` | model embedding | query head |
| `ProjectKey` | model embedding | key head |
| `ProjectValue` | model embedding | value head |
| `NormalizeQuery` | query head | query head |
| `NormalizeKey` | key head | key head |
| `ApplyQueryRoPE` | query head + frame/row/column | positioned query head |
| `ApplyKeyRoPE` | key head + frame/row/column | positioned key head |
| `Score` | positioned query head + positioned key head | attention score |
| `NormalizeScores` | attention score | attention probability |
| `AggregateValues` | attention probability + value head | attention output |
| `OutputProject` | attention output | model embedding |
| `AddResidual` | model embedding + model embedding | model embedding |

Pairing uses design 3 from the milestone choices: the closed `Score` function
signature is the narrow declaration-level contraction rule. It accepts exactly
positioned Q and positioned K and returns a score. It rejects Q×V without a
general pairing relation or operator system. A same-space Q/K design would lose
the useful role distinction; a generalized `pairing` language is unnecessary.

## RoPE result

The actual three-axis partition `[32,48,48]` and theta 256 are declared in the
real-shaped fixture. Query and key start in distinct unpositioned spaces.
`ApplyQueryRoPE` and `ApplyKeyRoPE` accept only the appropriate unpositioned head
plus frame, row, and column coordinate spaces, and return distinct positioned
head spaces.

Consequences proven by invalid fixtures:

- scoring an unpositioned query is rejected;
- calling `ApplyQueryRoPE` on a positioned query is rejected as double RoPE;
- a key using another positional convention is rejected by `Score`;
- a text-token coordinate is rejected where the frame coordinate is required.

The functions use only a zero-valued arithmetic witness. They do not implement
numerical RoPE.

## Token-domain and tensor result

Vector-value domain checking is useful now. In particular, the PoC distinguishes
image-axis coordinate values from a text-token coordinate. Spaced aliases also
work as record fields, payload-enum fields, function parameters/returns, locals,
array elements, and `ndarray` elements. The valid fixture declares the actual
equivalent of `ndarray<QueryHead, [1024,30]>`.

`@space` does not attach identity to an ndarray axis. It therefore cannot prove
that a tensor's first axis is QueryToken rather than KeyToken, nor that an
attention-probability key axis matches a value tensor's token axis. Encoding
those facts as whole-vector names would be misleading, so the PoC does not fake
them.

Classification:

- vector-value spaces: supported now;
- tensor element spaces: supported now;
- image versus text coordinate values: caught now;
- query/key token axes and cross-tensor token-domain alignment: require future
  tensor-axis/index semantics.

This gap does not block M1.

## Invalid corpus and diagnostics

Ten permanent invalid conformance fixtures reject with `SDSL-V4123`:

1. Q contracted with V instead of K.
2. K supplied where V is required.
3. Unpositioned Q paired with positioned K.
4. RoPE applied twice.
5. Incompatible positioned-key convention.
6. Raw Qwen hidden state used as a Z-Image residual.
7. Attention probability treated as an attention score.
8. Attention score aggregated before normalization.
9. Attention head added as a model residual.
10. Text-token coordinate supplied to image RoPE.

Representative diagnostics are:

```text
function Score requires space `zimage.attention.positioned_key_head`, got `zimage.attention.value_head`; establish `zimage.attention.positioned_key_head` with a function returning that space
```

```text
function ApplyQueryRoPE requires space `zimage.attention.query_head`, got `zimage.attention.positioned_query_head`; establish `zimage.attention.query_head` with a function returning that space
```

```text
function NormalizeScores requires space `zimage.attention.score_value`, got `zimage.attention.probability`; establish `zimage.attention.score_value` with a function returning that space
```

Each diagnostic is anchored to the offending argument range. The complete
messages, codes, and line/column locations are recorded in the JSON artifact.

## Valid and executable proof

`AttentionSpacePoc.sdslvvalid` is a real-shaped compute fixture containing the
frozen dimensions, fused QKV record, Q/K normalization, three-axis RoPE
establishment, positioned scoring, stable softmax arithmetic, value
aggregation, output projection, and model-space residual. It also covers record
fields, a payload-enum field, fixed-shape ndarray elements, function boundaries,
and temporary variables.

`AttentionSpaceGpuProof.sdslvtest` executes one synthetic float4 flow through
the native Vulkan SDSL-V test host. Stable case
`sdslv-9e17968fc94dcd6e3862968a` passed on the local RTX path. This proves the
same typed transitions remain ordinary executable arithmetic; it is not a
30-head or numerical-attention result.

## Backend erasure and identity

The semantic fixture and a mechanically annotation-erased control were compiled
with canonical SDSL-V HLSL generation, DXC SPIR-V generation, and `spirv-val`.

| Evidence | Semantic | Erased control |
|---|---|---|
| HLSL SHA-256 | `798682fd39f37f97134ac17282bffe34658f6eb561c2c5cd423c6b300a144c98` | `3868b53b2a84c6084975b5a0a0e5580a5744f111c35ef7962d37d9c76a8eddad` |
| SPIR-V SHA-256 | `bd5df6e5c12b0b4cdd18220ab997509d83e8abae5d206c7a949e096a0d914743` | `bd5df6e5c12b0b4cdd18220ab997509d83e8abae5d206c7a949e096a0d914743` |
| `spirv-val` | pass | pass |

The HLSL texts differ only in namespace/type comments and the inline-source
comment path. After removing non-runtime comments, they are identical. The
SPIR-V files are byte-identical, a stronger result than structural equivalence.

Both variants expose only:

- set 0 binding 0: readonly structured buffer of `f32`;
- set 0 binding 1: read/write structured buffer of `f32`;
- no push constants;
- unchanged `f32` buffer element layout.

There is no tag field, space ID, additional descriptor, storage allocation,
branch, or ABI change. VD-MIR retains `Type.Space`; HLSL physical type selection
does not. The machine artifact records source paths and source SHA-256 values,
so semantic-source identity remains explicit alongside generated identities.

## Deterministic artifact

`tools/sdslv_attention_space_poc` validates all ten negative fixtures, compiles
both controls, requires `spirv-val`, compares runtime HLSL and SPIR-V, and emits
`artifacts/AttentionSpacePoc/attention_space_poc.json`.

Two consecutive final generations after the grouped-declaration follow-up
produced identical artifact SHA-256
`e86a9ccd6edffd836e587867935c10baec1d4dceb9759a1ad2be11fb766db036`.
The internal projection identity is
`3f1a04aa91c3b5ebfc9d114b74f5818902743c6381359c6e034dcf580a04c57b`,
computed with its own field empty to avoid a self-hash cycle.

## EVT-2 M1 integration decision

1. EVT-2 M1 should use semantic spaces in production SDSL-V source at the
   vector-value boundaries.
2. Mandatory spaces are model embedding, Q, K, V, positioned Q, positioned K,
   attention score, and attention probability. Attention output and the three
   coordinate spaces are recommended where those functions are explicit.
3. Useful checks now are Q/K/V role separation, RoPE state transitions,
   positional-convention separation, score/probability state, output projection,
   residual domain, and coordinate-value domain.
4. Query-token/key-token axes and probability/value token-axis agreement need a
   future tensor-axis/index system.
5. This adds a small amount of alias and transition-function surface but reduces
   M1 debugging complexity. No runtime or backend complexity is added.
6. It catches more than three realistic errors before runtime; the corpus proves
   ten, spanning seven distinct mistake classes.

The exact recommendation is: use the eight mandatory nominal spaces for M1,
keep pairing as the closed `Score` signature, use explicit Q/K RoPE functions,
and do not block M1 on axis typing. Do not introduce a role/pairing relation
language or tensor calculus in M1.

## Deferred work

- tensor-axis/token-domain identities, only if a later concrete kernel justifies
  an index-system design;
- scalar semantic states, if a real scalar-only contract cannot use the existing
  vector-value boundary;
- richer diagnostic establishment hints tied to a declared function index, if
  generic wording proves insufficient in production.

None of these is required for the bounded M1 recommendation.

## Validation

All closeout lanes passed on Windows:

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./internal/octxiliary/...`
- `go test ./pkg/octxiliary/...`
- `go test ./internal/cli`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go test ./tools/build_sidecars`
- `go run ./tools/prometheus_native_manifest -check`
- `go run ./tools/sdslv_workspace_check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check`
- canonical conformance verification, including every new valid/invalid fixture
  and the unchanged graphics-space corpus
- `go run ./cmd/oct sdslv test
  Examples/SDSL-V/AttentionSpacePoc/AttentionSpaceGpuProof.sdslvtest`
- semantic and erased HLSL/SPIR-V generation through DXC
- standalone `spirv-val` on both modules
- SPIR-V resource-interface disassembly inspection
- two consecutive `go run ./tools/sdslv_attention_space_poc` generations and
  exact output-hash comparison

The broad `internal/sdslv/test` lane also passed its existing native-host tests.
SPIR-V disassembly shows exactly descriptor set 0 bindings 0 and 1, two
runtime arrays of `float`, and no push-constant variable. The semantic and
erased SPIR-V files remain byte-identical.
