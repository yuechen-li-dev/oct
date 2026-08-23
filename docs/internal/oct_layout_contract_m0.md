# OCT-LAYOUT-CONTRACT-M0 — Cross-Workload Semantic Layout IR Study

## 1. Verdict

**Success**

Current Oct and current local Copeland source support a coherent, small semantic
layout contract, but they do not support making physical memory layout part of
Oct. The contract that survives the cross-workload test is narrower than the
opening hypothesis:

- stable logical identity and lookup-key constraints;
- typed origin/provenance;
- exact extent or a semantically enforced hard bound;
- observable order;
- immutable/publication status.

Access family, hot fields/axes, and index eligibility are useful compiler
metadata, not semantic promises. Batch element/result shape already belongs to
`MIRBatchMap` and should remain adjacent to, rather than inside, data layout.
AoS/SoA, dense slots, offsets, padding, alignment, cache lines, allocation,
buffer ownership, and Go slice/map choices remain backend mechanisms.

The recommended milestone outcome is **option 2: add semantic LayoutContract IR
only; derive it from existing Oct constructs, no new syntax yet**. This report
designs that IR but implements nothing.

## 2. Research question

> Can Oct represent a small, general set of semantic layout facts that improve
> compilation/materialization across multiple unrelated workloads without
> turning Oct into a physical-memory-layout language?

**Answer: partially yes.** A bounded compiler-visible contract generalizes
across immutable compiled read data, numerical arrays/matrices, and persistent
FLOW control state. The common contract is about what values are allowed to
mean—not where their bytes sit. Some initially proposed facts do not generalize
as semantic invariants:

- access family and hotness are optimization metadata;
- relationship/index eligibility is derived metadata unless the relationship
  is itself observable;
- batch shape is execution IR;
- capacity is either a semantic hard bound or a backend preallocation choice,
  never an ambiguous single field;
- publication epoch is semantic only where a consumer observes snapshot/version
  identity.

## 3. OctetDB evidence carried forward

The motivating OctetDB C2 experiment is evidence for preserving semantic shape,
not universal proof of one representation. In the measured safe-Go lane:

- arbitrary external IDs mapped once to stable dense slots;
- authoritative accounts and behavioral state became compact contiguous slices;
- offered batches stayed batches through one ordered owner and one durable
  group;
- the WAL used a fixed-layout, checksummed batch encoding with a reusable safe-Go
  buffer;
- exact bounded deduplication, ledger history, durable acknowledgement,
  recovery, final-state digest, and per-agent behavioral state were retained;
- Go GC stayed enabled; the lane used no `unsafe`, arenas, custom allocator,
  mmap ownership, Direct I/O, or `io_uring`.

The final rotated medians reported 364,012 ops/s at batch 512 versus 228,196 for
the measured one-replica TigerBeetle control and 72,273 for direct Go. At 100k
accounts C2 reported 55,929 ops/s and 55.3 MB RSS versus direct Go's 36,639
ops/s and 1.55 GB RSS. Hot-source traffic improved from 468 to 61,364 ops/s
without weakening input-order serialization. These results came from a coupled
architecture change; they do not isolate or mandate AoS, dense slots, or the
binary encoding individually.

The transferable facts are therefore:

- external identity is distinct from a dense backend slot;
- a batch's one-input/one-output ordered shape is valuable compiler knowledge;
- bounded cardinality can justify prepayment when the bound is real;
- immutable fixed-schema data can be materialized without runtime reflection;
- the backend can choose representation after semantic facts survive lowering.

The non-transferable details are ledger names, account fields, transfer commands,
TigerBeetle's mechanisms, and C2's particular Go structs/slices/maps.

Evidence inspected read-only: `Database-Scheduler/docs/experiments/OCTETDB_LAYOUT_M0.md`
and `Database-Scheduler/experiments/LayoutM0/summary.json`. No file in
Database-Scheduler was changed.

## 4. Current Oct data model inventory

The authority order used here is current `Language/reference`, current language
fixtures, then current Go implementation. Historical reports are supporting
evidence only where current code agrees.

### Records and record tables

`Language/reference/language/11-records.md` defines ordinary records as nominal,
immutable product values. Declaration field order is type/schema information,
but record construction order is explicitly irrelevant.

`record table` is a distinct nominal record form for immutable validated
columnar data. The declared field type is a cell type; storage adds one array
depth. All columns have one shared extent. `Len(table)`, direct column access,
and `table[i]` are observable. A row projection is a compiler-owned anonymous
immutable record. Whole-column `with` returns the same nominal table and checks
replacement extent.

Current carriers and losses:

| Phase | Facts retained | Facts absent or lost |
| --- | --- | --- |
| AST `ast.RecordDecl` | nominal name, ordered fields/types, `IsTable`, `IsConcept` | key, uniqueness, bound, origin, order kind, publication state, access family |
| type checker | table marker, column types, anonymous row type, equal extent, immutability | no key/index/identity-role model; no hard-bound model |
| ordinary MIR | `MIRRecord` name and field types; table storage has already gained `[]`; synthetic row record is emitted | `IsTable` itself is lost; semantic table-versus-record distinction, extent relation, and source provenance are not represented |
| Go backend | typed structs/slices and checks generated from lowering context | no compiler-visible key/bound/order/access contract from which to choose an alternate realization |
| compiled-data IR | nominal type graph, ordered fields, `Table` bit, validated value, hashes, exact row count | no key, uniqueness, origin, order kind, publication epoch, access topology, or index relation |

The loss at ordinary MIR is concrete: `lowerProgram` converts a table field to
`fieldType + "[]"` and creates an `__oct_table_row_*` record, but `MIRRecord`
has only `Package`, `Name`, and `Fields`. A later backend cannot distinguish a
table contract from an ordinary record containing arrays without reconsulting
AST/typechecker state.

### Concepts

`Language/reference/language/18-concepts.md` defines:

- transparent aliases over existing concrete types;
- record-shaped Concepts lowered to ordinary nominal records;
- refined scalar/array Concepts with compile-time proof or explicit checked
  construction.

Concepts reuse the type system. They do not currently quantify over table
structure, fields, keys, cardinality, publication, or provenance. The reference
is explicit that `record table` remains record-only. Concepts can express a
bounded valid value when the bound is a predicate over one refined scalar or
array value and the bounded evaluator can prove/check it. They cannot currently
state “this table has unique field K” or “this value was published from origin
O” without widening Concept semantics.

### Arrays, vectors, and matrices

`Language/reference/language/07-arrays.md` makes arrays homogeneous ordered
containers. Runtime length and integer index order are observable. Arrays can
be jagged, so nested-array rectangular shape is not a semantic default.

`Language/reference/language/16-vectors-and-matrices.md` separates arrays from
mathematical vectors/matrices. Matrix row/column shape and contraction axes are
semantic mathematical structure. Current ordinary MIR carries element types
and operation-specific structure but has no general data-object contract for
exact extent, axis order, origin, or access metadata. Go currently realizes
vectors as slices and matrices as slices of slices; that is a backend choice,
not the mathematical meaning.

