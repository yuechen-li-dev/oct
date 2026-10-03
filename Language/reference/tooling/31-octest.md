# octest

## Overview

`oct test` executes test contracts from `.octest` and `.octfail` files.
`.octest` files are ordinary Oct package files with test-lane metadata; they use normal `package`, `import`, typechecking, and package-root resolution rules.
`.octfail` files are negative contracts: they pass only when the declared error substring is produced, either when the source is compiled or, for a runtime expectation, when its `Main` runs.

`oct test <path>` discovers `.octest` and `.octfail` files recursively under `<path>`.
A single `.octest` file may also contain `[Artifact]` and `[Benchmark]` functions, but `oct test` runs only `[Fact]` and `[Theory]` cases plus `.octfail` checks.
Use `oct artifact` for `[Artifact]` functions and `oct bench` for `[Benchmark]` functions.

For agent or CI consumption, add `--json` to one target. It emits one
`oct.cli.result.v1` object with command/version identity, test-file discovery,
pass/fail/skip counts, compiled and interpreted-fallback counts, diagnostics,
human output, timing, and exit status. Human-readable output remains the
default.

## `[Fact]`

`[Fact]` marks one test function.
A fact has no parameters and must use this signature:

```oct
[Fact]
fn Name() -> Void {
    Assert.Equal(2, 1 + 1, "math")
}
```

Facts have a fixed `30.0s` cycle time.
A passing fact or theory row must execute at least one `Assert.*` call unless it terminates with `SkipTest("reason")`.

## `[Theory]`

`[Theory]` marks a parameterized test function.
A theory must have at least one parameter, must return `Void`, and must declare at least one `[InlineData(...)]` row.
Each inline row becomes one test case with a zero-based display suffix such as `Package.Function[0]`.

```oct
[Theory]
[InlineData(1, 2)]
[InlineData(3, 4)]
fn PairIsOrdered(a: Int, b: Int) -> Void {
    Assert.True(a < b, "ordered")
}
```

`[InlineData(...)]` is valid only on `[Theory]` and supports scalar literals and enum values.
Theory rows default to `30.0s`; `[CycleTime(t)]` may override the row cycle time and requires exactly one positive `Float<s>` argument.

## Suites and selection

`[Suite("Name")]` optionally tags a `[Fact]`, `[Theory]`, `[Artifact]`, or `[Benchmark]` function with a suite name.
A function may repeat `[Suite("...")]` to belong to multiple suites.
Suite names must be non-empty after trimming; dotted names such as `"Experiments.FmBrownNoiseKalman.M1"` are allowed.
File-level `[Suite(...)]` is not supported.

`oct test <path> --suite <name>` runs only `[Fact]` and `[Theory]` cases whose suite set contains `<name>`.
In suite-target mode, unsuited tests are excluded.
Suite filtering is execution selection only: imports and dependencies still load and typecheck normally.
Tests from imported packages do not run unless they are explicitly selected by the requested suite.

## Assertions

Assert helpers are intended for `.octest` tests and are not part of general runtime program semantics.
The current commonly supported helpers are:

- `Assert.Equal(expected: T, actual: T, message: String) -> Void`
- `Assert.True(condition: Bool, message: String) -> Void`
- `Assert.False(condition: Bool, message: String) -> Void`
- `Assert.Near(expected: Float, actual: Float, tolerance: Float, message: String) -> Void`
- `Assert.Error(expr: T ! Error, message: String) -> Void`
- `Assert.LGTM(expr: T ! Error, message: String) -> T`

```oct
[Fact]
fn Assertions() -> Void {
    Assert.True(3 > 2, "true condition")
    Assert.False(2 > 3, "false condition")
    Assert.Equal(4, 2 + 2, "exact value")
    Assert.Near(1.0, 1.001, 0.01, "near float")
}
```

