# Concepts M1 — refined concepts and checked construction

## Verdict

**SUCCESS — REFINED CONCEPTS SUPPORTED WITH STATIC PROOF AND CHECKED CONSTRUCTION.**

The decisive path works: Oct can state a stable subset law once, reject a known bad member during checking, require a visible fallible constructor for an unknown value, preserve the admitted identity through ordinary code, and emit the original concrete representation. There is no runtime concept object.

## Definition and grammar

A refined concept is a nominal static subset of one existing representation. The adopted grammar is:

```oct
concept Name = BaseType {
    Require(Self predicate literal[, "stable explanation"])
    Require(...)
}
```

This remains distinct from the M0 forms `concept Length = Float<m>` (transparent alias) and `concept Position { X: Length Y: Length }` (nominal record). `Self` is bound to the candidate only in refinement requirements. Requirements are stored once as typed AST expressions and reused for static proof and the generated checked boundary.

## Construction and proof rules

- `let c: ColorChannel = 255` and bounded constant expressions are proved at compile time. Failure identifies the concept, source expression, failed law, and explanation.
- An unknown `Int` cannot flow into `ColorChannel` by assignment, argument, return, record field, array element, or update.
- `ColorChannel(value)?`, `!`, or `match` performs the explicit runtime check and returns `ColorChannel ! Error`.
- Direct binding, exact arguments/returns, records, arrays, indexing, and immutable record updates preserve proof.
- A refined scalar may flow to its base representation without conversion. Arrays are invariant.
- Equality/comparison use the base representation. Arithmetic and numeric unary operations lose refinement.
- Transparent aliases of a refinement preserve its identity. A transparent alias may be the base of a refinement. Refinement-on-refinement and cycles are rejected.
- Dimensioned numeric refinements retain dimensions. Nominal records, enums, vectors, matrices, functions, flows, `Error`, `Void`, `Range`, `UI`, and `Bytes` are unsupported bases in M1.

The purity boundary permits literals, `Self`, bounded Boolean/numeric operators, comparisons, and `Len(Self)` for arrays. It rejects arbitrary calls, mutation, I/O, process/environment/network access, time/randomness, host calls, reflection, and compiler objects.

## Lowering and erasure

Expansion synthesizes one package-local ordinary fallible function named internally as `__oct_refine_Name`. The function evaluates the declaration's requirement AST and returns the unwrapped representation on success. User syntax is rewritten to that function before checking and lowering. A compiler-only marker permits only this function's final base value to acquire the refined return identity.

MIR keeps the refined type on storage and signatures but contains a constructor call only where source used explicit checked construction. Go emits `type Package_Name = concrete`, so allocation and wrapper counts are zero. The `examples/ConceptsM1` evidence has one static `255`, one runtime source, and two downstream consumers: MIR contains one constructor call; the static binding and both consumers contain no requirement text or branch. Generated Go has one `int` alias and exactly two requirement-failure branches inside the explicit constructor.

## Primary specimen: UI color

Before M1, `Rgba(Int, Int, Int, Int)` called `ValidateChannel` four times and passed four strings such as `"Rgba.r"`; each use repeated two range branches. M1 declares:

```oct
concept ColorChannel = Int {
    Require(Self >= 0, "a color channel must be at least 0")
    Require(Self <= 255, "a color channel must be at most 255")
}

fn Rgba(r: ColorChannel, g: ColorChannel, b: ColorChannel, a: ColorChannel) -> Color {
    return Color { R: r G: g B: b A: a }
}
```

Known calls retain their behavior and emit no checks. Runtime values use `ColorChannel(raw)?` once; an admitted channel can be reused for every component without revalidation. `ValidateChannel` and its seven call-site label/check ceremonies are gone. `Libraries/UI` passes 57/57 contracts (24 compiled, 29 expected UI-builtin fallbacks, four invalid contracts), including runtime failure and invalid literal diagnostics.

`NonNegativePx = Int<px>` is a second UI shape and replaces `ValidatePxNonNegative` at three call sites.

## Secondary production specimen: dimensional geometry

Six radius/width/height/base helpers duplicated the same `Float<m> >= 0` law across 14 calls. They are replaced by `NonNegativeLength = Float<m>`. Planar and solid functions accept the refined value, derived runtime distances cross one explicit `NonNegativeLength(d)!` boundary, and admitted values are reused across planar and solid functions. Geometry passes 10/10 compiled contracts with no fallback.

## Repository-wide validation audit

The executable-name scan covered `.oct`, `.octest`, `.octfail`, and non-Prometheus/non-SDSL-V Go host code; bodies and call sites were then classified semantically. Before conversion it found 61 production Oct helpers and 16 Go helpers, with 191 Oct and 14 Go call sites (205 total). Five test names beginning with `Validate`/`Checked` were false positives and are not counted as production helpers. Inline guards around the same libraries were reviewed with their owning helper family.