### Batch and `MIRBatchMap`

`Language/reference/runtime/22-batch.md` defines a one-input-item to
one-output-item mapping over arrays. Output length and index order equal input
length/order; the lowest failing input index wins and no partial output is
published.

Current `MIRBatchMap` retains:

- input and output targets;
- worker identity;
- input and result element types;
- explicit lexical captures;
- whether the batch is nested.

This is already a successful semantic-preservation seam. It does not retain an
exact/static input length or connect its input to a record-table identity/key
contract. The Go backend uses the MIR node to choose sequential or contiguous
range execution and direct indexed result writes.

### FLOW and board state

`Language/reference/runtime/21-octomata.md` defines board memory as private,
fixed-field-shape, behavior-local control memory. It is not general application
storage. Fields are declared up front, default initialized, mutated only inside
states, and observable through detached typed `BoardSnapshot` values. Yield,
resume, history, result, and checkpoint compatibility are observable.

`MIRFlow` retains the flow's nominal name, construction/turn inputs, yield and
return types, fixed board field list, entry state, and state graph. The compiled
backend can already specialize board fields and policy sites. There is no
separate carrier for checkpoint relevance, stable field/schema identity, exact
array extent, or hot turn-state metadata. Current generated checkpoint logic
uses a flow fingerprint and typed schema, which is stronger evidence for
semantic compatibility identity than for physical memory layout.

### Octagon

`Language/reference/tooling/34-octagon.md` defines `.octagon` as a data-only,
single-value typed interchange format. It carries scalar/array/record/enum
surface data. A record table is represented by its nominal table literal and
complete column arrays. The loader materializes against an expected Oct type.

Octagon is therefore:

- a source data object when authored and loaded;
- a semantic snapshot when emitted for later typed consumption;
- publication data when an artifact chooses it as output;
- compiler input only through explicit `LoadOctagon<T>` evaluation.

It is not a memory image. The payload has no general envelope for source path,
publication epoch, access hints, index topology, allocator state, or physical
layout. The loader knows its filesystem path while loading, but the resulting
Oct value does not retain that path as provenance.

### `Artifact.WriteCompiledData` and compiled-data backend

The type checker admits `(path: String, symbol: String, value: typed immutable
data)` and requires a `.go` path. The artifact interpreter reconstructs a
`compileddata.Type` from the typed value/declarations, converts the value, and
calls `compileddata.EmitGo`.

The backend deliberately emits backend-native static Go, not a wire format or
runtime decoder. Root record tables currently become fixed Go row arrays;
ordinary arrays become fixed Go array literals. It emits canonical logical and
schema SHA-256 constants plus row count. It emits no `init`, parser, reflection,
or runtime reconstruction.

The existing compiled-data fixture proves that unique/sorted IDs and a
published-ID projection can be validated/derived in ordinary Oct. However,
those facts live only in helper code and generated companion data. The
compiled-data IR cannot name `ID` as a unique lookup key or `PublishedIDs` as an
index relationship, so later materializers cannot act on that meaning.

### Publication/read-model examples

`Language/Tooling/CompiledData/valid/compiled_data_publication.octest` loads a
nominal `Catalog` table, replaces one column immutably, proves unique sorted
IDs, derives published IDs, and writes both as compiled data. The artifact
system records source provenance for the artifact entry and generated-byte
hash/status, but that artifact provenance is not attached to the typed dataset
or emitted logical hash.

`Language/Data/Octagon/Load/valid/publication_catalog.octagon` and its typed load
fixture prove that nominal table identity, enum cells, refinements, shared
extent, and row order survive Octagon materialization. They do not declare a
key, index, publication epoch, or origin-bearing value.

## 5. Copeland layout precedent

Current local Copeland was inspected read-only at commit
`2b404befdd0aa29bbcacd3dda693f2d6fb2970a6`.

### 5.1 What Copeland calls a layout

Copeland calls `layout` finite immutable spatial structure. Current
`docs/Copeland/machina-layout.md` says it is a value that binds directly to a
typed layout record/tree, validates and normalizes stable named identities, and
projects through Machina. It is not CSS/Flexbox, a view-returning function, or
runtime mutation.

There are two relevant tabular facilities:

1. a fully implemented CSV-shaped `csv overlay ...` stream authoring surface;
2. read-only compiler-projected relations such as `layout::Layouts`,
   `layout::Boxes`, `layout::Bindings`, `layout::Derivations`, and
   `layout::Sources`.

### 5.2 Semantic facts

Evidence-backed semantic facts include:

- stable layout and named-slot identities;
- exact finite topology and optional `layout type` conformance;
- typed coordinate unit (`px` versus `ui`);
- mandatory declared root origin and its local coordinate-space meaning;
- unique sibling/slot names;
- immutable composition;
- declaration/authored order where it is the final paint tie-breaker;
- layer identity/rank, local z, and resolved paint order;
- directed derivation identity, source/target, read/write fields, and
  single-writer/cycle constraints;
- source spans and project-relative source provenance.

Changing these can change identity, validity, geometry, composition, or paint
behavior.

### 5.3 Spatial/physical facts that are not memory layout

Copeland also carries width/height, x/y, gap, padding, overflow, grid columns,
relative transforms, and host-unresolved origin. These are spatial semantics or
backend-resolvable constraints. They are “physical” in the UI sense, but they
say nothing about byte offsets, alignment, allocation, pointer ownership, or
AoS/SoA.

The transferable lesson is typed domain origin plus explicit resolution phase,
not copying `px`, `ui`, rectangles, z-order, or CSS into Oct's general data IR.

### 5.4 Origin representation

Copeland binds authored coordinates into:

```text
BoundLayoutCoordinate(Value, Unit)
BoundLayoutOrigin(X, Y)
```

and carries the non-null origin on `BoundLayoutDeclaration`. Normalization
produces `NormalizedLayoutOrigin(Local, ResolvedHostRelative?)`. The host-relative
value is deliberately absent until a backend provides a containing coordinate
space. Child nodes separately carry an origin relation such as declared root,
flow-derived, anchor-derived, or overlay-derived.

This is a clean precedent for distinguishing typed origin from resolved
backend placement and for making “not resolved yet” explicit.

### 5.5 Lowering specialized layout tables

The implemented CSV overlay is syntax sugar over the existing spatial model:

```text
StreamTableSyntax rows and ordinary cell expressions
  -> typed cell binders
  -> existing BoundLayoutNode overlay/slot nodes
  -> ordinary BoundLayoutBindingEntry values
  -> exact inferred topology + NormalizedLayoutGraph
  -> React/CSS or projected inspection relations
```

Nested nodes and CSV rows converge before normalization. Row names become the
same semantic slot identities as nested authoring. Relative derivation cells
become `BoundRelativeDerivationSpec`, then `BoundRowDerivation`, then
`layout::Derivations` rows. No source text is emitted/reparsed and no untyped
runtime table is introduced.

