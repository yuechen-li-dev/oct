# Artifacts, static checks, and benchmarks

[Reference home](../README.md) · [Testing](testing.md) · [Octagon](octagon.md)

Correctness tests, performance measurements, and generated outputs have
separate explicit entry points. They can share ordinary typed helper functions.

| Attribute | Command | Purpose |
| --- | --- | --- |
| `[Fact]`, `[Theory]` | `oct test` | Assert behavior and expected errors |
| `[Benchmark]` | `oct bench` | Measure a compiled benchmark invocation |
| `[Artifact]` | `oct artifact` | Evaluate and publish typed build-time outputs |

All these entries belong in `.octest` source. Make tooling additionally permits
artifact entries in `Make.oct`. Importing a package does not run its artifacts.

## A complete example

<!-- verified-example: ../../ReferenceExamples/valid/publication.octest -->
```oct
package ReferenceExamples

[Artifact]
fn PublishReferenceSummary() -> Void {
    let squares = [1, 4, 9]
    StaticAssert.Equal(3, Len(squares), "publication has three samples")
    Artifact.WriteOctagon("reference/squares.octagon", squares)
    Artifact.WriteText("reference/summary.txt", "Three squared samples")
}

[Benchmark]
fn MeasureReferenceCalculation() -> Void {
    var total = 0
    for i in 0..10000 { total = total + i }
    Assert.Equal(49995000, total, "consume and check the calculated result")
}
```

From the repository root:

```text
go run ./cmd/oct artifact Language/ReferenceExamples/valid --output-root .tmp/reference-output --json
go run ./cmd/oct bench Language/ReferenceExamples/valid --filter MeasureReferenceCalculation
```

`oct test` does not run either function above. The other lesson files hold the
correctness tests. Artifact paths are relative to the explicit output root,
so the first command publishes `.tmp/reference-output/reference/squares.octagon`.

## Publication is a phase

Artifact discovery follows successful loading, binding, and typechecking.
The typed build-time interpreter evaluates the selected entries, stages their
outputs, then publishes in deterministic path order. It does not generate an
application backend or invoke the Go compiler. `--execution compiled` on
`oct artifact` is a compatibility alias for this evaluator and reports that
delegation; it is not native-execution evidence.

Use `Artifact.WriteText`, `WriteLines`, `WriteMarkdown`, `WriteCsv`, `WriteJson`,
and `WriteOctagon` for outputs. Document values can also use the admitted
`Artifact.Markdown`, `Docx`, `Latex`, and `Pdf` renderers. Their renderer and
external-tool prerequisites are specified with the [Document library](../../../Libraries/Document/).
Typed markup is optional: [Oct-XML](../language/typed-markup.md) elaborates into
ordinary calls that can build those values.

Outputs cannot escape their root, collide case-insensitively, or create new
source in the program currently being evaluated. All entries finish before
publication. Equal content is left unchanged. The JSON result reports source
provenance, paths, byte counts, media types, hashes, and publication status.
`Artifact.Checkpoint` and `Artifact.Progress` provide deterministic progress
labels rather than an ambient clock.

Runtime filesystem or process APIs do not become general artifact authority.
Readback of outputs already declared in the phase has an explicit confined
path. Protected native operations require declared capability requests and
separate host grants; see [capability requests](../language/concepts-and-refinements.md#capability-requests-concepts-m2)
and the detailed [artifact rules](testing.md#lane-policy).

## Require, Assert, and StaticAssert

| Facility | When | What to use it for |
| --- | --- | --- |
| `Require(condition, explanation)` | Bounded compile-time proof/admission | Requirements the checker can prove; refined Concept construction |
| `Assert.*` | Executing a test/runtime assertion | Observable correctness and deliberately fatal runtime checks |
| `StaticAssert.*` | Executing a helper in a compiler-owned static phase | Publication invariants, including values computed by ordinary loops/functions |

`StaticAssert.True`, `False`, `Equal`, `Near`, and `Error` may be written in
ordinary deterministic helpers. They are evaluated only when that call graph
runs under the static phase, currently `oct artifact`. They do not silently
become runtime assertions outside it. Static evaluation still enforces the
phase's effect and authority boundaries; it is not unrestricted compile-time
host execution. Use ordinary fallible checks for dynamic input.

## Typed compiled data

`Artifact.WriteCompiledData(path, symbol, value)` publishes checked immutable
data directly as Go static data. The admitted initial forms include scalars,
arrays, records, tag-only enums, refined scalar Concepts, and record tables.
The table representation carries schema/logical hashes and row count, rather
than a startup decoder or runtime construction loop.

This is an explicit staged input to a later Go build, not language self-hosting
or a new runtime table type. The [compiled-data contracts](../../Tooling/CompiledData/valid/)
cover publication and proof scope, and the [backend note](../../../docs/internal/compiled_data_m0.md)
explains its current representation and limits.

## Benchmarking honestly

`[Benchmark]` functions take no parameters and return `Void`. The current
runner builds a compiled invocation; it does not silently fall back to the
interpreter. Its reported duration includes launching and running that
benchmark executable. Compilation is a separate preparation step. A single
short benchmark is therefore not a calibrated per-iteration throughput claim.

The example's assertion both consumes and checks its result, and its time is
part of the measurement. For performance work, choose enough useful work,
repeat measurements, keep the toolchain/environment recorded, and compare
equivalent results. Distinguish process-wall time from a kernel-only host
benchmark. The [scalar-loop investigation](../../../docs/internal/compiled_scalar_loop_recon.md)
demonstrates that distinction with generated Go and a handwritten control.

`--filter` selects a qualified-name substring. `--profile` emits an Oct-native
CPU profile; `--profile-format pprof` or `both` selects additional formats.
See the [CLI](cli.md) for output flags. A profile of the host runner is not
automatically a CPU profile of every native child process.
