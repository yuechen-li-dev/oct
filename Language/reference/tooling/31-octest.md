# octest

## Overview

`oct test` executes test contracts from `.octest` and `.octfail`.
`.octest` defines executable passing tests.
`.octfail` defines required compile-time rejection expectations.

`oct test` executes `.octest` functions through the interpreter execution path.
It does not compile `.octest` functions to `.octbin`, and no compiled parity guarantee is claimed for octest helper behavior in the current tooling contract.

## Rules

- `oct test <path>` discovers `.octest` and `.octfail` recursively under `<path>`.
- `.octest` supports test attributes only on functions.
- `[Suite("Name")]` optionally tags a `[Fact]`, `[Theory]`, `[Artifact]`, or `[Benchmark]` function with a suite name.
- `[Suite]` requires a non-empty string literal after trimming.
- Dotted suite names are allowed (example: `"Experiments.FmBrownNoiseKalman.M1"`).
- Repeating `[Suite("...")]` on one function is allowed; the function belongs to all declared suites.
- File-level `[Suite(...)]` is not supported in M0.
- `[Fact]` marks a test function.
- `[Theory]` marks a parameterized test function.
- `[InlineData(...)]` supplies one theory row.
- `[Artifact]` marks an artifact-emission function for `oct artifact`.
- `[Benchmark]` marks a benchmark function for `oct bench`.
- `[Fact]`, `[Artifact]`, and `[Benchmark]` signatures must be `fn Name() -> Void`.
- `[Theory]` signature must be `fn Name(params...) -> Void` with at least one parameter.
- `[Theory]` requires at least one `[InlineData(...)]` row.
- `[InlineData(...)]` is valid only on `[Theory]`.
- `[InlineData(...)]` supports scalar literals and enum values.
- `[Fact]` has fixed cycle time `30.0s` (not overrideable).
- `[Theory]` has default cycle time `30.0s` per row.
- `[CycleTime(t)]` overrides cycle time per `[Theory]` row only.
- `[CycleTime(t)]` requires exactly one positive time quantity argument of type `Float<s>`.
- `[Fact]`, `[Theory]`, `[Artifact]`, and `[Benchmark]` attributes are mutually constrained (invalid combinations are rejected).
- Theory case names use zero-based row indices: `Package.Function[0]`, `Package.Function[1]`, ...
- `oct test` runs `[Fact]`, `[Theory]` rows, and `.octfail` checks.
- `oct test <path> --suite <name>` runs only `[Fact]`/`[Theory]` cases whose suite set contains `<name>`.
- In suite-target mode, unsuited tests are excluded.
- Suite filtering is execution selection only: imports/dependencies still load and typecheck normally.
- Tests from imported packages do not run unless they are explicitly selected by the requested suite.
- Suite filtering currently applies to `oct test` execution; `oct artifact` suite selection is not in this pass.
- `[Fact]` and each `[Theory]` row must execute at least one `Assert.*` call, unless the test row ends with `SkipTest("reason")`; otherwise the test fails with a zero-assert diagnostic.
- Use `Assert.LGTM(<fallible-expression>, "<reason>")` for smoke/completion tests when the main contract is successful completion without runtime error.
- `SkipTest(reason: String) -> Void` is available only in `.octest` `[Fact]` and `[Theory]` test bodies.
- `SkipTest` reason is mandatory and must be non-empty.
- `SkipTest` is terminal for the current `[Fact]` or current `[Theory]` row; statements after it are not executed.
- `SkipTest` marks the test outcome as `SKIP` (not `PASS`).
- `SkipTest` is unavailable in `oct bench` (`[Benchmark]`) and `oct artifact` (`[Artifact]`) lanes.
- Exceeding cycle time is a `FAIL` outcome (not `SKIP`).
- `oct test` executes `.octest` functions through the interpreter path (source execution).
- `.octest` and `.oct` use the same package import resolver and repository package-root search order.
- `oct test` does not build or run a compiled `.octbin` test artifact.
- Compiled parity for `.octest` helper behavior is not a current contract.
- `oct test` does not run `[Artifact]` or `[Benchmark]` functions.
- `oct artifact <path>` runs `[Artifact]` functions only.
- `oct bench <path>` runs `[Benchmark]` functions only.
- Assertion-count requirements apply only to `oct test` `[Fact]`/`[Theory]`; `[Artifact]` and `[Benchmark]` do not require assertions.
- M4 does not change `[Artifact]` or `[Benchmark]` timeout policy.
- `oct bench <path> --filter <pattern>` runs only benchmarks whose qualified name (`Package.Function`) contains `<pattern>`.
- `oct bench <path> --profile` emits a deterministic `bench.cpu.octagon` benchmark profile artifact by default.
- `oct bench <path> --profile --profile-format pprof` emits raw `bench.cpu.pprof` output.
- `oct bench <path> --profile --profile-format both` emits both `.octagon` and `.pprof`.
- `oct bench <path> --filter <pattern> --profile` profiles only the filtered subset.
- `oct bench` with `--filter` fails if no benchmark names match the provided pattern.
- `.octfail` requires one header line: `expect error: "<non-empty substring>"`.
- `.octfail` passes when compilation fails and the error contains the declared substring.
- `.octfail` fails on missing rejection, malformed header, or mismatch.