Projected layout relations are read-only views over canonical bound/normalized
facts. `layout::Derivations`, for example, exposes stable IDs, foreign keys,
read/write sets, authored order, status, gap/padding, and a foreign key to
`layout::Sources`. The table is an inspection view; the normalized graph and
derivation records remain authoritative.

### 5.6 Status of `layout table`

`layout table Name` is **reserved/parsed but not implemented as a supported
layout profile**. `Parser.ParseLayoutDeclaration` accepts a possible profile
token. Current `LayoutDataCompiler.BindDeclaration` rejects every profile
except `page` with `COPE-LAYOUT-PROFILE-0001`. Therefore `table` is not a
working layout profile.

Separately, `csv overlay` is implemented and tested. It must not be described
as implementation of `layout table`.

There is a current documentation/source inconsistency: line 107 of
`docs/Copeland/machina-layout.md` says M0 accepts the general profile only, but
current binder source accepts `page`. The historical layout-data review also
lists table-profile implementation as absent. Current source controls this
study; the discrepancy should be corrected in Copeland, not copied into Oct.

### 5.7 Ideas that transfer cleanly

- semantic identity independent of renderer/backend identity;
- typed origin with a separate resolution phase;
- authored order only where observable;
- immutable normalized graphs/relations;
- explicit derivation provenance and stable source IDs;
- specialized authoring lowering into existing ordinary IR;
- read-only compiler projections as inspection, not duplicate authority;
- one canonical compiler-visible world consumed by build and tooling.

### 5.8 Ideas that should not transfer

- coordinate origins (`px`/`ui`) as a general data-origin model;
- boxes, anchors, overlays, layers, z, paint order, CSS, and host rectangles;
- spatial single-writer transforms as universal record relations;
- mandatory origin for every Oct value (it is mandatory only for Copeland
  spatial layouts because every box needs a coordinate space);
- CSV-shaped syntax or the name `layout table` merely for familiarity.

## 6. Workload 1 — compiled read data

Workload: the current compiled `Catalog` publication fixture and Octagon-backed
publication catalog.

| Fact | Present in source today? | Semantically observable? | Useful to optimizer/backend? | Stable across backends? |
| --- | --- | --- | --- | --- |
| nominal table identity | yes, `record table Catalog` | yes | yes | yes |
| row identity | only positional row projection; no declared stable identity | position is observable, stable entity identity is not declared | yes if declared/proved | yes |
| lookup key (`ID`) | used by helpers, not declared as key | helper behavior observes it; table type does not | yes | yes |
| uniqueness of `ID` | checked by `AllIDsUnique`/`StaticAssert` | publication validity in this artifact | yes | yes |
| exact cardinality | known after Octagon load/evaluation; emitted as row count | `Len` is observable | yes | yes |
| hard upper bound | no | no | potentially | yes if admission enforces it |
| row order | yes through column arrays and `table[i]` | yes | yes | yes |
| sorted-by-ID order | checked in helper, not attached to value | artifact validity here | yes | yes |
| immutability | yes | yes | yes | yes |
| publication status | artifact phase and `Status` cells exist; dataset publication state is not a type fact | artifact output is observable | yes | yes |
| logical/schema identity | derived by compiled-data emitter | emitted constants are observable to Go consumer | yes | yes conceptually |
| origin/provenance | source path exists in load/artifact machinery, not in dataset | not currently part of value meaning | useful for audit | yes |
| precomputed index | authored as separate `PublishedIDs` value | yes to consumer | yes | yes |
| hot columns/access family | no | no | yes | yes as metadata |

Findings:

- Existing source is sufficient to derive nominal shape, immutability, exact
  extent, and observable positional order.
- The unique/sorted key and derived index are visible only as executed helper
  logic. A later materializer cannot discover their role from compiled-data IR.
- Root tables are always emitted row-oriented even though source tables are
  columnar. That is a backend choice made without access metadata.
- A contract could permit a backend to choose a fixed row array, separate
  columns, a dense direct index, or a map/search structure without source-level
  AoS/SoA.

## 7. Workload 2 — numerical/scientific data

Workload: `Libraries/Signal/Signal.Core.oct`, the current
`SignalBatchAnalysis` experiment, and the authoritative vector/matrix surface.
The signal library uses ordered `Float[]` samples for convolution/correlation;
the batch experiment maps a homogeneous `Float[]`; matrix operations use
mathematical row/column shape and explicit contraction axes.

| Fact | Present in source today? | Semantically observable? | Useful to optimizer/backend? | Stable across backends? |
| --- | --- | --- | --- | --- |
| element type | yes | yes | yes | yes |
| ordered sample index | yes for arrays | yes; convolution/correlation depend on index/lag order | yes | yes |
| runtime extent | yes through `Len` | yes | yes | yes |
| exact compile-time extent | literals/constructors sometimes; not a general array type fact | yes for value, not always type | yes | yes |
| matrix rows/columns | yes at runtime and through mathematical operations | yes | yes | yes |
| jagged nested-array possibility | yes | yes | yes | yes |
| contraction/free axis | present in tensor/matrix operation structure | yes | yes | yes |
| hot iteration/access axis | derivable locally from loops/contractions in some cases; not declared | no | yes | yes as metadata |
| contiguity eligibility | not stated; may be inferred when value semantics allow | no | yes | yes as metadata |
| batch element/result shape | yes in `batch` and `MIRBatchMap` | yes | yes | yes |
| stable entity key | no evidence | no | no | not applicable |
| origin/provenance | experiment/artifact paths exist, not carried on arrays | usually no; sometimes audit metadata | sometimes | yes |
| hard bound | only where an algorithm validates one; ordinary array length is dynamic | only if failure/admission depends on it | yes | yes |

Findings:

- The strongest numerical layout facts are exact/runtime shape and observable
  axis order, not keys or row identity.
- Matrix/tensor shape already belongs to existing mathematical types and
  operation MIR. LayoutContract must reference it, not duplicate it.
- “Contiguous” and “hot axis” are optimizer conclusions. A C, WASM, Go, or GPU
  backend may realize them differently.
- An array literal of known extent can carry an exact compile-time extent fact;
  a function parameter `Float[]` cannot truthfully gain one.
- Signal convolution proves order semantics; reordering samples for locality
  would change results. Physical storage may still change if indexing semantics
  are preserved.

## 8. Workload 3 — stateful/control data

Workload: current Octomata board/snapshot and FLOW-turn checkpoint fixtures,
especially `DurableController` and `ScalarBoardProbe`.

