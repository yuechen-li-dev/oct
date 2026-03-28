# Oct v0

Oct is a compiled-first, statically typed, function-first language for scientific programming.  
Oct v0 focuses on a small, explicit core: native arrays, explicit error handling, SI dimensions, and PNG plotting.

---

## What Makes Oct Different

- **Units as types:** SI dimensions are native and participate directly in type checking.
- **Explicit error model:** fallibility is part of function signatures, and error results cannot be ignored.
- **Oct-native testing model:** valid and invalid language contracts are represented directly with `.octest` and `.octfail`.
- **Explicit package surface:** package declarations, imports, and qualification are explicit and non-magical.

---

## Minimal Example

```oct
package Main

fn Main() -> Int {
    let x = 10
    return x * 2
}
````

---

## What Oct Is Right Now

### Language Core

* Program entrypoint is `Main.Main()`.
* Packages are explicit: each `.oct` file must start with `package Name`, followed by zero or more `import Name` lines.
* Imports are explicit (no aliases or wildcards), and cross-package calls are qualified (e.g., `Geometry.Distance()`).
* Package resolution (v0): one directory = one package; imports resolve to sibling directories under the program root.
* Static base types: `Int`, `Float`, `Bool`, `String`, and built-in `Error`.
* Native one-dimensional arrays: `Int[]`, `Float[]`, `Bool[]`, including arithmetic and indexing.
* Non-capturing named function values are supported in a narrow surface via explicit function types (for example, `fn(Int) -> Int`), and only package-level named functions are valid function values.

---

### Control Flow and Selection

* `if` / `else` is supported.
* `switch` is an expression with literal `case` arms and a required `else`.
* Multi-condition selection should use condition-switch (`switch { case cond => value ... else => default }`) rather than decision-ladder `else { if ... }` chains.
* Nested `if` blocks remain valid for local control.

---

### Scientific Type Features

* SI base dimensions are supported on numeric types (`Int`, `Float`): `m`, `kg`, `s`, `A`, `K`, `mol`, `cd`.
* Dimensioned literals are supported (e.g., `5m`, `2.5s`) with derived dimensions (`m/s`, `m^2`).
* Dimensions are part of type identity; mismatches are type errors.
* Addition/subtraction require matching dimensions.
* Multiplication/division compose dimensions.
* No implicit unit conversion exists in v0.

---

### Built-ins and Runtime Utilities

* Built-ins include: `Len`, `Append`, `Abs`, `Sqrt`, `Sin`, `Cos`, `WriteOctagon`, `PlotLine`, `PlotScatter`, and `error("message")`.
* `Append(xs, x)` appends one element to an array and returns a new array value (`xs` must be an array, and `x` must exactly match its element type).
* In v0, `Append` is the intended primitive for explicit variable-length array construction (for example, `out = Append(out, value)`).
* `Sin` and `Cos` require dimensionless input.

---

### Native Testing and Verification Surface

* `.octest` is the native valid-program test format (`[Fact]`, `[Theory]`, `Assert`).
* `.octfail` is the native expected-failure format for invalid contracts discovered by `oct test`.
* `.octagon` is the native structured-data format (one top-level value, non-executable subset).
* `[Artifact]` enables explicit artifact generation via `oct artifact`.
* `[Benchmark]` enables explicit benchmark workloads via `oct bench`.
* Artifact and benchmark functions are `fn() -> Void` and are not executed by `oct test`.
* Benchmark execution currently runs once per function (no warmup/statistics yet).

### `.oct` vs `.octagon`

* Use `.oct` for executable/source concerns: packages, imports, declarations, tests, manifests, and language contracts.
* Use `.octagon` for structured data only: exactly one top-level value.
* `.octagon` allows scalar literals (`Int`, `Float`, `Bool`, `String`), dimensioned numeric literals (for example `5m`, `9.81m/s^2`), arrays, record literals, enum values (`Enum.Variant`), and nested combinations of these forms.
* `.octagon` rejects executable constructs: `package`, `import`, `fn`, `let`/`var`, control flow, function calls, mutation/assignment, and computed expressions such as `1 + 2`.
* `.octagon` is not a manifest replacement and not a scripting/config language.
* `.octagon` emission is explicit via `WriteOctagon(path, value)` (for example in `[Artifact] fn() -> Void` workflows).
* `.octagon` loading is explicit, runtime, typed, and fallible via `LoadOctagon[T](path: String) -> T ! Error`.
* `LoadOctagon[T](path)` only accepts `.octagon` paths, parses through the `.octagon` validator path, and strictly matches the loaded value against `T` (including exact record fields, enum identity, array element types, and numeric dimensions).
* `.octagon` remains non-executable and narrow: no compile-time include/import semantics, no assignment-target type inference for loads, no broad generic system, and no general file I/O API.

---

## Iteration Model

* `for` is used for structured counted iteration over integer progressions:

  * `for i in start..end`
  * `for i in start..end step s`
  * Range semantics: inclusive lower bound / exclusive upper bound.
  * `step` is optional (default `1`) and must be positive in v0.
* `while` is used for state-driven looping (convergence, retry, sentinel patterns).
* Style: prefer `for` for counted/indexed iteration; use `while` when control is state-driven.

---

## Error Handling Model

Fallible functions declare `! Error` in their signature.

* `?` propagates an error and is only valid inside a fallible function.
* `!` unwraps a fallible value and crashes if it contains an error.
* `match` handles fallible values explicitly:

  * `ok(value) => { ... }`
  * `err(e) => { ... }`
* Errors are constructed with `error("message")`.

Error results must always be handled (`?`, `!`, or `match`) — never ignored.

---

## Plotting

Built-in plotting:

* `PlotLine(x, y, path)`
* `PlotScatter(x, y, path)`

Constraints:

* Output must be `.png`.
* `x` and `y` must be numeric, dimensionless arrays.
* Arrays must be non-empty and equal length.
* Dimensioned arrays are rejected.

---

## Packages and Manifest

Oct formalizes packages as one directory per package.

A package contains:

* `manifest.oct`
* One or more `*.oct` source files
* Optional `*.octest` tests
* Optional invalid fixtures under `invalid/*.octfail`

Demo/integration programs live in a `Main/` package.

`manifest.oct` is standard Oct source and must expose:

* `record PackageManifest { Name, Version, Description, Dependencies }`
* `record Dependency { Name, VersionRequirement }`
* `fn Manifest() -> PackageManifest`

Current tooling validates manifest presence and structure.
Dependencies are metadata only in v0.

Not included in v0:

* No registry / publish / install flows
* No dependency resolution or lockfiles
* No remote fetching
* No build/feature metadata in manifests

---

## Repository Roles

* `Language/` — canonical language contracts (`.octest`, `.octfail`)
* `Packages/` — reusable proof packages
* `testdata/` — fixtures, demos, and synthetic inputs

Concept areas include:

* `Language/ControlFlow/...`
* `Language/Expressions/...`
* `Language/Errors/...`
* `Language/Functions/...`
* `Language/Types/...`

---

## Current Proof Packages

* `Numerics`
* `Mechanics`
* `Analysis`
* `Signal`
* `Structures`
* `Physics`

These demonstrate real Oct usage across scientific and numerical domains.

---

## Current Constraints and Deferred Areas

* No wildcard or alias imports
* No enum-variant injection (`case Euler` unsupported)
* No namespace flattening
* No closures, lambdas, local function literals, or partial application
* Dynamic array growth is limited (no append/push; preallocation is typical)

Some transitional semantic fixtures remain in `testdata/` while classification is finalized.

---

## Native Failure Fixtures (`.octfail`)

* Each file begins with: `expect error: "<substring>"`
* Remaining content is Oct code expected to fail

## M24f native-vs-host test boundary formalization

### What belongs in `.octest`

User-visible language behavior and contracts belong in native Oct test files under `Language/` as `.octest` and `.octfail`.

### What remains in Go tests

Go tests validate host-side integration boundaries (CLI behavior, package orchestration, and runtime wiring) rather than redefining language semantics.

### Decision rule for new tests

If a test defines language behavior, place it in `Language/`. If it validates host integration mechanics, keep it in Go.

### Demo vs test separation

`Packages/` and `testdata/` may hold demos/fixtures, but semantic pass/fail contracts are authored and maintained in `Language/`.
* Pass: compilation fails and diagnostic matches substring
* Fail: compilation succeeds or mismatch occurs

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
