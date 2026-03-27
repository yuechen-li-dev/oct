# Oct v0

Oct is a compiled-first, statically typed, function-first language for scientific programming.
Oct v0 focuses on a small, explicit core: native arrays, explicit error handling, SI dimensions, and PNG plotting.

## Oct v0 feature set

- Packages are explicit: each `.oct` file must start with `package Name`, followed by zero or more `import Name` lines.
- Package imports are Oct-native and explicit (no aliases/wildcards), and cross-package calls are qualified (`Geometry.Distance()`).
- Package resolution rule (v0): one directory = one package, and imports resolve to sibling directories under the program root.
- Program entrypoint is `Main.Main()`.
- Static base types: `Int`, `Float`, `Bool`, `String`, and built-in `Error`.
- Native one-dimensional arrays: `Int[]`, `Float[]`, `Bool[]`, including array arithmetic and indexing.
- Control flow: `if`/`else`, `for i in 0..n` with optional `step`, and expression `switch` with literal `case` arms plus required `else`.
- Built-ins: `Len`, `Abs`, `Sqrt`, `Sin`, `Cos`, `PlotLine`, `PlotScatter`, and `error("message")`.

### Loop model (M27)

- `for` is for structured counted iteration over integer progressions:
  - `for i in start..end` where `start` and `end` are `Int` expressions.
  - `for i in start..end step s` where `s` is an `Int` expression.
  - range semantics are inclusive lower bound / exclusive upper bound.
  - `step` is optional (defaults to `1`) and must be positive in v0.
- `while` remains for state-driven looping (convergence/retry/sentinel patterns).
- Style guidance: prefer `for` for clear counted/indexed iteration; keep `while` when loop progress or exit is primarily state-driven.

## M25 package layout + Oct-native manifest foundation

Oct now formalizes package shape around one directory per package.

- A formal package directory contains:
  - `manifest.oct`
  - one or more package source files (`*.oct`)
  - optional package tests (`*.octest`)
  - optional invalid fixtures under `invalid/*.octfail`
- Demo/integration consumers continue to live in a `Main/` package.

`manifest.oct` is ordinary Oct source (not JSON/TOML/YAML) and must expose:

- `record PackageManifest { Name, Version, Description, Dependencies }`
- `record Dependency { Name, VersionRequirement }`
- `fn Manifest() -> PackageManifest`

Current tooling validates manifest presence and shape for manifested package roots. For M25, dependency entries are metadata declarations only.

M25 intentionally does **not** add package-manager capabilities:

- no registry, publish, install, or update flows
- no dependency resolution or lockfiles
- no remote source fetching
- no feature/target/build-script metadata in manifests

## M26a golden-path package seed migration to `Packages/`

M26a starts the proof-package seed migration by moving a representative set of real package seeds into a first-class top-level package area:

- `Packages/Numerics`
- `Packages/Signal`
- `Packages/Structures`

Each migrated package keeps its package-local shape (`manifest.oct`, `*.oct`, `*.octest`, plus optional package-local support files).

Repository distinction after M26a:

- `Packages/`: real reusable proof-package seeds that evolve as first-class Oct packages.
- `testdata/`: fixtures-first area for invalid cases, synthetic parser/typechecker/runtime inputs, and command-level scenario fixtures.

M26a intentionally migrates only this small golden-path set; the rest of package-seed migration is deferred to later milestones.

Migration friction observed in M26a:

- several Go CLI/integration tests had direct `testdata/...` path assumptions for Numerics/Signal/Structures and now target `Packages/` for those seeds
- benchmark coverage previously pointed at `testdata/m24i/valid`; it now runs from the migrated `Packages/Signal` benchmark fixture
- demo `Main` fixture packages remain under `testdata/m22*/valid/Main` to keep integration behavior stable while avoiding unnecessary demo churn in this milestone

## M26b package-seed migration completion to `Packages/`

M26b completes the remaining real proof-package seed migration by moving:

- `Packages/Mechanics`
- `Packages/Analysis`
- `Packages/Physics`

Repository distinction after M26b:

- `Packages/`: real reusable/evolving proof packages (`Numerics`, `Signal`, `Structures`, `Mechanics`, `Analysis`, `Physics`).
- `testdata/`: fixtures-first inputs (invalid fixtures, synthetic parser/typechecker/runtime cases, and demo/integration `Main` packages).