| Fact | Present in source today? | Semantically observable? | Useful to optimizer/backend? | Stable across backends? |
| --- | --- | --- | --- | --- |
| flow nominal identity | yes | yes at type/checkpoint boundary | yes | yes |
| fixed board field shape | yes | yes through behavior and snapshot schema | yes | yes |
| stable board field identity | field names/types exist; no explicit version identity | snapshot/checkpoint compatibility makes it meaningful | yes | yes |
| board ownership | yes, private to one flow instance | yes | yes | yes |
| persistent state across turns | yes | yes | yes | yes |
| checkpoint-relevant fields | derivable from board/resume/history/utility/yield semantics | yes | yes | yes |
| hot turn-state fields | not declared; compiler can inspect reads/writes | no | yes | yes as metadata |
| array extent in board fields | arrays supported but default empty/dynamic | only current length is observable | yes | yes |
| hard array bound | no general board bound | no | potentially | yes if enforced |
| iteration/order | array fields preserve array order; state history order is observable | yes | yes | yes |
| origin/provenance | flow fingerprint/source exists in compiler/checkpoint machinery, not as board value origin | compatibility/audit relevance | yes | yes |
| dense slot | no | no | yes | no as semantic fact |

Findings:

- FLOW already benefits from semantic specialization: current MIR preserves
  fixed board shape and policy sites, enabling typed fields and elimination of
  unused generic machinery.
- Stable field/schema identity matters for checkpoint compatibility, but byte
  offsets and packing do not.
- Checkpoint relevance should be compiler-derived from FLOW semantics. A user
  should not annotate cache lines or “persist this Go field.”
- Board remains behavior-local memory. LayoutContract must not make it a
  database, global table, or generic application heap.

## 9. Optional fourth workload

Copeland spatial layout was used as a fourth, non-Oct workload precedent rather
than inventing an Oct geometry DSL. It changes the stress profile:

- stable identities and authored order are pervasive;
- origin is mandatory and typed because coordinate-space anchoring is semantic;
- derivation/source provenance is first-class;
- width/height/x/y are domain semantics;
- memory layout remains entirely absent.

This supports generalizing typed origin/provenance and stable semantic identity,
while rejecting a universal spatial-origin payload or geometry-specific fields
in Oct's LayoutContract.

## 10. Cross-workload semantic-fact matrix

Required comparison table:

| Fact | OctetDB | Compiled Read Data | Numerical Workload | Stateful/Flow Workload | Semantic? | Backend-owned? |
| --- | --- | --- | --- | --- | --- | --- |
| stable logical identity | account/external ID | nominal table; entity ID only if declared/proved | usually absent | flow and board-field/schema identity | yes when present | representation is backend-owned |
| lookup key | AccountID | `ID` used and uniqueness proved by artifact helper, but role not carried | absent | absent in tested flow | yes if lookup behavior names it | index structure yes |
| key uniqueness | exact account/command identities | publication invariant in fixture | absent | absent | yes when enforced | checking strategy/index yes |
| typed origin/provenance | durable/source lineage relevant | load/artifact source exists but is detached from data | optional experiment/sample lineage | flow/checkpoint source/fingerprint lineage | semantic for compatibility/audit cases; otherwise metadata | storage encoding yes |
| exact cardinality/shape | configured population/batch | exact row count | array/matrix extent where known | fixed field count; dynamic array lengths | yes for exact value shape | capacity/storage yes |
| hard bound | dedupe/population admission where enforced | none declared | only algorithm-specific validation | none for board arrays | yes only when failure/admission changes | preallocation yes |
| observable order | offered/result and ledger order | table index/column order; checked ID sort | sample/lag/axis order | state history and board-array order | yes | physical order may differ behind mapping |
| immutability/publication | durable committed snapshots | immutable table and static generated data | values/inputs commonly immutable | detached board snapshots/checkpoints | yes at publication boundary | buffer ownership yes |
| access family/hot fields | lookup-heavy IDs, hot accounts | lookup/scan and selected columns | streaming loops/contraction axes | repeatedly read/write turn fields | no, metadata | realization yes |
| batch element shape | homogeneous command batch | possible batch over rows | explicit homogeneous `Float` batch | turn input is one message, not general batch | execution semantic, not data-layout invariant | scheduling/chunking yes |
| relationship/index topology | external ID -> dense slot is mechanism; ledger relations semantic | catalog -> published-ID projection | contraction graph is operation semantics, not table index | state-transition graph belongs to FLOW MIR | sometimes semantic, often derived metadata | concrete indexes/slots yes |
| AoS/SoA | C2 chose one coupled representation | emitter currently rows from column source | backend-dependent | backend-dependent board struct | no | yes |

### Per-workload candidate inventory notes

Facts rejected for lack of evidence:

- a universal `publication epoch`: no tested numerical or FLOW value requires
  one; use a publication/snapshot identity only at actual publication/checkpoint
  boundaries;
- universal `row grouping`: numerical samples and FLOW fields are not rows;
- universal `relationship graph`: FLOW already has a state graph and tensor
  operations already have axis structure in their own MIR; duplicating both
  would create a second semantic system;
- source-declared hot columns: current examples provide no necessity evidence
  and compiler derivation is plausible.

## 11. Semantic invariant vs optimization metadata vs backend mechanism

Required boundary table:

| Layout fact | Source/IR/backend | Semantic invariant / metadata / mechanism | Why |
| --- | --- | --- | --- |
| stable identity | existing source structure when explicit; derived LayoutContract IR | semantic invariant | changing identity can break lookup, compatibility, recovery, or references |
| origin/provenance | typed source/artifact facts; LayoutContract IR when relevant | semantic invariant when compatibility/audit depends on it; otherwise compiler metadata | provenance can determine compatibility or explain derivation, but not every value needs it |
| key uniqueness | derived/proved from source/static assertion or future explicit semantic surface; IR | semantic invariant | duplicates change validity/lookup meaning |
| lookup key | derived from observable keyed operations; IR | semantic invariant when API meaning names the key | key role survives all backends; index form does not |
| hard cardinality bound | source/typecheck only if enforced; IR | semantic invariant | exceeding it changes admission/failure behavior |
| preallocation capacity | backend | backend mechanism | changing it must not change Oct results |
| observable order | existing array/table/batch/FLOW semantics; IR reference | semantic invariant | reordering changes indexed reads, numerical results, failure selection, or history |
| authored order | IR only where semantics consume it | semantic invariant in tie-breaking/sequence cases; otherwise provenance metadata | Copeland paint tie and Oct array/table construction differ from unordered record construction |
| stable canonical order | IR if a published compatibility rule establishes it | semantic invariant | consumer-visible canonicalization can affect hashes/compatibility |
| optimizer physical order | backend | backend mechanism | may change behind semantic mapping |
| hot access family | compiler-derived or profile-guided IR metadata | optimization metadata | changing it should affect performance only |
| likely lookup field | compiler-derived IR metadata until semantically proven as key | optimization metadata | a likely lookup is not uniqueness or identity |
| batch element shape | existing `MIRBatchMap`, with optional reference to LayoutContract subject | semantic execution invariant | one-to-one shape/order is observable, but it describes execution batching |
| expected batch size class | compiler/profile metadata adjacent to batch | optimization metadata | not correctness |
| exact batch length | MIR/value fact when known | semantic execution invariant | output extent equals input extent |
| index topology | derived LayoutContract metadata unless API exposes it | optimization metadata | map, search table, perfect hash, or dense index can change |
| relationship graph | owning domain IR; contract references only identity edges needed for materialization | semantic invariant or metadata depending on observability | FLOW/tensor/table relationships have different owners; no universal graph should duplicate them |
| AoS/SoA | backend | backend mechanism | representation only |
| byte order | backend except an existing wire/publication format contract | backend mechanism in LayoutContract | no new runtime format is proposed; a serializer may separately own byte order |
| padding | backend | backend mechanism | no Oct semantic effect |
| alignment | backend | backend mechanism | machine-specific |
| cache-line placement | backend | backend mechanism | machine-specific and profile-dependent |
| buffer ownership | backend/runtime | backend mechanism | implementation lifetime/aliasing strategy; Oct value semantics remain authoritative |
| allocator | backend/runtime | backend mechanism | implementation choice |
| dense backend slot | backend | backend mechanism | derived from stable external identity, never a replacement for it |

