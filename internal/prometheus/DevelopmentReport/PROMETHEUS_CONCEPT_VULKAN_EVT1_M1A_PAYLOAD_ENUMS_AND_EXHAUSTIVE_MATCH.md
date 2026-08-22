# Concept/Vulkan EVT1 M1A — payload enums and exhaustive match

Status: **SUCCESS**

Expected commit for this pass: `concept-vulkan: add payload enums and exhaustive match`

## 1. Starting checkpoint and worktree state

- starting checkpoint: `70c9ec5a40b03333d367ae32a6a042d7ddc610c3`
- starting worktree: clean porcelain
- starting branch: `main`
- repository state at start: one local commit ahead of `origin/main`, no uncommitted files

## 2. Inspected compiler and PoC authority

Inspected before implementation:

- `README.md`
- `docs/Concept-Vulkan/CONCEPT_VULKAN_LANGUAGE_CONSTITUTION.md`
- `docs/OCT_SDSLV_PROMETHEUS_LIVING_STATUS.md`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_G4_E2B_M1_REVIEWER_HANDOFF.md`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_DEVELOPMENT_EVIDENCE_INDEX.json`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_M0_CHARACTERIZATION.md`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_M1_KERNEL54_COMPILER_VERTICAL.md`
- `internal/prometheus/DevelopmentReport/PROMETHEUS_CONCEPT_VULKAN_M1D_EXECUTABLE_EQUIVALENCE_CLOSURE.md`
- `internal/conceptvulkan/conceptvulkan.go`
- `internal/conceptvulkan/conceptvulkan_test.go`
- `cmd/concept-vulkan/main.go`
- `Examples/Concept-Vulkan/kernel54_probe.concept`
- `primer/cpp-primer.md`
- `primer/vulkan-primer.md`

## 3. Scope and non-goals

This pass adds payload enums and exhaustive `match` to the bounded
Concept/Vulkan compiler path. It does not resume M1E, does not route
production through generated EVT1 outputs, and does not add templates,
concepts, compile-time evaluation, wildcard patterns, or a new ownership
model.

## 4. Owner transition from PoC to EVT1

- the original kernel-54 compiler/native proof is accepted as sufficient proof
  of the basic vertical;
- M1D remains honestly classified as `MEANINGFUL PROGRESSION`;
- the proposed M1E handwritten create-path failure seam is preserved as design
  history, not the active assignment;
- EVT1 is active;
- EVT1 M1A is payload enums and exhaustive `match`;
- EVT1 M1B is concepts-first templates and compile-time evaluation, deferred;
- Prometheus RQ-M1 physical batching remains preserved but paused;
- DVT-2 optimization remains preserved but paused.

## 5. Exact grammar added

Added a new EVT1 parser path with:

- `enum Name { Variant, Variant(Type payload), Variant(Type first, Type second) }`
- qualified variant construction `EnumName::Variant(...)`
- qualified match patterns `EnumName::Variant(...)`
- expression-form `match (subject) { Pattern => expression, ... }`
- statement-form `match (subject) { Pattern => { ... } ... }`
- Rust-style `=>`
- existing C++-shaped `ReturnType Function(Type name)` declarations
- function prototypes ending with `;`

Still rejected:

- `fn`, `let`, `var`, `name: Type`, `-> ReturnType`
- wildcard arms
- guards
- block-valued expression arms
- non-block statement arms
- nested destructuring beyond one variant payload list

## 6. Enum declaration semantics

EVT1 M1A supports mixed unit and payload variants in one enum. Variant tags are
assigned deterministically in source declaration order beginning at zero.
Variant payload fields remain ordered, named, and typed.

## 7. Variant construction semantics

Construction resolves the enum first, then the named variant, then checks
payload arity and payload types left-to-right. Unit variants reject arguments,
payload variants require arguments, and all constructors remain qualified by
their enum type.

## 8. Payload typing and evaluation order

Payload expressions are type-checked against the declared positional payload
types. C lowering materializes constructor and call arguments into explicit
temporaries before use, preserving source order and exactly-once evaluation.

## 9. Match statement semantics

Statement-form `match`:

- accepts enum subjects only;
- requires each arm body to be a braced block;
- binds payload names only inside the selected block;
- rejects duplicate arms and non-exhaustive coverage;
- lowers to an explicit `switch` with case-local scopes and no fallthrough.

## 10. Match expression semantics

Expression-form `match`:

- accepts enum subjects only;
- requires each arm to be one expression;
- rejects block arms;
- requires all arm result types to match exactly under the existing bounded
  Concept/Vulkan rules;
- lowers to a subject temporary plus a typed result temporary assigned exactly
  once in each successful case.

## 11. Exhaustiveness rules

The compiler computes exhaustiveness from the declared variant set of the
subject enum. Missing variants are reported deterministically in declaration
order as `Enum::Variant`.

## 12. Payload-binding scope

Payload bindings are introduced only within the matching arm scope. Duplicate
binding names inside one pattern are rejected. A use of a payload binding after
its arm exits receives a dedicated diagnostic (`CV4114`).

## 13. Typed representation