M26b keeps package-local ownership intact (`manifest.oct`, package sources, `.octest`, and optional package-local files) and leaves demo consumers in `testdata/.../Main` where they remain useful for CLI integration coverage.

Migration friction observed in M26b:

- additional Go CLI/integration tests still had hard-coded `testdata/...` package-seed paths and were updated to copy package seeds from `Packages/`
- package-seed vs fixture intent was still ambiguous in historical docs/examples and required explicit documentation refresh to reinforce `Packages/` vs `testdata/` roles
- demo/consumer `Main` fixtures intentionally remain in `testdata/`, so integration setups now combine `Packages/*` libraries with `testdata/.../Main` consumers


## M29a/M29b permanent semantic-suite home in `Language/`

M29a established `Language/` as the permanent home for canonical native language contract suites, and M29b generalizes that pattern to additional clearly classifiable semantic groups.

Repository distinction after M29b:

- `Language/`: canonical native language contracts (`.octest` and `.octfail`) organized by language concept.
- `Packages/`: reusable/evolving package seeds.
- `testdata/`: fixtures, demos/integration inputs, and synthetic/temporary scenario inputs.

Canonical concept-first structure after M29b:

- `Language/ControlFlow/IfExpression/{valid,invalid}`
- `Language/ControlFlow/ConditionSwitch/{valid,invalid}`
- `Language/ControlFlow/DecisionLadder/{valid,invalid}`
- `Language/Expressions/Arithmetic/{valid,invalid}`
- `Language/Expressions/Logical/{valid,invalid}`
- `Language/Errors/Fallible/{valid,invalid}`

M29b moved the following classes out of transitional roots:

- arithmetic expression `.octest` contracts from `testdata/m28a/main_valid` into `Language/Expressions/Arithmetic/valid`
- logical expression `.octest` contracts from `testdata/m28a/main_valid` into `Language/Expressions/Logical/valid`
- fallible-signature / `?` misuse `.octfail` contracts from `testdata/m28b/main_invalid` into `Language/Errors/Fallible/invalid`

Intentionally deferred transitional remainder after M29b:

- mixed control-flow, array/indexing, dimensioned numerics, print, literals, and loop fixtures still in `testdata/m28a/main_valid`
- call-shape invalid fixture (`call_arity_mismatch.octfail`) still in `testdata/m28b/main_invalid` because it is better classified under a future call semantics bucket rather than forced into the fallible taxonomy

Migration friction observed across M29a/M29b:

- several Go CLI orchestration tests had historical `testdata/...` assumptions and were updated to point at `Language/ControlFlow/...` for migrated classes
- additional orchestration assumptions needed extension to `Language/Expressions/...` and `Language/Errors/...` roots for migrated classes
- extracted corpus outside cleanly separable classes is still too mixed to classify safely without a larger follow-up pass

## Error handling model

Fallible functions declare `! Error` in their signature.

- `?` propagates an `Error` from a fallible expression and is only valid inside a fallible function.
- `!` unwraps a fallible expression and crashes with a fatal runtime error when it contains `Error`.
- `match` handles fallible values explicitly:
  - `ok(value) => { ... }`
  - `err(e) => { ... }`
- Error values are constructed with `error("message")`.

Error results are never silently ignored: they must be handled with `?`, `!`, or `match`.

## SI dimensions and units

Oct supports SI base dimensions on numeric types (`Int` and `Float`):

- `m` (meter), `kg` (kilogram), `s` (second), `A` (ampere), `K` (kelvin), `mol` (mole), `cd` (candela)
- Dimensioned literals such as `5m`, `2.5s`, and derived dimensions like `m/s` or `m^2`.
- Dimensions participate in type identity, so mismatched dimensions are type errors.
- Addition/subtraction require matching dimensions.
- Multiplication/division compose dimensions.
- `Sin` and `Cos` require dimensionless input.
- No implicit unit conversion exists in v0.

## Plotting

Oct v0 provides two plotting built-ins:

- `PlotLine(x, y, path)`
- `PlotScatter(x, y, path)`

Plotting contract:

- Output path must end with `.png` (PNG-only).
- `x` and `y` must be arrays of numeric, dimensionless values.
- Dimensioned arrays are rejected.
- `x` and `y` must be non-empty and have equal length.