Classification rule for mixed facts: the IR must record the invariant and the
metadata separately. It must never encode a semantic hard bound as a capacity
hint or a likely lookup field as a unique key.

## 12. Minimum LayoutContract IR

Design only. The minimum is an optional attachment to an existing typed data
subject, not a new type graph:

```text
LayoutContract {
    Subject: DataSubjectRef

    Invariants: {
        Identity?: IdentitySpec
        Origins: OriginRef[]
        Keys: KeySpec[]
        Extent?: ExtentSpec
        Order?: OrderSpec
        Publication?: PublicationSpec
    }

    Metadata: {
        AccessFamilies: AccessFamily[]
        IndexEligibility: IndexCandidate[]
    }
}
```

The referenced parts are deliberately small:

```text
DataSubjectRef
    existing MIR record/table, array value, matrix/tensor value,
    compiled dataset, FLOW board schema, or publication result

IdentitySpec
    scope: value | entity | field-schema | snapshot
    source: nominal-type | field-ref | compiler-derived
    stability: within-value | across-publication | across-recovery

OriginRef
    kind: source | artifact-input | derived-from | publication | checkpoint
    identity: existing compiler source/artifact/value identity
    resolution: source | compile | publication | startup | runtime

KeySpec
    fields: existing FieldRef[]
    role: identity | primary-lookup | external-reference
    uniqueness: required-and-proved | required-and-runtime-checked

ExtentSpec
    exact: existing constant/value-shape expression
    OR hardUpper: existing checked expression + enforcement phase
    // never a capacity/preallocation hint

OrderSpec
    kind: observable-index | authored | canonical
    basis: existing sequence identity or FieldRef[]

PublicationSpec
    state: immutable-snapshot
    identity?: existing logical/schema/flow fingerprint reference
    compatibility?: existing schema/type identity reference

AccessFamily
    kind: lookup | scan | append | axis-traversal | read-mostly | update-heavy
    selectors: FieldRef[] or existing AxisRef[]
    provenance: compiler-derived | profile-guided
    confidence: exact | heuristic
```

### Design constraints

- `FieldRef`, type identity, dimensions, array depth, matrix axes, FLOW fields,
  and expressions refer to existing typed IR. They are not copied into a second
  type system.
- The invariant and metadata substructures are separately typed; no consumer
  may treat a hint as a proof.
- Optionality is essential. Numerical arrays need order/extent but usually no
  key; a compiled catalog may use all invariant fields; a FLOW board needs
  schema/snapshot identity but no lookup key.
- The attachment survives until materialization. A backend may consume it and
  discard it after committing to a representation.
- `IndexEligibility` says only that a derived index is legal/useful. The
  concrete index is backend-owned unless published as a separate semantic value.
- Relationship graphs stay in their owning IR. LayoutContract may reference a
  key/derived-value edge; it does not re-encode FLOW transitions or tensor
  contractions.
- Batch shape is not a field. `MIRBatchMap` may reference the input/output
  subjects' contracts.

## 13. Record-table relationship

The answer is: **metadata attached to existing record-table/data-object IR,
with compiler derivation and no new source syntax**.

| Candidate | Strength | Failure mode | Decision |
| --- | --- | --- | --- |
| specialized declaration/type | could make constraints explicit | adds language/type surface; does not cover arrays, matrices, or FLOW boards without generalization | reject for M0 |
| metadata on existing record-table IR | preserves nominal table/type authority; easy field references; survives materialization | needs a real table marker in MIR and derivation pass | preferred for table subjects |
| Concept-driven constraint | can reuse nominal/refined validity rules | current Concepts cannot quantify table fields/uniqueness/provenance; a layout Concept would imply misleading type identity | do not use in M0 |
| compiler-derived contract, no syntax | zero language surface; can cover tables, arrays, compiled data, and boards | some key/bound facts may remain unprovable until later evidence | preferred overall |

LayoutContract is not a specialized record table because numerical arrays and
FLOW boards are valid subjects and because a table's type/schema remains owned
by `record table`. It is not a Concept over record tables. It is a compiler-side
attachment keyed by an existing typed subject. The ordinary MIR should first
stop erasing the table marker/row-extent relation; the contract then adds only
facts not already in the type.

## 14. Concept relationship

### Can existing Concepts express these facts?

- Stable nominal value identity: record-shaped Concepts already provide it.
- Scalar/array value bounds: refined Concepts can express some predicates over
  `Self`, including `Len(Self)`, subject to the bounded evaluator.
- Stable table key: no.
- Cross-row uniqueness: no.
- Published immutable: no; ordinary records/tables are immutable values, but a
  publication lifecycle is not a Concept predicate.
- Origin-bearing: no.

### What should be inferred?

- nominal identity from record/Concept/table types;
- immutability from value/table semantics and publication phase;
- exact extent from literals, constructors, Octagon materialization, or static
  evaluation;
- matrix axes from existing mathematical types/operations;
- board field/schema identity from FLOW MIR;
- likely access family from uses and profiles.

### Facts that are not types

Access hotness, likely lookup field, profile-guided locality, publication source
path, and index eligibility can vary without changing a value's Oct type. They
belong in compiler metadata. Snapshot compatibility identity can be semantic
without becoming a value type.

### Why not a layout-specific Concept?

A nominal `HasStableKey`-style Concept would suggest that layout is a runtime
value category or nominal domain identity. It would also need field predicates,
cross-row quantification, lifecycle state, and compiler provenance—none of
which current Concepts own. That would widen Concepts and create a second
constraint/type system. Do not do this in M0.

## 15. Octagon relationship

Facts that should survive into Octagon only when part of the published semantic
value/compatibility contract:

- nominal record/table/enum/refinement identity (already materialized through
  expected type);
- observable array/table order (already in the value);
- exact extent (already in arrays/columns);
- stable key fields as ordinary data, if present;
- publication/snapshot identity only if a future versioned envelope has an
  independently justified interoperability requirement.

Facts that should normally remain compiler IR/artifact metadata:

