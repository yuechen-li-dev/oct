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
- `[Fact]`, `[Theory]`, `[Artifact]`, and `[Benchmark]` attributes are mutually constrained (invalid combinations are rejected).
- Theory case names use zero-based row indices: `Package.Function[0]`, `Package.Function[1]`, ...
- `oct test` runs `[Fact]`, `[Theory]` rows, and `.octfail` checks.
- `[Fact]` and each `[Theory]` row must execute at least one `Assert.*` call, unless the test row ends with `SkipTest("reason")`; otherwise the test fails with a zero-assert diagnostic.
- Use `Assert.LGTM(<fallible-expression>, "<reason>")` for smoke/completion tests when the main contract is successful completion without runtime error.
- `SkipTest(reason: String) -> Void` is available only in `.octest` `[Fact]` and `[Theory]` test bodies.
- `SkipTest` reason is mandatory and must be non-empty.
- `SkipTest` is terminal for the current `[Fact]` or current `[Theory]` row; statements after it are not executed.
- `SkipTest` marks the test outcome as `SKIP` (not `PASS`).
- `SkipTest` is unavailable in `oct bench` (`[Benchmark]`) and `oct artifact` (`[Artifact]`) lanes.
- `oct test` executes `.octest` functions through the interpreter path (source execution).
- `.octest` and `.oct` use the same package import resolver and repository package-root search order.
- `oct test` does not build or run a compiled `.octbin` test artifact.
- Compiled parity for `.octest` helper behavior is not a current contract.
- `oct test` does not run `[Artifact]` or `[Benchmark]` functions.
- `oct artifact <path>` runs `[Artifact]` functions only.
- `oct bench <path>` runs `[Benchmark]` functions only.
- Assertion-count requirements apply only to `oct test` `[Fact]`/`[Theory]`; `[Artifact]` and `[Benchmark]` do not require assertions.
- `oct bench <path> --filter <pattern>` runs only benchmarks whose qualified name (`Package.Function`) contains `<pattern>`.
- `oct bench <path> --profile` emits a deterministic `bench.cpu.octagon` benchmark profile artifact by default.
- `oct bench <path> --profile --profile-format pprof` emits raw `bench.cpu.pprof` output.
- `oct bench <path> --profile --profile-format both` emits both `.octagon` and `.pprof`.
- `oct bench <path> --filter <pattern> --profile` profiles only the filtered subset.
- `oct bench` with `--filter` fails if no benchmark names match the provided pattern.
- `.octfail` requires one header line: `expect error: "<non-empty substring>"`.
- `.octfail` passes when compilation fails and the error contains the declared substring.
- `.octfail` fails on missing rejection, malformed header, or mismatch.

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