## Lane policy (M5)

- `[Fact]` / `[Theory]` are correctness-contract lanes.
- `[Benchmark]` is a measurement lane (performance/timing), not a correctness-proof lane.
- `[Artifact]` is a code-driven artifact-generation lane (evidence/scratch outputs), not a correctness-test lane.

### Mixed-file partitioning

A single `.octest` file may contain multiple lanes, but each CLI command executes only its lane:

- `oct test` runs `[Fact]` / `[Theory]` + `.octfail`.
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
    WriteOctagon("out/reference.octagon", data)
}
```

### Assertion and timeout scope

- `[Fact]` and `[Theory]` require assertions or `SkipTest("reason")`.
- `[Benchmark]` and `[Artifact]` do not require assertions.
- `[Fact]` has fixed `30.0s` cycle time; `[Theory]` rows default to `30.0s` and can opt into `[CycleTime(...)]`.
- Current timeout policy for `[Benchmark]` and `[Artifact]` is unchanged/deferred.

### Artifact output policy

- Artifact functions write files explicitly from user code.
- Prefer `Artifact.Write*` for artifact emission (`WriteText`, `WriteLines`, `WriteMarkdown`, `WriteCsv`, `WriteJson`, `WriteOctagon`) when authoring `[Artifact]` functions.
- Use `Artifact.Checkpoint(label)` for explicit phase boundaries and `Artifact.Progress(label, current, total)` for deterministic progress emission in long-running artifact functions.
- `Artifact.Progress` signature is fixed: `(label: String, current: Int, total: Int)`. For message-only phase markers, use `Artifact.Checkpoint(label)`.
- `oct artifact` prints plain CI-friendly lines (`CHECKPOINT <Function>: <label>`, `PROGRESS <Function>: <label> <current>/<total>`); this is runner output and does not change artifact file contents.
- Progress/checkpoint calls are explicit (no automatic loop inference), non-fallible, and become no-op when no artifact progress recorder is configured.
- Artifact lane execution does not impose a mandatory output directory.
- Keep compute helpers pure in normal test lanes (no `Artifact.*` or write side effects); call artifact IO and progress only from `[Artifact]` entrypoints.

### `.octagon` guidance

- `.octagon` artifacts are intended to be human-readable, LLM-readable, and Oct-type-shaped evidence.
- Schema/version metadata is encouraged for long-lived or machine-consumed artifacts, but not mandatory.
- Reports summarize conclusions; artifacts preserve the underlying evidence.

## Assert helpers (`.octest` test-only surface)

These helpers are intended for `.octest` tests and are not part of general runtime program semantics:

- `Assert.True(condition: Bool, message: String) -> Void`
- `Assert.False(condition: Bool, message: String) -> Void`
- `Assert.Equal(expected: T, actual: T, message: String) -> Void`
- `Assert.Near(expected: Float, actual: Float, tolerance: Float, message: String) -> Void`
- `Assert.Error(expr: T ! Error, message: String) -> Void`
- `Assert.LGTM(expr: T ! Error, message: String) -> T`

See [09 builtins](../language/09-builtins.md) for non-test builtin surface.

See also [34 octagon](./34-octagon.md) for benchmark artifact output via `--octagon-out`.

## Examples

Valid:

```oct
package Main

[Theory]
[InlineData(1, 2)]
[InlineData(3, 4)]
fn Pair(a: Int, b: Int) -> Void {
    Assert.True(a < b, "ordered")
}
```

```text
oct test Language
oct artifact Language
oct bench Language --octagon-out bench.octagon
oct bench Language --filter MatrixMul
oct bench Language --filter Signal.HotPath --profile
oct bench Language --profile --profile-format pprof
```

Invalid:

```text
expect error: ""
package Main
fn Main() -> Int { return 0 }
```