- source path and derivation provenance;
- uniqueness proof status;
- access hints and index eligibility;
- hard admission bounds not part of the data payload;
- checkpoint relevance;
- physical index topology.

Facts that must not enter Octagon:

- AoS/SoA, Go slice/map headers, dense slots, byte offsets, padding, allocator,
  pointers, buffer ownership, or cache-line placement.

Octagon can be source data, a semantic snapshot, or a publication
representation depending on the call site. It is not automatically all three,
and it must not become a memory dump. This milestone adds no envelope or runtime
format.

## 16. Compiled-data relationship

### Facts already derivable

- complete nominal type graph and field order;
- table marker and equal column extent;
- exact evaluated row count;
- immutable static value;
- canonical logical and schema hashes;
- literal array extents;
- tag-only enum/refinement representation.

### Facts currently lost or opaque

- whether a field is a semantic key or external identity;
- uniqueness/sortedness proof after artifact helpers return;
- the relationship between a source table and separately generated index;
- source/load origin attached to the dataset;
- semantic versus merely authored/canonical order;
- access family/hot columns;
- hard bound versus backend capacity.

### Potential realizations enabled by the contract

Without source-level physical directives, a backend could legally choose:

- a fixed dense row array when exact extent and row scans dominate;
- separate fixed column arrays when selected columns/axis scans dominate;
- a precomputed map, sorted search index, direct dense index, or perfect-hash
  candidate when a unique key and exact static dataset are known;
- specialized lookup functions that preserve external key and failure meaning;
- generated encoders whose field/order/schema semantics are fixed while byte
  order and buffering remain serializer/backend choices;
- bounded buffers only from an exact extent or semantic hard bound;
- multiple projections from one semantic dataset when hashes/provenance keep
  them correlated.

The current emitter's root-table row array becomes one candidate, not language
law. A binary encoder would still require an independently owned serialization
contract; LayoutContract alone does not create a wire format.

## 17. Batch relationship

Batch is adjacent execution metadata, not part of LayoutContract.

`MIRBatchMap` already retains homogeneous input/result element types, captures,
nested status, one-to-one output shape, and ordered result semantics. It should
gain, at most, references to the input/output `DataSubjectRef` contracts and any
exact extent already known for those values. Expected batch size or body cost
belongs in execution-planning metadata.

Interaction example:

```text
record-table/array contract:
  stable key + observable row order + immutable input

MIRBatchMap:
  one result per row + ordered output + pure/eligible body facts

backend option:
  contiguous range processing and direct indexed writes
```

The backend may still choose sequential execution. Data layout does not imply
parallelism, and batching does not imply a database table.

## 18. FLOW relationship

FLOW-local board state remains behavior-local control memory, not general
storage. Useful semantic layout facts are:

- fixed declared field/schema identity;
- persistence across turns;
- fields required by checkpoint/snapshot compatibility;
- observable board-array order and current extent;
- hard bounds only if FLOW admission/checkpoint semantics actually enforce
  them.

Useful derived metadata includes hot turn-state fields and fields never observed
outside the flow. Current flow specialization already proves the pattern:
compiler analysis can omit unused history/resume/generic helpers and use typed
policy state without changing Octomata semantics.

LayoutContract must not add keyed board tables, arbitrary application records,
global queries, or storage APIs. State-transition graphs remain `MIRFlow`; the
contract references the board schema/snapshot identity and does not duplicate
the graph.

## 19. Cross-backend portability

| Candidate fact | Go | native C | WASM | GPU/Prometheus | serialized publication | Portable conclusion |
| --- | --- | --- | --- | --- | --- | --- |
| stable identity/key | map/slice index choices | hash/array choices | table/linear-memory choices | host/device index choices | encoded key | semantic |
| uniqueness | validation/index | same | same | host preprocessing | schema/data validation | semantic |
| typed origin/provenance | compiler/artifact metadata | same | custom section/host metadata if needed | model/data lineage | manifest/envelope if justified | semantic or metadata, not Go-specific |
| exact/hard extent | arrays/slices | arrays/pointers | linear memory/table | dispatch/tensor sizes | payload count | semantic when exact/enforced |
| observable order | indexed API | same | same | kernels must preserve logical order | encoded order | semantic |
| access family | choose map/columns | choose structures/loops | choose memory/table access | choose axis/coalescing | choose indexes/chunks | portable metadata |
| AoS/SoA | structs/slices | structs/arrays | linear memory layout | often decisive kernel mechanism | encoding layout | backend mechanism |
| allocator/buffer ownership | Go runtime | malloc/custom | linear-memory allocator | device/host buffers | writer buffers | backend mechanism |

“GPU/Prometheus backend” is a thought experiment, not a new backend proposal.
The test shows why mathematical shape/order and access axis can survive while
coalescing, workgroup tiling, and device allocation cannot be Oct language
facts.

## 20. Origin/provenance

Oct has several partial provenance seams today:

- AST declarations/functions retain `SourcePath`;
- `oct artifact --json` reports entry source provenance and output hashes;
- artifact staging records package/function/source path/kind;
- Octagon loader knows the input path during materialization;
- compiled-data emits logical/schema hashes but excludes path/time/machine;
- FLOW checkpoints use version/flow fingerprints and schema compatibility;
- package lock Octagons and experiment artifacts use explicit content identity
  in their owning subsystems.

There is no unified origin-bearing Oct value. That is acceptable.

Recommended policy by workload:

| Case | Origin treatment |
| --- | --- |
| ordinary transient numeric array | compiler-only optional provenance |
| authored/loaded compiled read dataset | optional typed origin metadata; retain through publication if audit/freshness requires it |
| derived static index | `derived-from` relation to the source dataset in compiler/artifact metadata |
| FLOW checkpoint | semantic compatibility provenance: flow/schema fingerprint and checkpoint origin |
| Copeland spatial layout | mandatory semantic coordinate origin in Copeland only |
| external entity identity | identity/key, not automatically origin |

Origin should therefore be a typed list of role-specific references, not a
mandatory universal string and not a coordinate tuple. It becomes a semantic
invariant only when compatibility, recovery, publication, or audit behavior
observes it. Otherwise it remains compiler provenance.

## 21. Bounds semantics

The IR must use distinct variants:

| Bound kind | Meaning | Example | Resolution | May backend preallocate? |
| --- | --- | --- | --- | --- |
| exact extent | this value has exactly N elements/rows | compiled catalog after evaluation; matrix rows/cols | compile/publication/startup/runtime depending on value | yes |
| semantic hard upper | admission fails or program invalid above N | bounded dedupe/state table if enforced | source/typecheck/configured startup | yes, but failure semantics remain |
| configured runtime hard upper | host configuration is part of admitted instance behavior | a controller/store opened with max N | startup | yes |
| estimated size | compiler/profile estimate only | likely 10k samples | compile/profile | yes, as hint only |
| capacity hint | desired preallocation; growth remains legal | reserve 100k slots | backend/startup | yes; not in invariant contract |

