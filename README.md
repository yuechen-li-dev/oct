# Oct v0

Oct is a compiled-first, statically typed, function-first language for scientific programming.
Oct v0 focuses on a small, explicit core: native arrays, explicit error handling, SI dimensions, and PNG plotting.

## What Oct Is Right Now

### Language Core

- Program entrypoint is `Main.Main()`.
- Packages are explicit: each `.oct` file must start with `package Name`, followed by zero or more `import Name` lines.
- Package imports are Oct-native and explicit (no aliases/wildcards), and cross-package calls are qualified (for example, `Geometry.Distance()`).
- Package resolution rule (v0): one directory = one package, and imports resolve to sibling directories under the program root.
- Static base types: `Int`, `Float`, `Bool`, `String`, and built-in `Error`.
- Native one-dimensional arrays: `Int[]`, `Float[]`, `Bool[]`, including array arithmetic and indexing.

### Control Flow and Selection

- `if`/`else` is supported.
- `switch` is an expression with literal `case` arms plus required `else`.
- Multi-condition selection should use condition-switch (`switch { case cond => value ... else => default }`) rather than decision-ladder `else { if ... }` chains.
- Ordinary nested local-control `if` blocks remain valid (for example, nested checks inside a branch or loop body).

### Built-ins and Runtime Utilities

- Built-ins include `Len`, `Abs`, `Sqrt`, `Sin`, `Cos`, `PlotLine`, `PlotScatter`, and `error("message")`.
- `Sin` and `Cos` require dimensionless input.

### Scientific Type Features

- Oct supports SI base dimensions on numeric types (`Int` and `Float`): `m`, `kg`, `s`, `A`, `K`, `mol`, `cd`.
- Dimensioned literals are supported (for example, `5m`, `2.5s`) along with derived dimensions such as `m/s` and `m^2`.
- Dimensions participate in type identity, so mismatched dimensions are type errors.
- Addition/subtraction require matching dimensions.
- Multiplication/division compose dimensions.
- No implicit unit conversion exists in v0.

### Native Testing and Verification Surface

- `.octest` is the native valid-program test format (`[Fact]`, `[Theory]`, `Assert`).
- `.octfail` is the native expected-failure format for invalid contracts discovered by `oct test`.
- `[Artifact]` supports explicit artifact generation in `.octest` via `oct artifact <file-or-root>`.
- `[Benchmark]` supports explicit benchmark workloads in `.octest` via `oct bench <file-or-root>`.
- Artifact and benchmark functions are `fn() -> Void`, run explicitly, and are not executed by `oct test`.
- Benchmark execution is currently one measured run per benchmark function (no warmup/repetition/statistics yet).

## What Makes Oct Different

- **Units as types:** SI dimensions are native and participate directly in type checking.
- **Explicit error model:** fallibility is part of function signatures, and error results cannot be ignored.
- **Oct-native testing model:** valid and invalid language contracts are represented directly with `.octest` and `.octfail`.
- **Explicit package surface:** package declarations, imports, and qualification are explicit and non-magical.

## Iteration Model

- `for` is for structured counted iteration over integer progressions:
  - `for i in start..end` where `start` and `end` are `Int` expressions.
  - `for i in start..end step s` where `s` is an `Int` expression.
  - Range semantics are inclusive lower bound / exclusive upper bound.
  - `step` is optional (defaults to `1`) and must be positive in v0.
- `while` remains for state-driven looping (convergence, retry, sentinel patterns).
- Style guidance: prefer `for` for clear counted/indexed iteration; keep `while` when loop progress or exit is primarily state-driven.

## Error Handling Model

Fallible functions declare `! Error` in their signature.

- `?` propagates an `Error` from a fallible expression and is only valid inside a fallible function.
- `!` unwraps a fallible expression and crashes with a fatal runtime error when it contains `Error`.
- `match` handles fallible values explicitly:
  - `ok(value) => { ... }`
  - `err(e) => { ... }`