## M22a proof package: Numerics (Root Finding v0)

M22a adds a native `Numerics` proof package (now at `Packages/Numerics`, originally introduced under `testdata/m22a/valid/Numerics`) with:

- root-finding methods: `Bisection` and `NewtonRaphson`
- structured solver configuration via `SolveOptions`
- structured outcomes via `RootResult`
- explicit termination status via `RootReason`
- a small fixed problem selector (`ProblemSqrt2()` / `ProblemCubic()`) for `x*x - 2` and `x*x*x - x - 2`

This milestone is a proof that package-based Oct code can express real iterative numerical algorithms with records/enums, loops, convergence checks, explicit fallible errors, and cross-package reuse from `Main`.

Implementation friction observed in M22a:

- Without first-class functions, solver APIs use explicit problem selectors and package-owned residual/derivative functions.
- Cross-package enum value flow is still awkward in v0, so this proof package uses integer problem IDs while keeping a dedicated termination enum for package status design.

## M22b proof package: Mechanics (Unit-Safe 2D Statics / Kinematics Core)

M22b adds a native `Mechanics` package in Oct source (now at `Packages/Mechanics`, originally introduced under `testdata/m22b/valid/Mechanics`) with:

- domain records: `Vec2Force`, `Displacement2`, `Stiffness2`, and `EquilibriumReport`
- enum-driven helpers via `Axis` and `DominantAxis`
- unit-safe 2D statics helpers: `AddForce`, `MagnitudeSquared`, and `ResultantReport`
- a matrix/vector-backed mechanics helper: `InternalForce` computes `K @ d` for a 2x2 stiffness-like model

This milestone is a proof that packages, records/enums, SI dimensions, comparisons/logical flow, and matrix/vector operations compose naturally in small mechanics-oriented Oct code.

Implementation friction observed in M22b:

- A single generic `Vec2<T>` shape is still not available, so this proof uses explicit domain-specific records (`Vec2Force`, `Displacement2`).
- Matrix shape constraints are runtime/value-level rather than encoded in the type, so API documentation and tests carry shape intent (`2x2` with `2x1`).

## M22c proof package: Analysis (Data -> Compute -> Plot Workflow v0)

M22c adds a native `Analysis` package in Oct source (now at `Packages/Analysis`, originally introduced under `testdata/m22c/valid/Analysis`) with:

- structured workflow output via `AnalysisResult { X, Y, Z }`
- one linear end-to-end function: `GenerateAndCompute(start, increment)`
- explicit array-oriented analysis flow for a small fixed-size dataset (`x = start + i*increment`, `y = x*x - 2`, `z = x*x`)
- direct plotting from package-produced arrays using built-in `PlotLine`/`PlotScatter` in `Main`

This milestone is a proof that Oct can express a readable data -> compute -> structured result -> plot flow with packages, records, loops, mutable locals, indexing, `Len`, and plotting.

Implementation friction observed in M22c:

- v0 arrays do not support dynamic append, so fixed-size preallocation is still required for loop-driven datasets.

## M22cr proof refresh: Analysis (Post-M23 Array Ergonomics Recheck)

M22cr refreshes the same `Analysis` package and workflow after M23 array element assignment landed.

- `GenerateAndCompute` now fills arrays directly inside the loop with `x[i] = ...`, `y[i] = ...`, and `z[i] = ...`.
- the workflow shape remains intentionally unchanged: generate data -> compute values -> return `AnalysisResult` -> plot from `Main`.
- tests continue to validate result correctness, plotting output generation, package integration/build behavior, and now include a proof-oriented check that the package source uses direct element assignment.

Ergonomics assessment (M22cr):

- **Material improvement:** yes. The core population loop is now straightforward Oct code and no longer relies on whole-array reassignment helpers.
- **Earlier awkwardness removed:** mostly yes for fixed-size workflows; direct indexed writes make the proof package read naturally.
- **Residual friction:** meaningful but narrower. Dynamic-growth workflows remain awkward because append/push is still unavailable, so this proof continues using fixed-size arrays.

## M22d proof package: Signal (Small Signal Processing Workflow v0)

M22d adds a native `Signal` proof package (now at `Packages/Signal`, originally introduced under `testdata/m22d/valid/Signal`) with:

- structured workflow output via `SignalAnalysisResult { X, Raw, Smoothed }`
- deterministic sample generation via `GenerateSignalValues()`
- explicit moving-average filtering via `MovingAverage(values, window)`
- one end-to-end analysis function via `AnalyzeSignal(start, delta, window)` that prepares axis values, applies filtering, and returns a record
- direct plotting from package-produced arrays in `Main` using `PlotLine`

This milestone is a proof that Oct can express a small, readable signal workflow (generate -> smooth -> return structured arrays -> plot) using packages, records, loops, mutable locals, indexing, array assignment, `Len`, and strings for output paths.

Implementation friction observed in M22d:

- The workflow is clean for fixed-size arrays, but dynamic dataset growth still requires predeclared sizing because append/push is not available in v0.

## M22e proof package: Structures (Small Structural / Truss-Lite v0)

M22e adds a native `Structures` proof package (now at `Packages/Structures`, originally introduced under `testdata/m22e/valid/Structures`) with:

- structured element input via `BarElement2D { Area, YoungsModulus, Length }`
- structured output via `AxialElementResponse { Stiffness, Displacement, InternalForce }`
- a matrix-producing mechanics helper `AxialStiffness(element)` for a 2x2 axial bar element local matrix (`AE/L` form)
- a matrix @ vector response path `AxialForceFromDisplacement(element, displacement)`
- an end-to-end package API `AnalyzeAxialElement(element, u1, u2)` used by `Main`

This milestone is a proof that Oct can express a small, physically meaningful structural computation with packages, records, SI dimensions, matrices, vectors, and clear engineering formulas.

Implementation friction observed in M22e:

- Matrix/vector shape intent is still validated by usage/tests instead of being encoded at the type level (fixed 2-entry displacement for a 2x2 local stiffness workflow).

## M24d proof/package native-test generalization

M24d completes the current proof-package migration to package-local native tests by extending the M24c pattern across the remaining proof packages.

- package-local native tests now cover the current proof suite:
  - `Packages/Numerics/solvers.octest`
  - `Packages/Mechanics/mechanics.octest`
  - `Packages/Analysis/analysis.octest`
  - `Packages/Signal/signal.octest`
  - `Packages/Structures/structures.octest`
- proof-package correctness checks now run through `oct test` with `[Fact]`, `[Theory]`, and `Assert`
- `Main/main.oct` files remain demo/workflow entrypoints (printing/plotting/integration) and are kept distinct from correctness tests
- Go harness tests now focus on CLI/integration boundaries for these proof fixtures rather than reenacting package semantics in ad hoc `Main` return-code checks

Migration friction observed in M24d:

- dimension-heavy assertions still rely on manual normalization (for example dividing by `1kg*m/s^2`) to use scalar `Assert.Near`, which is workable but verbose
- package-local tests are readable for fixed-size workflows, but dynamic-growth array workflows remain awkward while append/push is still unavailable in v0

Out of scope (intentionally unchanged in M24d):

- buried Go compiler/runtime test migration
- testing-framework redesign or fixture/snapshot systems


## M24e native language-contract golden path: if-expression semantics

M24e migrates one coherent class of user-visible language contracts into native `.octest`: **if-expression selection semantics**.

- native coverage lives in `Language/ControlFlow/IfExpression/valid/if_contracts.octest` and validates then/else value selection, lazy non-selected branch behavior, and sign-bucket selection through `[Fact]` + `[Theory]`
- corresponding fixture code lives in `Language/ControlFlow/IfExpression/valid/if_contracts.oct`
- rejection-path coverage now lives natively at `Language/ControlFlow/IfExpression/invalid/if_expression_branch_type_mismatch.octfail` (branch-type mismatch), asserted via `oct test`

Go vs Oct boundary clarified by M24e:

- **Oct-native tests own:** user-visible language semantics and contract readability for if-expression behavior
- **Go tests own:** CLI integration/orchestration and invalid-fixture rejection assertions, plus compiler/parser/runtime internals

Migration friction observed in M24e:

- negative/rejection contracts are now represented natively with `.octfail`, but broader migration for mixed legacy suites is still deferred

## M24f native-vs-host test boundary formalization

M24f closes this migration round by documenting and enforcing where tests belong.

### What belongs in `.octest`