`ExtentSpec` admits only exact extent or an enforced hard upper. Estimates and
capacity live in metadata/backend plans. A hard bound must name its enforcement
phase and failure/admission consequence. The literal number `100000` is
insufficient to distinguish semantics from preallocation.

Anything knowable before execution should be propagated to materialization.
Runtime-only lengths remain runtime facts; the compiler must not manufacture
compile-time declarations for them.

## 22. Key/identity semantics

Four separate identities must remain separate:

| Concept | Meaning | Owner |
| --- | --- | --- |
| external identifier | value used by outside systems/callers | Oct semantic data/API |
| stable entity/row identity | sameness across updates/publications/recovery | Oct semantic contract when present |
| primary lookup key | field(s) used to locate an entity, usually with uniqueness | Oct semantic contract when API establishes it |
| dense backend slot | compact internal ordinal used for storage | backend mechanism |

One field may serve the first three, but that is a proved role relationship,
not an assumption. The dense slot is never serialized or substituted for
external identity unless an independent format makes it semantic.

OctetDB C2 demonstrates the correct lowering:

```text
arbitrary external AccountID
  -> one key-to-slot mapping
  -> dense backend arrays
```

Compiled data could similarly map a unique catalog ID to a fixed row/column
slot. Numerical sample index is observable sequence position, not automatically
entity identity. FLOW board field ordinal is a backend choice; stable field name
and schema identity are the semantic facts.

## 23. Order semantics

| Order kind | Semantic status | Examples |
| --- | --- | --- |
| observable index order | invariant | Oct arrays, table rows/columns, batch results, state history |
| authored order | invariant only when a rule consumes it; otherwise provenance | Copeland paint tie; batch case/failure order; record field construction order is not semantic |
| stable canonical order | invariant when used for hashes/interchange/compatibility | canonical schema/value hashing or explicitly sorted published IDs |
| optimizer physical order | mechanism | row permutation behind a key-to-slot map, column packing, SoA/AoS |

For record tables, `table[i]` makes logical row order observable. A backend may
store a different physical order only if it preserves the logical index mapping
and column/row behavior. The contract must not call ordinary record declaration
or literal field-construction order a row-order promise; current record rules
explicitly make construction order independent.

## 24. Access-family metadata

Evidence supports access facts as derived/profile metadata:

- OctetDB: lookup-heavy by account ID, hot-source contention, append-heavy
  durable history;
- compiled catalog: repeated ID lookup and scan/filter over `Status`, with a
  separately derived published-ID projection;
- signal/numerical: streaming scans and loop/contraction axes;
- FLOW: frequently read/written turn fields versus checkpoint-only or
  unobserved fields.

Recommended sources, in priority order:

1. exact compiler derivation from operations (field used as equality lookup,
   loop axis, append target, immutable publication);
2. static artifact derivation evidence (a generated index linked to source);
3. profile-guided metadata with explicit provenance/confidence;
4. no metadata when evidence is weak.

Do not add source performance-hint syntax now. “Likely lookup by K” is not the
same as “K is a unique semantic key.” Profiles can change without invalidating
the program. A backend may ignore all access metadata and must remain correct.

## 25. Lifecycle / prepayment classification

| Fact | Earliest truthful resolution | Notes |
| --- | --- | --- |
| nominal record/table/Concept identity | source/typecheck | already available |
| board field/schema identity | source/typecheck | FLOW lowering already has fields |
| matrix/vector operation shape | typecheck or runtime constructor | do not duplicate existing type/operation facts |
| literal array/table exact extent | typecheck/compile time | preserve to materialization |
| Octagon-loaded table exact extent | artifact/publication or startup | file content is not known to all compile paths |
| unique/sorted catalog key in current fixture | publication static evaluation | helper proves it during artifact execution |
| semantic hard bound | source/typecheck if declared by existing construct; otherwise startup configuration | must name enforcement |
| runtime current length | runtime | no forced compile-time fiction |
| immutable publication snapshot | publication time | can generate logical/schema identities then |
| FLOW checkpoint compatibility | compile time for schema/fingerprint; checkpoint time for instance state | encoding remains boundary-owned |
| access family from source uses | compile time | exact/heuristic provenance required |
| profile-guided hotness | post-profile/next compile or startup | optimization metadata only |
| dense slot mapping | publication/startup/runtime backend plan | never language identity |
| preallocation capacity | backend planning/startup | mechanism |
| AoS/SoA/alignment/cache lines | backend compile/materialization | target-specific |

Prepayment rule: exact extent, proven key uniqueness, immutable publication,
and known batch shape should reach the backend before execution. Unknown input
length, host-relative origin, and profile hotness remain at the phase where they
become truthful.

## 26. Hypothetical lowering examples

These are design notation, not proposed Oct syntax.

### 26.1 Compiled catalog read data

```text
semantic source/evaluation facts
  nominal record table Catalog
  immutable published value
  exact extent = 6
  logical row order observable
  ID uniqueness and sorted order proved during artifact evaluation
  PublishedIDs derived from Catalog.Status

-> LayoutContract
  Subject = CatalogData
  Identity = nominal snapshot identity
  Keys = [{ fields: [Catalog.ID], role: primary-lookup,
            uniqueness: required-and-proved }]
  Extent = exact(6)
  Order = observable-index; canonical-by(ID) only because the artifact proved it
  Publication = immutable snapshot + logical/schema hash refs
  Origins = [artifact-input catalog.octagon, publication output]
  Metadata = scan(Status), lookup(ID), index candidate ID

-> possible Go realization (backend choice)
  [...]CatalogRow + binary search
  OR separate fixed columns + generated lookup
  OR [...]CatalogRow + map[int]uint32
  OR a perfect-hash candidate for the fixed set
```

No choice changes `Catalog` identity, uniqueness, logical order, or hashes.

### 26.2 Signal/numerical data

```text
semantic source facts
  Float[] samples
  observable sample order
  runtime or literal exact extent
  convolution/correlation scans ordered axes
  batch maps one Float to one Float in input order

-> LayoutContract for the array
  Extent = exact(N) only when known
  Order = observable-index
  Metadata = axis-traversal(index 0), read-mostly/scan

-> MIRBatchMap (separate)
  InputType = Float
  ResultType = Float
  ordered one-to-one output; may reference array contract

-> possible Go realization
  []float64 or [N]float64
  contiguous range loop when eligible
  backend-owned scratch/reuse/vectorization decision
```

For a matrix contraction, existing matrix rows/cols and contraction axes replace
new layout type fields; a GPU backend could choose tiled/device storage while a
Go backend chooses slices or flat storage.

### 26.3 Persistent FLOW controller

