# Template Torture Lab M0

## Outcome

**Outcome B: robust typed-specialization core with one major bounded limitation.**

Oct templates are a practical typed metaprogramming substrate for ordinary scientific code: cross-file and cross-package composition, nested records, higher-order functions, explicit captures, FLOW, arrays/vectors/matrices, dimensional values, refined Concepts as concrete arguments, deterministic deduplication, and interpreted/compiled parity all work through early specialization. The major limitation is constraint expressiveness: current Concepts describe values/refinements, not behavioral/operator capabilities of an arbitrary `T`, and the type grammar cannot name generic transformed dimensions such as `T^2`. M0 does not add traits, reflection, const generics, or macros to route around that boundary.

## Cross-file root cause and repair

The elaborator already collected all declarations before rewriting, but explicit `.octest` loading removed unselected sibling test sources before template collection. A template in `uses_double_crossfile.octest` therefore specialized while `Double2` was absent and left an ordinary call node with type arguments; downstream typechecking misleadingly reported that `Double2` did not accept type arguments.

Test selection now controls execution, not template authority. Same-package template-bearing and declaration-only support sources join the selected compilation unit, while unrelated standalone `.octest` programs that intentionally reuse `package Main` remain isolated. Directory/package loading continues to merge the entire package normally. Regressions cover function-to-function, function-to-record, FLOW-to-function, record use from a template function, forward references, a three-file chain, explicit-file invocation, and a three-package A -> B -> C chain.

## Correctness and safety repairs

- Runtime recursion now reuses the in-progress semantic specialization key. `Factorial<Int>` emits once and recurses normally at runtime.
- Ever-growing specialization receives a maximum depth of 128 and a maximum unique-specialization budget of 4096. The diagnostic includes the complete chain; the compiler does not hang, overflow the Go stack, or OOM.
- Statistics now expose requests, reuse, and maximum depth in addition to unique record/function/FLOW counts and elapsed elaboration time.
- A post-elaboration structural invariant rejects residual template declarations/type-parameter lists, unresolved open parameters, and calls to known templates before downstream phases.
- `TemplateOrigin` retains the outer-to-inner instantiation chain. Typechecking errors include that chain, concrete arguments, and the failing template function source path.
- Complex `Real`/`Imag` helper emission now imports `math`; torture testing found the prior generated-Go compile failure.
- FLOW persistence now follows refined Concept aliases to their concrete scalar representation, so a specialized `Box<PositiveLength>` board value is admitted consistently with ordinary refined values.

## Executable corpus

Permanent fixtures live at `Language/Types/TemplateTortureM0/`:

- `valid/TemplateTortureValid/`: seven same-package files covering cross-file/forward/nested composition, records, functions, FLOW, captures, generic consumers, dimensions, Concepts, recursion, ODE, linear algebra, optimization-shaped objectives, statistics accumulators, and Complex signal transforms.
- `packages/`: imported template composition across `TemplateOrigin` -> `TemplateBridge` -> `Main`.
- `invalid/`: dimensional transformation, nonnumeric conditional invalidity, ODE dimensional mismatch, refined-Concept admission, infinite type growth, type-parameter-as-value misuse, wrong type arity, ordinary-function type arguments, and unsupported template enums.
- `provenance/`: nested cross-file specialization failure used by compiler diagnostic regression tests.

Dormant invalidity is intentionally accepted: `DormantInvalidForSomeTypes<T>` is present but not instantiated. The same operation fails only when a bad concrete application is requested.

## Template power matrix