Added explicit EVT1 AST and MIR representations in `internal/conceptvulkan`:

- `EVT1EnumDecl`, `EVT1VariantDecl`, `EVT1Pattern`
- `EVT1ConstructExpr`, `EVT1MatchExpr`, `EVT1MatchStmt`
- deterministic `EVT1MIR`, `EVT1MIREnum`, `EVT1MIRFunction`, and
  `EVT1MIROperation`

The MIR text now makes enum identity, variant order, constructor nodes, match
nodes, and pattern nodes inspectable without reading generated C.

## 14. C11 enum representation

Each enum lowers to:

- one explicit tag enum;
- one wrapper struct containing that tag;
- one payload union;
- one struct member per payload-bearing variant;
- one `none` member kept uniformly present for unit variants and all-unit enums.

## 15. Constructor lowering

Each variant gets a deterministic private constructor helper:

- `concept_vulkan_<enum>_make_<variant>(...)`

The caller evaluates payload expressions into temporaries first, then calls the
helper. No helper is exported through public Prometheus headers.

## 16. Match lowering

Both statement-form and expression-form `match` lower through ordinary C11
`switch` dispatch on the tag field. Payload reads are emitted only in the
matching case scope. Expression matches use an explicit result temporary; no
fabricated semantic default is introduced.

## 17. Subject single-evaluation proof

Generated C binds each match subject into a dedicated temporary before the
`switch`. `TestEVT1LanguageSpecimenNativeC11` proves this behavior with the
`ObserveState()` side effect: `ObserveAndClassify()` returns the expected value
and `ObserveCalls()` reports exactly `1`.

## 18. Expression-result lowering

Expression matches allocate a typed `cv_match_result_*` local and assign it
inside each case before `break`. The final C expression context consumes the
temporary after the switch.

## 19. Invalid-tag behavior

Added one private deterministic invalid-state path:

- `concept_vulkan_abort_invalid_tag(const char* enum_name)`

It reports the enum name to `stderr` and terminates via `abort()`. EVT1 M1A
does not silently select an arm or invent a zero result for invalid tags.

## 20. Deterministic naming and tag assignment

- tags: declaration-order, zero-based
- C enum/struct names: `concept_vulkan_<snake>`
- generated function symbols: `concept_vulkan_<source_base>_<snake>`
- constructor helpers: `concept_vulkan_<enum>_make_<variant>`
- temporaries: `cv_<role>_<nn>`

## 21. Complete diagnostic inventory

Added focused diagnostics covering:

1. `CV4100` duplicate variant declaration
2. `CV4102` unknown enum/type in construction or pattern
3. `CV4103` unknown variant for a known enum
4. `CV4104` missing payload args or unit-variant args
5. `CV4106` wrong constructor/call payload count
6. `CV4107` wrong payload type
7. `CV4108` non-enum match subject
8. `CV4109` wrong-enum pattern
9. `CV4110` unknown match-arm variant
10. `CV4111` payload binding count mismatch
11. `CV4112` unit bindings or duplicate binding names
12. `CV4113` duplicate match arm
13. `CV4114` payload binding escape
14. `CV4115` non-exhaustive match
15. `CV4116` incompatible expression-arm result types
16. `CV4117` expression-form block arm
17. `CV4118` statement-form arm missing block
18. `CV4119` unsupported owned payload

The EVT1 parser also retains bounded grammar diagnostics in the `CV4000`-range.

## 22. Positive compiler matrix

- parse the language specimen: PASS
- parse the Vulkan-shaped specimen: PASS
- lower typed MIR for both: PASS
- generate deterministic C/H/MIR/map/manifest for both: PASS
- check checked-in outputs: PASS
- native strict-C11 compile and execute both specimens: PASS

## 23. Negative compiler matrix

`TestEVT1DiagnosticsAreStable` covers duplicate variants, unknown enums,
unknown variants, unit/payload constructor misuse, payload count/type mismatch,
wrong-enum patterns, non-enum subjects, duplicate arms, non-exhaustive
matches, mismatched expression results, statement/expression arm-shape errors,
binding leakage, and owned payload rejection.

## 24. Native language specimen

Added `Examples/Concept-Vulkan/evt1_m1a_language.concept` plus generated
authority under `internal/conceptvulkan/generated/`. It proves:

- unit, one-payload, multi-payload, and enum-payload variants
- construction of every variant
- statement-form and expression-form `match`
- nested `match`
- observable subject single-evaluation

## 25. Vulkan-shaped specimen

Added `Examples/Concept-Vulkan/evt1_m1a_vulkan.concept` plus generated
authority under `internal/conceptvulkan/generated/`. It proves:

- `Empty`
- `LayoutCreated(PipelineLayout layout)`
- `Ready(PipelineLayout layout, Pipeline pipeline)`
- `Failed(VulkanError error)`
- reverse cleanup order for the ready state
- status extraction via expression-form `match`

## 26. Native execution results