```text
semantic source facts
  flow DurableController
  fixed private board { Total: Int, Choice: Int }
  board persists across turns
  snapshot/checkpoint compatibility observes field schema
  utility commitment/resume/yield state is checkpoint-relevant

-> LayoutContract for board/snapshot subject
  Identity = field-schema identity stable across checkpoint recovery
  Order = none for field semantics; board arrays would retain index order
  Publication = immutable BoardSnapshot/checkpoint compatibility identity
  Origins = [flow source/fingerprint, checkpoint]
  Metadata = compiler-derived hot fields Total/Choice

-> possible Go realization
  compact typed controller struct
  direct scalar fields
  generated checkpoint encoder with schema/fingerprint checks
  omitted unused history/resume helpers when feature analysis proves absence
```

The board does not become a keyed table or general store.

### 26.4 OctetDB thought-transfer (motivation only)

```text
semantic facts
  external entity ID + stable identity + unique lookup
  enforced population/dedupe bounds
  ordered homogeneous command batch
  immutable durable publication/recovery contract

-> LayoutContract + MIR batch/execution facts

-> possible backend realization
  map[ExternalID]uint32 + dense arrays
  OR another backend-specific index/storage structure
  plus a separately owned durable encoder
```

`AccountID`, `Balance`, `Transfer`, and `Ledger` do not appear in the IR.

## 27. Physical-layout rejection findings

| Directive | Required to recover proven C2 optimization? | Portable? | Semantically observable? | Backend can derive/choose? | Finding |
| --- | --- | --- | --- | --- | --- |
| AoS | no; C2 evidence proves one implementation, not a source requirement | weak | no | yes | reject from source/semantic IR |
| SoA | no | weak | no | yes | reject |
| `align 64` | no | no across targets/cache designs | no | yes | reject |
| `pack 1` | no; may harm targets and changes ABI concerns | no | not in ordinary Oct semantics | serializer/ABI backend | reject |
| `cacheline` | no | no | no | profile/target backend | reject |
| allocator | no; C2 used ordinary safe Go/GC | no | no | runtime/backend | reject |
| arena | no | no | no | runtime/backend if ever justified | reject |
| pointerless | no source directive needed; backend can infer pointer content from types | target representation only | no | yes | reject |
| byte offsets | no | no | no | yes | reject |
| buffer ownership | no | no | Oct value/alias semantics are observable, buffer object is not | yes | reject |

C2's proven gains require the compiler/backend to know stable identity, bound,
batch shape, and immutable fixed schema. They do not require the Oct programmer
to select cache-line width, packing, pointer form, or allocator lifetime.

## 28. Anti-special-case / anti-Zig / anti-magic checks

### Anti-special-case

Pass. The proposed fields contain no ledger or geometry nouns. They apply as:

- identity/key for compiled catalogs and entity stores;
- order/extent for signal arrays and matrices;
- schema/snapshot identity for FLOW boards;
- typed origin/provenance across compiled data, checkpoints, and Copeland-like
  domain IR.

Optional fields are allowed. Generality does not require inventing a fake key
for numerical arrays or a fake row table for boards.

### Anti-Zig

Pass. Normal Oct users need not reason about byte offsets, padding, allocator
lifetime, pointer ownership, manual frees, or cache lines. The contract contains
only logical references to existing types/fields/axes plus semantic bounds and
order. Physical choices are backend plans.

### Anti-magic

Pass only if the IR is added and preserved. Current handwritten/artifact helper
code can know that `ID` is unique/sorted and that `PublishedIDs` is an index,
while compiled-data IR cannot. Ordinary MIR also erases the table marker. If Go
alone owns key/bounds/origin/access topology, semantic/physical drift returns.

The middle boundary is:

```text
Oct/source/typecheck owns or proves meaning
  -> compiler-visible LayoutContract + adjacent execution metadata
  -> backend chooses representation
```

The backend may ignore hints, but it may not invent or contradict invariants.

## 29. Future source-surface sketches

Reconnaissance only; no syntax is recommended now.

### Sketch A — no new source surface (preferred now)

Derive the contract from record tables, Concepts/refinements, arrays/matrices,
FLOW schemas, artifact evaluation, and use analysis. Preserve proof facts in
compiler IR. This is sufficient for the first prototype and avoids premature
promises.

### Sketch B — semantic metadata on an existing record table

If later non-ledger evidence proves derivation insufficient, a future bounded
surface could attach key/hard-bound/order/publication semantics to an existing
`record table`. It must reference existing fields/types and must not contain
AoS/SoA, alignment, capacity, allocator, or cache hints.

No concrete syntax is selected because the required field-constraint and proof
model is not yet established.

### Sketch C — Concept-bound declaration

A future declaration could theoretically constrain a table through reusable
semantic propositions. Current Concepts cannot express cross-row uniqueness,
field roles, provenance, or lifecycle without major widening, so this sketch is
not presently recommended. It is retained only as a comparison, not a syntax
direction.

A separate `layout table` profile is not included among the preferred sketches.
Copeland's name is spatial/domain-specific, its actual profile remains
unsupported, and its implemented CSV overlay solves a different authoring
problem.

## 30. Final recommendation

**2. Add semantic LayoutContract IR only; derive it from existing Oct
constructs, no new syntax yet.**

Rationale:

- at least three unrelated workload families share identity/origin/extent/order/
  publication facts;
- current MIR demonstrably loses table identity and has no carrier for proven
  key/bound/provenance facts;
- numerical and FLOW evidence rejects making the abstraction table-only;
- access facts generalize as metadata, not source promises;
- physical directives are unnecessary for the proven safe-Go gains and fail
  portability/semantic tests;
- existing types, fields, axes, record-table rules, batch MIR, and FLOW MIR can
  be referenced rather than duplicated;
- source syntax is not yet justified because current examples can seed a useful
  derived IR prototype.

## 31. What NOT to implement yet

Do not implement:

- new Oct syntax of any kind;
- `layout table`;
- a new runtime or Octagon format;
- AoS/SoA directives;
- key, alignment, padding, packing, cache-line, allocator, arena, pointer, or
  buffer-ownership syntax;
- source-level access/hot-column hints;
- a layout-specific Concept or widened Concept semantics;
- a second record-table/type system;
- a universal relationship graph duplicating FLOW/tensor IR;
- batch syntax or changed batch behavior;
- board tables, global board storage, or changed Octomata semantics;
- a physical index/slot in Oct semantic identity;
- publication epoch fields without a real compatibility consumer;
- binary encoder/wire-format policy merely because LayoutContract exists;
- any Database-Scheduler change.

Also do not treat the current conceptual IR names as stable public vocabulary.
The prototype should test whether these facts can be derived and consumed before
the language promises a “layout” feature.

## 32. Exactly one next recommendation

**Prototype an internal, syntax-free LayoutContract IR on the existing compiled
catalog fixture and one numerical or FLOW fixture, preserving the record-table
marker through MIR and proving that one backend materialization decision can
consume the contract without changing language behavior.**

The prototype should remain compiler-internal, reuse existing type/field/axis
references, keep access hints separately typed from invariants, and add no new
runtime format. Its success criterion should be one real materialization
improvement plus unchanged authoritative language fixtures—not merely a new IR
struct.
