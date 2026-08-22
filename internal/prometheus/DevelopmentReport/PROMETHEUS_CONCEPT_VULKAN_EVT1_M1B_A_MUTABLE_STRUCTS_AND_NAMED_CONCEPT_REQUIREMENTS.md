# Concept/Vulkan EVT1 M1B-A — Mutable Structs and Named Concept Requirements

Status: **SUCCESS**

Expected commit for this pass: `concept-vulkan: add structs and named concepts`

## 1. Full starting checkpoint and worktree state

- starting checkpoint: `d7d6464ceed412d70921f3d031d6e6c10368660e`
- starting branch: `main`
- starting worktree: clean porcelain
- starting repository state: M1A commit at `HEAD`, no uncommitted files

## 2. Inspected M1A compiler authority

Inspected before implementation:

- `README.md`
- `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
- `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_EVT1_M1A_PAYLOAD_ENUMS_AND_EXHAUSTIVE_MATCH.md`
- `cmd/concept-vulkan/main.go`
- `internal/conceptvulkan/conceptvulkan.go`
- `internal/conceptvulkan/evt1_parse.go`
- `internal/conceptvulkan/evt1_types.go`
- `internal/conceptvulkan/evt1_validate.go`
- `internal/conceptvulkan/evt1_generate.go`
- `internal/conceptvulkan/evt1_test.go`
- `Examples/Concept-Vulkan/evt1_m1a_language.concept`
- `Examples/Concept-Vulkan/evt1_m1a_vulkan.concept`
- checked M1A generated outputs under `internal/conceptvulkan/generated/`
- Oct concept terminology in `Language/reference/language/18-concepts.md`
- historical Oct concept background in `docs/experiments/CONCEPTS_M0.md`

## 3. Inspected ownership and `immovable` authority

- repository-wide search found no existing `immovable` authority in tracked source or docs
- existing Concept/Vulkan authority already defined `borrow`, `owned`, `unsafe`, `imported`, and explicit move vocabulary in the language constitution
- existing EVT1 code had no prior stable-address or record-like immutable aggregate semantics
- existing Oct concept documents were separate language history and not Concept/Vulkan authority

## 4. Resolved `immovable` meaning

M1B-A adopts the intended systems meaning directly:

- ordinary `struct` is a mutable value type
- `immovable struct` is mutable but non-copyable and non-relocatable in the current bounded model
- `immovable` does not mean immutable
- `immovable` does not imply structural equality
- `immovable` does not create Oct-style record semantics
- direct construction is allowed only in final local storage
- mutation is allowed through valid mutable borrow paths

No conflicting in-repository `immovable` authority had to be reconciled.

## 5. Scope and non-goals

This pass adds:

- ordinary mutable structs
- bounded immovable structs
- named one-parameter concepts
- prerequisite concepts
- explicit concrete satisfaction assertions
- typed MIR and deterministic C11 lowering for all of the above

This pass does not add:

- templates or monomorphization
- general compile-time evaluation
- runtime interfaces, vtables, or witnesses
- methods, constructors, destructors, or implicit cleanup
- a new ownership or borrow system
- production-route changes

## 6. Exact struct grammar

Canonical declaration and construction:

```concept
struct BufferRange
{
    int bufferId;
    int offset;
    int size;
};