- package-local proof/package correctness (for example `Packages/Numerics/solvers.octest`, `Packages/Mechanics/mechanics.octest`)
- user-visible valid-language semantics (for example `Language/ControlFlow/IfExpression/valid/if_contracts.octest`)
- readable language-contract checks that should be expressed from an Oct user's perspective

### What remains in Go tests

- parser/lexer/compiler/typechecker internals and host runtime plumbing
- invalid-fixture orchestration and expected compile-failure assertions for non-migrated fixture areas
- CLI/tooling boundary checks (`oct run`, `oct build`, `oct test`) and artifact expectations

### Decision rule for new tests

- if behavior is a **valid, user-visible language contract**, prefer `.octest`
- if behavior is an **implementation/internal detail** or an **invalid fixture / tool orchestration** concern, prefer Go tests

### Demo vs test separation

- `Main/main.oct` files remain demo/integration entrypoints and are not the primary home for correctness assertions
- `.octest` files are the canonical home for correctness assertions
- Go tests remain the host-side verification layer for invalid fixtures and CLI/tool boundaries

### M24f consolidation notes

- condition-switch valid semantics now have native coverage in `Language/ControlFlow/ConditionSwitch/valid/condition_switch.octest`
- condition-switch invalid contracts are now native `.octfail` fixtures in `Language/ControlFlow/ConditionSwitch/invalid`

Migration friction still present after M24f:

- remaining non-migrated semantic classes are intentionally deferred until they can be cleanly classified by concept
- dimension-heavy numeric assertions still require normalization workarounds before scalar `Assert.Near`
- dynamic-growth array workflows remain awkward while append/push ergonomics continue to evolve

## Native Artifact Generators (`[Artifact]`)

M24h adds explicit artifact generation in `.octest` for test-adjacent utilities such as fixture templates and small harness outputs.

- `[Artifact]` applies to `.octest` functions only
- artifact functions must be `fn() -> Void`
- run artifacts explicitly with `oct artifact <file-or-root>`
- artifacts are not tests and are not executed by `oct test`
- artifact functions are expected to be idempotent by convention; the runner always executes them when requested

## Native Benchmarks (`[Benchmark]`)

M24i adds explicit benchmark workloads in `.octest` so measurement code is separate from correctness tests.

- `[Benchmark]` applies to `.octest` functions only
- benchmark functions must be `fn() -> Void`
- run benchmarks explicitly with `oct bench <file-or-root>`
- benchmarks are not tests and are not executed by `oct test`
- M24i intentionally performs one measured execution per benchmark function (no warmup/repetition/statistics yet)

## Expected-Failure Tests (`.octfail`)

M24g adds native expected-failure fixtures so invalid language contracts can live in Oct-native artifacts instead of embedded Go source strings.

- `.octfail` is a single-failure fixture format, discovered by `oct test`
- each file must start with one header line: `expect error: "<substring>"`
- the rest of the file is Oct source that is expected to fail build/typecheck
- pass rule: compile fails and the error message contains the expected substring
- fail rule: compile succeeds unexpectedly, header is malformed/missing, or message substring does not match

Example:

```oct
expect error: "condition switch requires else arm"

package Main

fn Main() -> Int {
    return switch {
        case true => 1
    }
}
```

How it differs from `.octest`:

- `.octest` runs executable `[Fact]` / `[Theory]` tests for valid programs
- `.octfail` runs compile-failure expectation fixtures for invalid contracts

## M23d ergonomics polish: explicit cross-package enum flow

M23d tightens cross-package enum ergonomics while keeping package and enum qualification explicit.

- imported enum types continue to be written explicitly in type positions (`Physics.Method`)
- imported enum values continue to be written explicitly (`Physics.Method.Euler`)
- package-qualified enum values now flow consistently through local bindings, records, returns, mutation, and enum `switch` labels across package boundaries

Intentionally unchanged in M23d:

- no wildcard imports
- no alias imports
- no enum-variant injection (`case Euler` remains unsupported)
- no package namespace flattening or package-model redesign

## M23e control-flow shape rule: no decision-ladder `if/else`

M23e deliberately disallows nested decision-ladder `if/else` chains (the `else { if ... }` multi-case shape).

- for multi-condition selection, use the condition-switch expression (`switch { case cond => value ... else => default }`)
- ordinary nested local-control `if` blocks remain valid (for example nested checks inside a then-branch or loop body)