| Helper or pattern | Location(s) | Calls before | Behavior | Class | Decision / reason |
|---|---:|---:|---|---:|---|
| `ValidateChannel` | `Libraries/UI/UI.Style.oct` | 4 | one `Int`, fixed 0–255 law | 1 | converted to `ColorChannel` |
| `ValidatePxNonNegative` | `Libraries/UI/UI.Style.oct` | 3 | one `Int<px>`, fixed lower bound | 1 | converted to `NonNegativePx` |
| six `ValidateNonNegative*` geometry helpers | `Libraries/Geometry/Geometry.Planar.oct`, `Geometry.Solids.oct` | 14 | one `Float<m>`, identical non-negative law | 1 | converted to shared `NonNegativeLength` |
| `ValidateBlockSize` / `ValidateKBlockSize` (five milestone copies) | `Experiments/PrometheusSgemmAlgorithmLab/M1,M2,M4` | 12 | positive/divisibility experiment knobs | 1 | retained; milestone evidence is historical and parity/divisibility laws differ |
| derivative/integration/Simpson/FFT length validators | `Libraries/Mathematics` | 6 | positivity plus Simpson parity | 1 | retained; low repetition and parity-specific error behavior |
| `ValidateStep` | `Libraries/Tensor2D` | 2 | positive finite-difference step | 1 | retained; one local boundary, no repeated identity consumer |
| radius/height/level/area/dt/gravity/coefficient/thermal-area helpers | `Libraries/Thermofluids` | 11 | several distinct positivity/non-negativity laws | 1 | retained; laws and error contexts are not one reusable identity |
| `ValidateSeries`, `ValidatePairedSeries`, `ValidateStrictlyIncreasing` | `Libraries/Analysis` | 24 | non-empty, paired lengths, ordered sequence | 2/3 | retained; collection structure and cross-element relations |
| `ValidateStepInputs`, `ValidateSolveInputs` | `Libraries/DifferentialEquations` | 5 | coupled time/step/count laws | 3 | retained; cross-value invariant |
| SGEMM input/shape helpers (eight milestone copies) | `Experiments/PrometheusSgemmAlgorithmLab/M0,M1,M2,M4` | 32 | matrix shape and A/B compatibility | 2/3 | retained; structural and relational historical specimens |
| `ValidateGiteaInstanceConfig` | `Libraries/Deployment` | 7 | whole external configuration | 2 | retained; aggregate boundary errors matter |
| `ValidatePiecewiseInputs` | `Libraries/Interpolation` | 4 | paired lengths and ordering | 3 | retained |
| LU/vector/matrix/square validators | `Libraries/LinearAlgebra` | 31 | shape, stored-data length, factor consistency | 2 | retained; structural construction validation |
| `ValidateFFTInput` | `Libraries/Mathematics.Transforms` | 1 | collection shape plus length delegation | 2 | retained |
| bisection/Newton validators | `Libraries/Numerics.Roots` | 2 | tolerance/count and bracket relations | 3 | retained |
| `IsValidMimoChannel`, `ValidateSParameters2Port` | `Libraries/RF` | 16 | record shape, trace lengths, monotonic axes | 2/3 | retained; multi-field and cross-element laws |
| fixed-step/trace validators | `Libraries/Simulation` | 4 | coupled setup and trace shape | 2/3 | retained |
| utility feature/evidence validators | `Libraries/Statistics.UtilityFit` | 2 | names, dimensions, observation consistency | 2 | retained |
| point/vector-field-output validators | `Libraries/Tensor2D` | 6 | vector dimensional shape | 2 | retained |
| thermal mass/cp and tau/dt helpers | `Libraries/Thermofluids` | 3 | relations between independent quantities | 3 | retained |
| `CheckTools*` helpers | `examples/Chimera*` | 2 | tool availability / make target | 6 | retained; operational failure, not membership |
| name/version/manifest/wrapper/lock/registry validators | `internal/newpkg`, `manifest*`, `pkgmgr` | 9 | external metadata and graph consistency | 4 | retained at host boundary |
| request/response/value validators | `internal/octxiliary` | 1 direct plus nested calls | transport and handle graph validation | 4 | retained; external protocol boundary |
| OctGen artifact validation/check helpers | `internal/octgen`, `experimental/octgen` | 3 | path safety and generated-output freshness | 4/6 | retained; external artifact boundary / operational check |
| `typecheck.Check`, `CheckProgram` | `internal/typecheck` | orchestration entry points | compiler checking, not a value validator | 10 | not validation |

Classification totals (all 77 production helper declarations): class 1 = 26, class 2 = 17, class 3 = 15, class 4 = 12, class 5 = 0, class 6 = 5, class 7 = 0, class 8 = 0, class 9 = 0, class 10 = 2. Eight helpers were converted; 69 were retained. The retained majority is structural, relational, external, or too low-repetition to justify a new domain identity.

## Diagnostics

M1 adds focused errors for malformed/empty refinement bodies, unknown/invalid bases, duplicate declarations, cycles, illegal names in requirements, non-Boolean requirements, impure/unsupported expressions, known failed construction, unproven implicit base-to-refined flow, invalid refined returns/fields/array elements, and runtime requirement failure. Existing parser source positions and function/binding context remain in the diagnostic chain; requirement explanations are stable source text and no Go detail is exposed.