BufferRange range = BufferRange{bufferId, offset, size};
```

Immovable form:

```concept
immovable struct CommandPoolState
{
    VkCommandPool pool;
    bool initialized;
};
```

No designated initializers, field defaults, anonymous structs, tuple structs, or redundant initializer forms were added.

## 7. Struct declaration semantics

- struct identity is nominal and declaration-scoped
- field order is source order
- duplicate struct declarations are rejected
- duplicate field names are rejected
- empty structs are rejected
- field types must resolve
- ordinary and immovable identity is retained explicitly in typed MIR

## 8. Aggregate construction semantics

- construction resolves the struct first, then checks initializer count and type in declaration order
- initializer expressions are type-checked positionally
- ordinary local construction is direct and explicit
- checked lowering introduces no hidden allocation
- owned-field construction is permitted only as direct initialization, not as an implicit copy license

## 9. Initializer evaluation order

All struct and enum constructor arguments lower through explicit temporaries in source order before constructor/helper use. The language specimen proves:

- each initializer executes exactly once
- field positions retain left-to-right source order
- no helper-call argument order leak survives to C11

## 10. Field access semantics

- field access requires a struct-typed base
- nested field access is preserved
- field lookup retains exact field type
- unknown fields are rejected with focused diagnostics
- borrow-based field reads lower to pointer-member access in C

## 11. Field mutation semantics

- whole-field assignment is explicit
- mutation requires a mutable assignable path
- mutation through `borrow const` is rejected
- borrow-based field mutation lowers to direct pointer-member assignment
- no compound assignment, increment, or destructuring assignment was added

## 12. Ordinary copy rules

- ordinary structs are copyable only when every field is copyable under existing ownership authority
- ordinary copy works for local initialization, assignment, enum payloads, parameter passing, and returns only when that copyability rule holds
- copied ordinary struct values are independent after later mutation

## 13. Ownership-containing struct behavior

- a struct containing an `owned` field may be constructed directly
- it is not implicitly copyable
- attempts to copy such a value by local copy, assignment, call, return, or enum payload use are rejected
- no hidden clone, retain, release, or reference counting path was added

## 14. `immovable struct` construction and restrictions

- direct local final-storage construction is supported
- mutation through mutable borrow is supported
- whole-value copy is rejected
- whole-value assignment is rejected
- by-value parameter passing is rejected
- by-value return is rejected
- by-value embedding is rejected
- enum payload placement is rejected

## 15. Exact concept grammar

Canonical form:

```concept
concept Validatable<T>
{
    requires bool IsValid(borrow const T value);
}

concept ResourceState<T>
{
    requires Validatable<T>;
}

