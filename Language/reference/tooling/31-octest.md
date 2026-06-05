# octest

## Overview

`oct test` executes test contracts from `.octest` and `.octfail` files.
`.octest` files are ordinary Oct package files with test-lane metadata; they use normal `package`, `import`, typechecking, and package-root resolution rules.
`.octfail` files are negative compile-time contracts: they pass only when the declared error substring is produced.

`oct test <path>` discovers `.octest` and `.octfail` files recursively under `<path>`.
A single `.octest` file may also contain `[Artifact]` and `[Benchmark]` functions, but `oct test` runs only `[Fact]` and `[Theory]` cases plus `.octfail` checks.
Use `oct artifact` for `[Artifact]` functions and `oct bench` for `[Benchmark]` functions.

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

`.octfail` is different from a runtime failure test.
An `.octfail` file requires a first line of `expect error: "<non-empty substring>"` and passes when compilation/typechecking fails with an error containing that substring.
It fails on missing rejection, malformed header, or substring mismatch.

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
In `auto`, the runner first tries compiled execution for each `.octest` case and falls back to interpreted execution when compiled execution is unsupported for that case.
In `compiled`, a test case must run through the compiled test path or it fails.
In `interpreted`, tests run through source interpretation.

Compiled test execution may build and run generated compiled artifacts internally, but users should treat this as a test execution mode rather than a stable artifact layout.
Some packages still use language/library features that are not compiled-supported, and missing wrapper sidecars can affect compiled wrapper tests.
Interpreted and compiled parity is tracked by package and test coverage, so do not assume every test package compiles until it has been run in compiled mode.
`.octfail` remains a compile-time rejection check; it is not a compiled runtime test case.

## File and layout conventions

- `.octest` and `.oct` files use the same package import resolver and repository package-root search order.
- `.octest` supports test-lane attributes only on functions.
- `[Fact]`, `[Theory]`, `[Artifact]`, and `[Benchmark]` attributes are mutually constrained; invalid combinations are rejected.
- `[Artifact]` and `[Benchmark]` functions must use `fn Name() -> Void` and do not require assertions.
- Current Language fixtures commonly organize accepted behavior under `valid/` and rejected behavior under `invalid/`, with runtime-pass cases often under `runtime/valid/`.
- Selecting a single `.octest` file limits execution to that selected source file; selecting a directory discovers tests recursively under that directory.

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

Artifact functions write files explicitly from user code.
Prefer `Artifact.Write*` helpers (`WriteText`, `WriteLines`, `WriteMarkdown`, `WriteCsv`, `WriteJson`, `WriteOctagon`) when authoring `[Artifact]` functions.
Use `Artifact.Checkpoint(label)` and `Artifact.Progress(label, current, total)` for deterministic progress output in long-running artifact functions.

See [09 builtins](../language/09-builtins.md) for non-test builtin surface.
See [34 octagon](./34-octagon.md) for benchmark and artifact output guidance.