`Assert.Near` currently supports scalar `Float` values with matching dimensions.
Use `Assert.LGTM(<fallible-expression>, "reason")` when the main contract is successful completion and you need the unwrapped value for later assertions.
Use `Assert.Error(<fallible-expression>, "reason")` when success would be a test failure.

## Fallible tests, errors, and `.octfail`

Fallible expressions can be handled with the normal Oct error surface (`?`, `!`, or `match`) in test bodies.
For smoke-style fallible calls, prefer `Assert.LGTM` so the fallible success is counted as an assertion and the underlying runtime error is reported clearly on failure.

```oct
fn MightFail(value: Int) -> Int ! Error {
    if value > 0 {
        return value + 1
    }
    return error("value must be positive")
}

[Fact]
fn FallibleSmoke() -> Void {
    let value = Assert.LGTM(MightFail(4), "call should succeed")
    Assert.Equal(5, value, "returned value")
    Assert.Error(MightFail(0), "negative input should fail")
}
```

An `.octfail` file holds a program that must fail. Its first non-blank line is one expectation header, and the rest is ordinary Oct source checked as a `.oct` file. The source is checked as a copy, on its own: it may `import` a library of the repository it lives in, but files and packages beside it are not part of it.

- `expect error: "<non-empty substring>"` is a compile-time contract. The file passes when compilation fails with an error containing the substring. It fails if the source compiles.
- `expect runtime error: "<non-empty substring>"` is a runtime contract. The source must compile, and running its `Main` must stop with a failure whose message contains the substring: a runtime error, a failed `Assert.True`, a failed `!` unwrap, or an `Error` returned from a fallible `Main`. The file fails if the source does not compile, if `Main` runs to completion, or if the message does not contain the substring.

A runtime contract is checked per execution lane. Under the default `--execution auto` the interpreted lane and the compiled lane must both fail with the expected text; `--execution interpreted` and `--execution compiled` check that lane alone. Neither lane stands in for the other. A compiled run that does not stop within 30 seconds fails the contract.

Use a runtime contract for failures the type system cannot see, such as a length mismatch between two arrays or a violated builtin precondition. `Assert.Error` remains the form for an ordinary fallible result inside a `[Fact]`.

```oct
expect runtime error: "array length mismatch: 3 vs 1"

package Main

fn Main() -> Void {
    let xs: Float[] = [1.0, 2.0, 3.0]
    let ys: Float[] = [10.0]
    let bad = xs + ys
}
```

A file with a malformed header, an empty substring, or more than one header is an error.

## Skips and cycle time

`SkipTest(reason: String) -> Void` is available only in `.octest` `[Fact]` and `[Theory]` bodies.
The reason is mandatory and must be non-empty.
`SkipTest` is terminal for the current fact or theory row; statements after it are not executed.
A skipped test reports `SKIP`, not `PASS`.
Exceeding cycle time is a `FAIL` outcome, not a skip.

## Interpreted vs compiled execution

`oct test` supports explicit and automatic execution modes:

```text
oct test <path> --execution interpreted
oct test <path> --execution compiled
oct test <path> --execution auto
```