`TestEVT1LanguageSpecimenNativeC11` and `TestEVT1VulkanSpecimenNativeC11`
passed under an initialized MSVC environment. The language specimen validated
correct arm selection, payload order, nested matches, and single subject
evaluation. The Vulkan-shaped specimen validated cleanup order
`DestroyPipeline -> DestroyPipelineLayout` and propagated the failure code.

## 27. Generated-authority result

Generated authority is checked in under:

- `internal/conceptvulkan/generated/evt1_m1a_language.*`
- `internal/conceptvulkan/generated/evt1_m1a_vulkan.*`

`TestEVT1CheckedOutputsMatch` proves the checked outputs match freshly
generated outputs.

## 28. Deterministic double-generation result

`TestParseEVT1SpecimensAndGenerateDeterministically` proves repeated in-memory
generation is byte-identical. `TestEVT1DoubleGenerationMatchesAcrossDirectories`
proves writing two separate directories yields byte-identical files.

## 29. Stale-output rejection

`TestEVT1CheckRejectsHandEdit` edits a generated C file and verifies `Check`
fails with `CV3001`.

## 30. Source-map and manifest validation

`TestParseEVT1SpecimensAndGenerateDeterministically` parses every generated
`.mir.json`, `.map.json`, and `.manifest.json` file as JSON and fails on any
malformed output.

## 31. Public API and ABI preservation

No public header or public export was changed. EVT1 M1A touched only the
bounded compiler, examples, generated specimen outputs, and repository
documentation.

## 32. Production-route preservation

Production still routes through handwritten Prometheus native code. The new
generated EVT1 outputs live under `internal/conceptvulkan/generated/` and are
not listed as production native sources.

## 33. Shader/package/kernel preservation

No shader source, SPIR-V, package identities, kernel entries, manifests, or
locks changed. `go run ./tools/prometheus_stage0 -check` still reports:

- package identity `prometheus.core@1`
- 84 exported symbols
- canonical signature digest `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`

## 34. Dominatus boundary

Preserved. No policy, admission, judgment, authorization, or progression moved
into Concept/Vulkan.

## 35. SDSL-V boundary

Preserved. No shader-side language or package behavior moved into
Concept/Vulkan.

## 36. RQ-M1 and DVT-2 paused-state preservation

Preserved. This pass does not resume physical batching or DVT-2 optimization.

## 37. Known limitations

- EVT1 M1A remains a bounded compiler path, not a general Concept compiler.
- Match patterns remain one-level qualified variant patterns only.
- Expression-arm type unification remains exact-type equality in this bounded
  implementation.
- Invalid-tag handling is a private abort path, not a recoverable runtime
  status.

## 38. Unresolved blockers

None for M1A. The feature vertical requested by EVT1 M1A is complete within
its bounded assignment.

## 39. M1A completion assessment

Assessment: **CONCEPT/VULKAN EVT1 M1A: SUCCESS**

Mixed unit/payload enums, qualified construction, exhaustive statement and
expression matches, scoped payload bindings, explicit MIR, deterministic C11,
strict C11 native compilation, executable proof, checked-output validation,
and production-boundary preservation all passed.

## 40. Exact proposed M1B assignment

```text
Concept/Vulkan EVT1 M1B — concepts-first templates and compile-time evaluation
```

This remains deferred until after the completed M1A payload-enum/match
vertical.

## 41. Rollback boundary

Rollback removes:

- `internal/conceptvulkan/evt1_*`
- `Examples/Concept-Vulkan/evt1_m1a_*.concept`
- `internal/conceptvulkan/generated/evt1_m1a_*`
- this report and the related constitution/status/evidence-index/handoff
  updates

Rollback does not touch:

- production handwritten routing
- public ABI or exports
- shader/package authority
- Stage 3–6 authorities

## 42. Complete validation matrix

| Lane | Result |
| --- | --- |
| starting checkpoint / clean worktree verification | PASS |
| compiler package tests (`go test ./internal/conceptvulkan`) | PASS |
| CLI/package compile lane (`go test ./cmd/concept-vulkan ./internal/conceptvulkan`) | PASS |
| checked specimen outputs (`TestEVT1CheckedOutputsMatch`) | PASS |
| deterministic repeated generation | PASS |
| deterministic double-directory generation | PASS |
| stale-output rejection | PASS |
| source-map / manifest / MIR JSON parse | PASS |
| strict C11 language specimen compile+run | PASS |
| strict C11 Vulkan-shaped specimen compile+run | PASS |
| native toolchain authority | PASS — MSVC `cl /std:c11` via `VsDevCmd`, Vulkan SDK `C:\VulkanSDK\1.4.350.0` |
| public-header comparison | PASS — no `internal/prometheus/native/include` or `reactor_api.h` changes |
| export comparison | PASS — 84 exports unchanged |
| ABI digest comparison | PASS — `89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262` |
| production-route preservation | PASS |
| shader/package/manifest/lock/kernel preservation | PASS |
| focused Prometheus audit (`go run ./tools/prometheus_stage0 -check`) | PASS |
| native Vulkan runtime / hardware dispatch for EVT1 M1A | NOT RUN — language-only specimens compile against headers but do not submit GPU work |
| `git diff --check` | PASS |