requires ResourceState<CommandPoolState>;
```

Supported requirement forms:

1. required free-function signature
2. prerequisite named concept over the same type parameter

## 16. Concept parameter scope

- M1B-A supports exactly one concept type parameter
- the parameter is visible only inside concept requirement types
- it is not a runtime type
- it cannot appear as a local, field, or function type outside that bounded requirement surface

## 17. Operation-requirement semantics

- required operations are matched only against existing free-function declarations/definitions
- matching is exact on name, arity, parameter order, parameter type, borrow/const qualifier, and return type
- no overload search, implicit conversion, or candidate ranking was added

## 18. Prerequisite-concept semantics

- prerequisite concepts are resolved by name
- they must apply to the same concept parameter in M1B-A
- they are checked recursively in declaration order
- nested failure messages preserve the requirement path

## 19. Concrete satisfaction syntax

Declaration-level proof request:

```concept
requires VulkanResource<CommandPoolState>;
```

The assertion is explicit source authority and remains visible in typed MIR. It emits no C runtime machinery.

## 20. Structural satisfaction algorithm

For each concrete assertion the compiler:

1. resolves the named concept
2. resolves the concrete type
3. substitutes the concrete type for `T` in every operation requirement
4. walks prerequisite concepts recursively
5. checks exact-signature matches against existing free functions
6. succeeds silently on full satisfaction or emits one deterministic focused error

## 21. Exact-signature matching rules

- exact function name
- exact parameter count
- exact parameter order
- exact parameter type
- exact borrow versus non-borrow shape
- exact `const` behavior
- exact return type

Wrong-name absence and wrong-signature presence are diagnosed separately.

## 22. Cycle detection

- direct concept cycles are rejected
- indirect concept cycles are rejected
- cycle detection is deterministic and terminates without repeated recursion
- by-value struct / enum layout cycles are also rejected deterministically

## 23. Typed struct representation

Added explicit typed representation for:

- `EVT1StructDecl`
- `Immovable` flag
- ordered fields
- struct construction expressions
- field access
- assignment
- final-storage construction distinction for non-copyable values

## 24. Typed concept representation

Added explicit typed representation for:

- `EVT1ConceptDecl`
- operation requirements
- prerequisite requirements
- concrete assertions
- concept-only MIR nodes

## 25. C11 struct representation

- every struct lowers to one transparent field-ordered `typedef struct`
- ordinary and immovable structs share the same field layout shape
- no runtime metadata, hidden owner cell, or wrapper table is emitted

## 26. Construction lowering

- copyable struct expressions lower through deterministic private constructor helpers
- direct local struct initialization lowers through explicit temporaries and field stores
- ordinary enum construction remains helper-based and deterministic

## 27. Immovable final-storage lowering

`immovable struct` locals lower as:

1. final local declaration
2. source-ordered temporaries for initializer expressions
3. direct field assignments into that final local

No temporary struct value is fabricated and then copied into place.

## 28. Concept compile-time-only lowering

- concept declarations emit no C declaration
- prerequisite edges emit no runtime object
- concrete assertions emit no runtime symbol
- generated C/H contains no vtable, witness, registry, or concept metadata object

## 29. Deterministic naming

- structs: `concept_vulkan_<snake>`
- enums: `concept_vulkan_<snake>` plus explicit tag enum
- generated functions: `concept_vulkan_<source_base>_<snake>`
- constructor helpers: deterministic private `..._make...`
- temporaries: deterministic `cv_<role>_<nn>`

## 30. Complete diagnostic inventory

Added or preserved focused diagnostics covering:

- duplicate structs / fields / enums / variants / concepts
- wrong initializer count or type
- unknown field
- const-path mutation
- ownership-illegal copy
- immovable copy / assignment / by-value parameter / by-value return / embedding / enum payload
- unknown concept / unknown prerequisite
- missing required operation
- wrong operation parameter type
- wrong operation borrow / const qualifier
- wrong operation return type
- concept cycle
- concept used as runtime type
- constrained-template syntax rejected
- preserved M1A match exhaustiveness diagnostics

## 31. Positive compiler matrix

- M1A language specimen parse/generate/check: PASS
- M1A Vulkan-shaped specimen parse/generate/check: PASS
- M1B-A language specimen parse/generate/check: PASS
- M1B-A Vulkan-shaped specimen parse/generate/check: PASS
- deterministic MIR / map / manifest JSON generation: PASS
- native language specimen under strict C11: PASS
- native Vulkan-shaped specimen under strict C11 with real Vulkan headers: PASS

## 32. Negative compiler matrix

Focused negative tests cover:

- duplicate fields
- wrong initializer count
- wrong initializer type
- unknown field
- mutation through const borrow
- ownership-illegal ordinary struct copy
- immovable copy
- immovable whole-value assignment
- immovable by-value parameter
- immovable by-value return
- immovable embedding
- immovable enum payload
- unknown concept
- unknown prerequisite
- missing required operation
- wrong operation parameter type
- wrong operation borrow / const qualifier
- wrong operation return type
- failed nested prerequisite
- direct concept cycle
- indirect concept cycle
- concept used as runtime type
- constrained template rejection

## 33. Language specimen

Added `Examples/Concept-Vulkan/evt1_m1b_a_language.concept`. It proves:

- ordinary structs
- nested structs
- positional construction
- field access
- field mutation
- value-copy independence
- enum payload interoperability
- exhaustive `match`
- immovable final-storage construction
- mutable borrow of an immovable value
- successful concept assertions
- exactly-once ordered initializer execution

## 34. Vulkan-shaped specimen

Added `Examples/Concept-Vulkan/evt1_m1b_a_vulkan.concept`. It proves:

- an ordinary Vulkan-handle range struct
- an immovable Vulkan-handle lifecycle struct
- mutable borrow-based state change
- a composed concept over the Vulkan-shaped immovable type
- explicit cleanup selection through an external operation
- legal enum payload use with the ordinary range struct
- compilation against real Vulkan headers without hardware dispatch

## 35. Native execution results

- hardware-independent specimen: PASS under MSVC strict C11, returned expected ordered initializer evidence, copy-independence evidence, enum/match evidence, and immovable-borrow evidence
- Vulkan-shaped specimen: PASS under MSVC strict C11 plus Vulkan SDK headers, returned expected range classification, immovable validation, and explicit cleanup evidence

## 36. Deterministic-generation result

- repeated in-memory generation is byte-identical
- separate-directory double generation is byte-identical
- checked outputs match fresh generation

## 37. Stale-output rejection

`Check` still rejects hand-edited or stale generated outputs with `CV3001`. The M1B-A test lane edits a generated C file and confirms rejection.

## 38. Source-map and manifest validation

Every generated `.mir.json`, `.map.json`, and `.manifest.json` output is parsed as JSON in tests. Generation remains repository-relative, timestamp-free, and deterministic.

## 39. Public API and ABI preservation

- no public Prometheus headers were edited
- no exported symbols were added or removed
- canonical ABI digest remained `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`

## 40. Production-route preservation

Production remains handwritten. EVT1 outputs stay in `internal/conceptvulkan/generated/` as checked authority and are not wired into Prometheus production routing.

## 41. Shader / package / kernel preservation

- shader source unchanged
- SPIR-V authority unchanged
- package identity unchanged: `prometheus.core@1`
- kernel identities unchanged, including `kernel-68-default` and `kernel-69-default`
- manifests, locks, and native package authority unchanged

## 42. Dominatus boundary

Preserved exactly. No policy, admission, or progression authority moved into Concept/Vulkan.

## 43. SDSL-V boundary

Preserved exactly. No shader-side syntax, generation, or semantics moved into Concept/Vulkan.

## 44. M1A regression result

M1A enum and exhaustive `match` behavior remains green:

- old M1A fixtures still parse and generate
- checked M1A outputs refresh deterministically under the richer compiler
- preserved M1A negative exhaustiveness lane still fails correctly

## 45. RQ-M1 and DVT-2 paused-state preservation

Preserved. No physical batching, DVT-2 optimization, Stage 7, or M1E work was resumed.

## 46. Known limitations

- EVT1 remains a bounded compiler path, not a full general Concept compiler
- no methods, constructors, or destructors
- no template/constrained-template consumer yet
- no compile-time user-function evaluator yet
- no user-visible move syntax beyond the earlier bounded ownership vocabulary

## 47. Unresolved blockers

None for M1B-A. The bounded contract for structs, immovability, named concept requirements, and preserved M1A behavior is complete.

## 48. M1B-A completion assessment

Assessment: **CONCEPT/VULKAN EVT1 M1B-A: SUCCESS**

Ordinary structs, immovable structs, named concepts, prerequisite traversal,
concrete assertions, exact-signature checking, typed MIR, deterministic C11,
strict native compile/run, generated authority checks, and M1A regression
preservation all passed.

## 49. Exact proposed M1B-B assignment

```text
Concept/Vulkan EVT1 M1B-B — Concept-Constrained Templates and Deterministic Monomorphization
```

It should consume the named concept propositions implemented here without redefining their semantics.

## 50. Rollback boundary

Rollback for this pass removes:

- EVT1 struct / concept compiler changes in `internal/conceptvulkan`
- new M1B-A specimens
- new M1B-A generated authority files
- M1B-A documentation and status updates

Rollback does not touch:

- production handwritten routing
- public ABI / exports
- shader or package authority
- Stage 3–6 authorities

## 51. Complete validation matrix

| Lane | Result |
| --- | --- |
| starting checkpoint / branch / clean worktree verification | PASS |
| immovable / ownership terminology audit | PASS |
| `go test ./internal/conceptvulkan` | PASS |
| `go test ./cmd/concept-vulkan ./internal/conceptvulkan` | PASS |
| `go build ./cmd/concept-vulkan` | PASS |
| deterministic repeated generation | PASS |
| deterministic double-directory generation | PASS |
| checked outputs match | PASS |
| stale-output rejection | PASS |
| source-map / manifest / MIR JSON validation | PASS |
| strict C11 language specimen compile+run | PASS |
| strict C11 Vulkan-shaped specimen compile+run | PASS |
| native compiler authority | PASS — `cl.exe` from Visual Studio 18 Community via `VsDevCmd.bat` |
| Vulkan SDK authority | PASS — `C:\VulkanSDK\1.4.350.0` |
| focused Prometheus authority check (`go run ./tools/prometheus_stage0 -check`) | PASS |
| public-header comparison | PASS — no public-header files changed |
| export comparison | PASS — 84 exports unchanged |
| ABI digest comparison | PASS — `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262` |
| production-route preservation | PASS |
| shader/package/manifest/lock/kernel preservation | PASS |
| `git diff --check` | PASS |