`auto` is the default when `--execution` is omitted.
In `auto`, the runner first tries compiled execution for each `.octest` case and falls back to interpreted execution when compiled execution is unsupported for that case. Tests restricted to one lane are the exception; see [Execution lane restrictions](#execution-lane-restrictions-interpreted-and-compiled).
In `compiled`, a test case must run through the compiled test path or it fails.
In `interpreted`, tests run through source interpretation.

Compiled test execution may build and run generated compiled artifacts internally, but users should treat this as a test execution mode rather than a stable artifact layout. When `OCT_KEEP_TEST_ARTIFACTS=1` is used for diagnostics, each owned runner scope retains distinct `<case>.generated.go` source and `<case>.octbin[.exe]` executable paths; the Windows executable suffix is `.octbin.exe`.
Some packages still use language/library features that are not compiled-supported, and missing wrapper sidecars can affect compiled wrapper tests.
Interpreted and compiled parity is tracked by package and test coverage, so do not assume every test package compiles until it has been run in compiled mode.
A compile-time `.octfail` is checked the same way in every execution mode. A runtime `.octfail` is run in the lanes that the execution mode selects.

## Execution lane restrictions: `[Interpreted]` and `[Compiled]`

A `[Fact]` or `[Theory]` may be restricted to one execution lane, interpreted or compiled:

```oct
[Fact]
[Compiled("the source bodies are stubs; only the compiled lane replaces them with sidecar calls")]
fn WrapperEchoesThroughTheSidecar() -> Void ! Error {
    Assert.Equal("hello", EchoString("hello")?, "echo")
}

[Fact]
[Interpreted("the interpreted lane runs the source body of a function the manifest also names")]
fn SourceBodyRuns() -> Void ! Error {
    Assert.Equal("source:value", ShadowRaw("value")?, "source body")
}
```

- The reason is required. It is a non-empty string literal, and the runner prints it whenever it skips the test.
- The attribute applies to `[Fact]` and `[Theory]` functions only. On a `[Theory]` it covers every `[InlineData]` row. It does not apply to `[Artifact]` or `[Benchmark]` functions.
- `[Interpreted]` and `[Compiled]` cannot both apply to one function. A test that runs in both lanes takes neither.
- `--execution interpreted` reports a `[Compiled]` test as `SKIP`, and `--execution compiled` reports an `[Interpreted]` test as `SKIP`. A skipped test is counted as skipped, not as passed.
- `--execution auto` runs every test, each in its own lane. A `[Compiled]` test that the compiled lane cannot build or run fails; it does not fall back to interpreted execution. An `[Interpreted]` test is not counted as a fallback.
- The compiled lane does not build an `[Interpreted]` test, so such a test may reach a builtin the compiled lane does not support without affecting the other tests in its file.
- The whole file is still parsed and typechecked in every mode. A lane restriction selects execution; it does not excuse a type error.

Use a lane restriction only when the lane is the subject of the test: the two lanes are specified to behave differently and the test pins one of those behaviours. Do not use it for a feature that one lane is missing. A test for a missing feature should fail in that lane until the feature is implemented; restricting it hides the gap.

## File and layout conventions

- `.octest` and `.oct` files use the same package import resolver and repository package-root search order.
- `.octest` supports test-lane attributes only on functions.
- `[Fact]`, `[Theory]`, `[Artifact]`, and `[Benchmark]` attributes are mutually constrained; invalid combinations are rejected.
- `[Artifact]` functions must have no parameters and return `Void` or `Void ! Error`; `[Benchmark]` functions use `fn Name() -> Void`. Neither lane requires assertions.
- Current Language fixtures commonly organize accepted behavior under `valid/` and rejected behavior under `invalid/`, with runtime-pass cases often under `runtime/valid/`.
- Selecting a single `.octest` file limits execution to that selected source file; selecting a directory discovers tests recursively under that directory.
- Declarations shared by several test files belong in a `.oct` file, or in a `.octest` file that declares no `[Fact]`, `[Theory]`, `[Artifact]` or `[Benchmark]`. The compiled lane builds each test file with those support sources and does not see what another test file declares; the interpreted lane loads the directory as one package and does. So a test file must not use a declaration from a sibling test file (the compiled lane rejects it), and two test files in one directory must not declare the same name (the interpreted lane rejects it).

## Lane policy

`[Fact]` and `[Theory]` are correctness-contract lanes.
`[Benchmark]` is a measurement lane, not a correctness-proof lane.
`[Artifact]` is a code-driven artifact-generation lane, not a correctness-test lane.

A mixed `.octest` file is allowed, but each CLI command executes only its lane:

- `oct test` runs `[Fact]` and `[Theory]` cases plus `.octfail` checks.
- `oct bench` runs `[Benchmark]` only.
- `oct artifact` runs `[Artifact]` only.

```oct
package Main

[Fact]
fn CorrectnessCheck() -> Void {
    Assert.Equal(1 + 1, 2, "math")
}

[Benchmark]
fn MeasureSomething() -> Void {
    Print(1 + 1)
}

[Artifact]
fn EmitReferenceData() -> Void {
    let data = [1, 2, 3]
    Artifact.WriteOctagon("out/reference.octagon", data)
}
```

Artifact functions are build-time entry points. They are discovered only after
the selected package graph binds and type-checks successfully, run only for an
explicit `oct artifact` command, do not run when imported, and cannot be called
directly from ordinary Oct code. Normal backend lowering excludes them.

Artifact functions write files explicitly from user code.
Prefer `Artifact.Write*` helpers (`WriteText`, `WriteLines`, `WriteMarkdown`, `WriteCsv`, `WriteJson`, `WriteOctagon`) when authoring `[Artifact]` functions.
`StaticAssert.True`, `StaticAssert.False`, `StaticAssert.Equal`,
`StaticAssert.Near`, and `StaticAssert.Error` validate publication invariants in
this phase without requiring `[Fact]` and without entering the runtime backend.
They are a general language feature and may appear in ordinary deterministic
helpers, including helpers that use loops and table indexing. A call is
evaluated only while its call graph runs under a compiler-owned static phase;
the current concrete phase is `oct artifact`. Ordinary runtime execution does
not degrade a `StaticAssert` into a runtime assertion, and normal backend
lowering excludes artifact entry points. Artifact-phase capability checks still reject ambient
I/O, time, mutation outside declared outputs, and other impure operations;
`Require` remains the bounded refinement-admission mechanism.

`Artifact.WriteCompiledData(path, symbol, value)` sends typed immutable data
directly to the Go static-data backend. The initial backend supports scalars,
arrays, records, tag-only enums, refined scalar Concepts, and record tables.
For a record table it emits a fixed Go row array plus nominal row/enum types and
compiler-owned logical/schema hashes and row count. It emits no decoder,
reflection materializer, `init` function, append loop, or runtime constructor.
Use `Artifact.Checkpoint(label)` and `Artifact.Progress(label, current, total)` for deterministic progress output in long-running artifact functions.

`oct artifact <path> [--output-root <directory>]` selects artifact functions in the entry package by
default, matching `oct test`'s default package scope. Add `--all-packages` only
when imported package artifact lanes are deliberately part of the run. Its
single-target `--json` result reports requested and actual execution, source
provenance, generated path, produced/unchanged status, MIME type, byte count,
and SHA-256.

The actual execution mode is always `build-time-interpreted`: parse, package
resolution, binding, type checking, deterministic discovery, typed interpreter
evaluation, staging, and publication. `--execution compiled` is a temporary
compatibility alias for this same evaluator and never generates, compiles, or
runs a host backend.

Output paths are relative to the explicit output root, which defaults to the
working directory. Empty, absolute, volume-qualified, and escaping paths are
rejected. Duplicate paths are rejected case-insensitively. All selected entry
points finish before publication begins; publication is sorted and each changed
file is replaced from a same-directory temporary file. Equal content is left
untouched. The phase exposes no ambient network, process, clock, operating-system
randomness (`Entropy`), environment, unrestricted filesystem, or wrapper-sidecar
capability. `Random` draws are functions of their seed and remain available. Reads are limited to outputs already
declared in the same phase.

Artifact output cannot add source to the typed program being evaluated. A
generator that feeds a later compilation must remain an explicit staged build
step. Concepts, records, refinements, arrays, enums, units, matching, ordinary
pure functions, and fallibility are ordinary typed Oct semantics and need no
artifact-specific template system.

See [09 builtins](../language/09-builtins.md) for non-test builtin surface.
See [34 octagon](./34-octagon.md) for benchmark and artifact output guidance.