| Capability | Works? | How | Failure mode / boundary | Needed? |
|---|---:|---|---|---:|
| Generic functions | Yes | Explicit concrete type arguments and early cloning | Bad operations fail concrete typecheck | Yes |
| Generic records | Yes | Nominal specialization per semantic key | No structural field reflection | Yes |
| Generic FLOW/query | Yes | Concrete FLOW emitted before lowering | No runtime generic FLOW metadata | Yes |
| Cross-file composition | Yes | Package-wide template collection/support sources | Unrelated standalone test programs remain isolated | Yes |
| Cross-package composition | Yes | Consumer-owned specialization with qualified origin | Template-private dependencies must be visible | Yes |
| Nested templates | Yes | Recursive type/call rewriting | Depth/count guard for type growth | Yes |
| Template calls template | Yes | Same semantic lookup for same/cross file/package | Wrong arity is direct error | Yes |
| Function-valued parameters | Yes | Exact `fn(...) -> ...` substitution | Signature mismatch rejected | Yes |
| Template returns function | Yes | Ordinary anonymous function after substitution | Illegal operator fails specialization | Yes |
| Explicit/escaped captures | Yes | Capture environment stays ordinary concrete code | Missing capture uses ordinary diagnostic | Yes |
| Concept constraints | Partial | Refined Concept works as a concrete type argument | No behavioral/operator bound over arbitrary `T` | Yes, later evidence needed |
| Dimensioned types | Yes | Ordinary dimensional checking after specialization | Generic transformed result type not nameable | Yes |
| Payload enums | Partial | Concrete payload enums can appear in template records/functions | `template enum` unsupported | Useful, not required now |
| Vectors/matrices | Yes | Specialized element type plus ordinary tabulation | Operators remain unconstrained until specialization | Yes |
| Recursive runtime calls | Yes | In-progress key resolves to same concrete function | Runtime termination remains program responsibility | Yes |
| Recursive specialization | Bounded | Repeated key reuses; new-key growth guarded | 128 depth / 4096 unique budget | Defensive necessity |
| Type reflection | No | None by design | Cannot branch on exact `T` | No M0 need |
| Field reflection | No | Typed `Selector<R,F>` only, no enumeration | Cannot derive serializers/methods | No current scientific need |
| Value parameters | No | Runtime fields/arguments, vectors/matrices carry runtime extent | No `Matrix<T,R,C>` / `Array<T,N>` | Potential later correctness benefit |
| Variadics | No | Arrays/records model heterogeneous or repeated inputs | No parameter packs | No observed need |
| Higher-kinded types | No | Templates accept concrete types, not `F<_>` | Container-generic abstraction unavailable | No concrete corpus demand |
| Syntax generation | No | Declaration cloning only | No token/AST emission | Not needed |

## Scientific pressure results

### Statistics and dimensions

One `Mean<T>` specializes for `Float<m>`, `Float<s>`, and the corpus also checks midpoint specializations for temperature, pressure expressed as `kg/m/s^2`, and dimensionless `Float`. No unit annotations occur inside the algorithm. `SquareSameType<Float<m>>` fails because the body produces `Float<m^2>` while the declared result is `Float<m>`. Oct cannot currently spell a generic `T^2` return relation; that is an expressiveness limit, not a reason to erase dimensions. (`Pa` is not currently a recognized source unit alias.)

`Fold<T,A>` is exercised with `T = Float<m>` and `A = LengthStatistics`, as well as ordinary numeric cases. Exact reducer signatures and record/dimensional accumulator identity survive specialization.

### Differential equations

`EulerStep<State,Derivative,Time>` accepts `Float<m>`, `Float<m/s>`, and `Float<s>` and produces `Float<m>` through ordinary dimension algebra. A deliberately wrong derivative type fails at specialization (`cannot add m and m*s`). The relationship can be expressed with three concrete parameters, but M0 cannot constrain those parameters universally at declaration time.

### Linear algebra, optimization, and signal

Generic vector tabulation preserves `Vector<Float<m>>`; template records hold arrays, vectors, and matrices. A template adapter consumes a captured `fn(Candidate) -> Float` optimization objective. Generic mapping supports both dimensioned scalars and `Complex`; the Complex case exposed and fixed the missing generated-Go `math` import.

The existing compiled suites were also run as real-world pressure: Statistics 47/47, DifferentialEquations 11/11, Optimization 44/44, Signal 38/38, SymbolicRegression 8/8, and Algorithms 13/13, all with zero interpreted fallback. SymbolicRegression did not reveal a bounded refactor worth forcing; its current concrete feature records and functions do not demonstrate demand for reflection or higher-kinded types.

## Rust comparison

### Oct templates vs Rust generics

Both produce concrete typed behavior and reject invalid concrete operations. Rust has trait bounds, associated types, const generics, and richer inference; Oct currently has explicit type arguments, exact function types, value/refinement Concepts, dimensions, and early source-level monomorphization. Oct's missing behavioral bound is real friction for universally safe numeric algorithms.

### Oct templates vs Rust declarative macros