- Error values are constructed with `error("message")`.

Error results are never silently ignored: they must be handled with `?`, `!`, or `match`.

## Plotting

Oct v0 provides two plotting built-ins:

- `PlotLine(x, y, path)`
- `PlotScatter(x, y, path)`

Plotting contract:

- Output path must end with `.png` (PNG-only).
- `x` and `y` must be arrays of numeric, dimensionless values.
- Dimensioned arrays are rejected.
- `x` and `y` must be non-empty and have equal length.

## Packages and Manifest

Oct formalizes package shape around one directory per package.

A formal package directory contains:

- `manifest.oct`
- One or more package source files (`*.oct`)
- Optional package tests (`*.octest`)
- Optional invalid fixtures under `invalid/*.octfail`

Demo/integration consumers continue to live in a `Main/` package.

`manifest.oct` is ordinary Oct source (not JSON/TOML/YAML) and must expose:

- `record PackageManifest { Name, Version, Description, Dependencies }`
- `record Dependency { Name, VersionRequirement }`
- `fn Manifest() -> PackageManifest`

Current tooling validates manifest presence and shape for manifested package roots. Dependency entries are currently metadata declarations only.

Not in scope for v0 package tooling:

- Registry, publish, install, or update flows
- Dependency resolution or lockfiles
- Remote source fetching
- Feature/target/build-script metadata in manifests

### Repository Roles

- `Language/`: canonical native language contracts (`.octest` and `.octfail`) organized by language concept.
- `Packages/`: real reusable/evolving proof packages.
- `testdata/`: fixtures, demos/integration inputs, and synthetic/temporary scenario inputs.

Canonical concept-first areas currently include:

- `Language/ControlFlow/IfExpression/{valid,invalid}`
- `Language/ControlFlow/ConditionSwitch/{valid,invalid}`
- `Language/ControlFlow/DecisionLadder/{valid,invalid}`
- `Language/ControlFlow/Loops/{valid,invalid}`
- `Language/Expressions/Arithmetic/{valid,invalid}`
- `Language/Expressions/Literals/{valid,invalid}`
- `Language/Expressions/Logical/{valid,invalid}`
- `Language/Errors/Fallible/{valid,invalid}`
- `Language/Functions/Calls/{valid,invalid}`
- `Language/Types/Arrays/{valid,invalid}`

## Current Proof Packages

Oct currently maintains these proof packages under `Packages/`:

- `Numerics`
- `Mechanics`
- `Analysis`
- `Signal`
- `Structures`
- `Physics`

These packages demonstrate package-based Oct code for iterative numerics, unit-safe mechanics/statics, analysis workflows, signal processing, and small structural computations, using records/enums, loops, SI dimensions, arrays, and cross-package integration.

Package-local correctness is expressed through native tests in each package via `.octest`, while `Main/main.oct` files remain demo/integration entrypoints.


## Current Constraints and Deferred Areas

- No wildcard imports, no alias imports, no enum-variant injection (`case Euler` remains unsupported), and no package namespace flattening.
- Dynamic array growth ergonomics remain limited in v0 (append/push is still unavailable), so fixed-size preallocation is often used for loop-driven datasets.
- Some semantic fixtures intentionally remain transitional in `testdata/` where taxonomy/deduplication is still being finalized (for example mixed dimensioned arithmetic, print contracts, and related transitional fixtures).

## Native Failure Fixtures (`.octfail`)

- `.octfail` is a single-failure fixture format discovered by `oct test`.
- Each file must begin with exactly one header line: `expect error: "<substring>"`.
- The remaining body is Oct source expected to fail build/typecheck.
- Pass rule: compile fails and the diagnostic contains the expected substring.
- Fail rule: compile succeeds unexpectedly, header is malformed/missing, or the expected substring does not match.

Minimal example:

```oct
expect error: "condition switch requires else arm"

package Main

fn Main() -> Int {
    return switch {
        case true => 1
    }
}
```
