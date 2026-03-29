# octest

## Overview

`oct test` executes test contracts from `.octest` and `.octfail`.
`.octest` defines executable passing tests.
`.octfail` defines required compile-time rejection expectations.

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
- `oct test` does not run `[Artifact]` or `[Benchmark]` functions.
- `oct artifact <path>` runs `[Artifact]` functions only.
- `oct bench <path>` runs `[Benchmark]` functions only.
- `.octfail` requires one header line: `expect error: "<non-empty substring>"`.
- `.octfail` passes when compilation fails and the error contains the declared substring.
- `.octfail` fails on missing rejection, malformed header, or mismatch.
- `Assert.True`, `Assert.False`, `Assert.Equal`, and `Assert.Near` are the supported assert builtins.

See also [16 octagon](./16-octagon.md) for benchmark artifact output via `--octagon-out`.

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
```

Invalid:

```text
expect error: ""
package Main
fn Main() -> Int { return 0 }
```