Rust declarative macros match and emit syntax. Oct templates substitute types inside a fixed typed declaration shape. Oct cannot generate variadic declarations or syntax families, but it keeps parsing, name resolution, dimensional analysis, Concepts, FLOW lowering, and backends on one ordinary concrete path. No M0 scientific case justified token rewriting.

### Oct templates vs Rust procedural macros

Rust procedural macros can inspect fields, derive serializers, branch on syntax, and emit arbitrary declarations. Oct deliberately cannot enumerate record fields, inspect compiler types, perform I/O, or mutate ASTs. Typed selectors cover known-field access without reflection. The torture corpus found no scientific/application case strong enough to justify a plugin/macro authority surface.

## Performance, scale, and determinism

The representative torture program collected 30 templates, produced 6 unique record, 29 function, and 3 FLOW specializations from 54 requests with 16 reuses, reached depth 3, and generated 54 concrete MIR declarations and 67,870 bytes of Go. The measured elaboration was 0.512 ms on the reporting Windows run; repeated tiny runs may fall below the timer floor.

The existing controlled scaling lane measured:

| Unique functions | Project load | Elaboration | Lower + emit | Generated Go |
|---:|---:|---:|---:|---:|
| 1 | 8.23 ms | timer floor | timer floor | 1,483 B |
| 10 | 7.76 ms | timer floor | timer floor | 9,311 B |
| 100 | 8.74 ms | 1.03 ms | 2.60 ms | 90,435 B |

Repeated loads produce identical concrete declaration names, sorted identities, counts, reuse, maximum depth, and generated structure; only elapsed timing is excluded from equality. Concrete names remain collision-resistant through canonical type mangling plus a SHA-256 suffix for long names and are not a cross-version ABI promise.

## Failure classification

| Observation | Classification | Resolution |
|---|---|---|
| Explicit-file sibling template missing | Compiler/package-loader bug | Fixed package template authority |
| Runtime recursion rejected as infinite | Compiler bug | Same-key in-progress reuse |
| New-type recursion could expand without a useful bounded chain | Safety/diagnostic gap | Depth/count budgets plus chain |
| Complex `Real`/`Imag` generated Go omitted `math` | Backend parity bug | Import authority fixed |
| Refined scalar Concept rejected on persistent FLOW board | Compiler type-authority bug | Persistence follows refinement base representation |
| Generated-name-only specialization errors | Diagnostic-quality issue | Instantiation chain/source provenance |
| Behavioral/operator constraint over `T` unavailable | Missing future design / intentional M0 limit | Documented; no trait system added |
| Generic transformed dimension (`T^2`) unavailable | Intentional current type-expression limit | Concrete failure retained |
| Template enum/type reflection/field enumeration/value params/variadics/HKT/syntax generation unavailable | Intentional language limits | Executable negative or documented boundary |
| `Pa` alias unavailable | Reference/request vocabulary gap | Base SI expression used; no alias invented |

## Public contract

Oct templates are typed compile-time specialization declarations. A template is instantiated with explicit concrete type arguments, rewritten into one deduplicated ordinary concrete Oct declaration, and then checked, lowered, interpreted, or compiled using exactly the same semantics as handwritten concrete code. Template bodies may remain dormant without universal semantic validation; errors appear when a requested specialization becomes ordinary code. No open template parameter or template call may reach a downstream phase.

They are not token macros, procedural AST macros, runtime generics, type/field reflection, untyped code generation, const/value templates, or compiler plugins. Diagnostic-only origin metadata survives long enough to explain specialization and is not runtime generic metadata.

## Central answer and next milestone

Typed specialization reaches far enough for ordinary scientific metaprogramming involving reusable algorithms, data containers, units, FLOW, and callables. It does not need a macro system for those jobs. The evidence also sharpens the correctness thesis: obviously wrong concrete programs are rejected normally; type-dependent wrong programs fail at specialization; value/refinement admission survives specialization; but conditionally valid operator-generic programs cannot yet be made universally unrepresentable because Concepts do not describe behavior.

**One proposed next milestone only:** Behavioral Concepts M0 — design one bounded, deterministic operator-capability contract for a real generic numeric consumer (for example additive/divisive mean), without reflection, implicit search, runtime dictionaries, or macro expansion. Do not begin it as part of Template Torture Lab M0.