## Compatibility and tests

M0 transparent aliases, record-shaped concepts, unit aliases, compiler-owned `Require`, records, enums, fallibility, and concrete Go lowering remain intact. Focused evidence:

- parser, concept, typechecker, and integration backend Go suites pass;
- `Language/Types/ConceptsM1`: ten invalid contracts pass; the valid file runs two compiled facts with no fallback;
- UI: 57/57 pass; Geometry: 10/10 compiled;
- generated example executable exits 0; its MIR and retained generated Go prove one explicit check boundary and zero downstream checks;
- M0 Concepts, Units, enum/exhaustiveness, OctGen, formatter, interpreter, ordinary Go, and diff hygiene are part of the final command ledger.

Final command ledger:

- `go test ./...`: pass across all Go packages;
- `go test ./internal/typecheck ./internal/build`: pass;
- `go test -tags=integration ./internal/build -run TestRefinedConceptStaticProofErasesAndCheckedConstructionIsSingleBoundary -count=1`: pass;
- `oct test Language/Types/ConceptsM1`: 10/10 invalid contracts pass; direct valid-file run: 2/2 compiled, no fallback;
- `oct test Libraries/UI`: 57/57 pass (24 compiled, 29 pre-existing UI-builtin fallbacks, four invalid);
- `oct test Libraries/Geometry`: 10/10 compiled, no fallback;
- M0 Concepts: 12/12 invalid plus 1/1 compiled valid; Units M1: 5/5 invalid plus 1/1 compiled valid; associated enums: 10/10 invalid plus 6/6 compiled valid;
- `go test ./internal/octgen ./experimental/octgen`: pass;
- `oct fmt <modified production-or-valid-file> --check`: pass for all nine applicable files; the deliberately malformed parser fixture is excluded;
- `git diff --check`: pass.

## Measurements

- validation helpers discovered/audited: 77 production declarations; 205 call sites;
- converted: 8 helpers; retained: 69;
- repeated validation calls removed: 21;
- manual error-label strings removed: 7;
- refined declarations added: 3 production (`ColorChannel`, `NonNegativePx`, `NonNegativeLength`) plus language/example specimens;
- explicit checked constructions added: six focused runtime/specimen boundaries; production downstream validation after admission: zero;
- runtime checks on the converted repeated paths: 25 branches/checks before, only the declared requirements at explicit construction afterward; known production literals emit zero;
- generated example: 4,949-byte Go source, one alias, one constructor call, two requirement branches, zero wrappers/allocations/registries;
- wrappers/allocations introduced: 0;
- compile-time overhead: one bounded evaluation per known admission; no whole-program inference or solver;
- runtime overhead: linear in the number of requirements only at explicit checked boundaries;
- implementation diff before documentation: AST/parser +77/−6, concept expansion +149/−72, typechecker +412/−44, MIR/backend +37, backend/parser tests +62; lexer, formatter, and interpreter code changed 0 because existing tokens/surfaces and synthesized ordinary functions are reused;
- accepted UI/Geometry outputs and values are stable; failure context now names the concept law instead of call-site label strings.

## Files changed

- compiler: `internal/ast/program.go`, `internal/parse/parse.go`, `internal/concept/expand.go`, `internal/typecheck/typecheck.go`, `internal/build/compiler.go`;
- Go tests: `internal/parse/parse_test.go`, `internal/build/compiler_test.go`;
- language contracts: all 11 files under `Language/Types/ConceptsM1`;
- production/specimen code: `Libraries/UI/UI.Style.oct`, `Libraries/Geometry/Geometry.Refinements.oct`, `Geometry.Planar.oct`, and `Geometry.Solids.oct`;
- production tests: `Libraries/UI/UI.ConceptsM1.octest`, `Libraries/UI/UI.ConceptsM1InvalidChannel.octfail`, `Libraries/Geometry/Geometry.Planar.octest`, and `Geometry.Solids.octest`;
- evidence and documentation: `examples/ConceptsM1/main.oct`, `Language/reference/language/18-concepts.md`, and `CONCEPTS_M1.md`.

## Limitations and rejected alternatives

M1 does not infer intervals, preserve proof through arithmetic, solve symbolic propositions, refine nominal records, stack refinements, expose runtime descriptors, or implement behavioral concepts/generics/templates. It does not silently insert validation. Package-qualified refined types and static calls work, but package-qualified checked-constructor syntax is deferred; declaring packages can expose checked APIs where needed. Arrays are invariant. Vector/matrix refinements and aggregate error collection remain ordinary validation.

A runtime registry, wrapper structs, validation on every use, a second compile-time VM, a general theorem prover, and conversion of relational/external validators were rejected because they violate erasure, explicit fallibility, boundedness, or honest error aggregation.

## Recommendation

Next: **stabilize refinements**, then add package-qualified checked-constructor ergonomics. Expand bounded requirement expressions only from real validator evidence. Do not begin behavioral concepts until one real conformance/specialization consumer exists.
