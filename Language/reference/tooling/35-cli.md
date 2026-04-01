# CLI

## Overview

`oct` is the primary command surface for compile, run, test, formatting, package, and experimental remote execution workflows.
`oct build` writes a compiled artifact.
`oct run` executes from source.

## Rules

- `oct run <path>` executes an Oct entry file.
- `oct build <path>` compiles and writes an artifact at `<path>.octbin`.
- `.octbin` is the compiled binary artifact produced by `oct build`.
- `oct run` executes program behavior and does not require a prebuilt `.octbin`.
- `oct test <path>` runs `.octest` and `.octfail` suites.
- `oct artifact <path>` runs `[Artifact]` functions only.
- `oct bench <path>` runs `[Benchmark]` functions only.
- `oct bench <path> --filter <pattern>` runs only benchmarks whose qualified name (`Package.Function`) contains `<pattern>`.
- `oct bench <path> --profile cpu` writes a deterministic CPU profile artifact for the benchmark run.
- `oct bench <path> --profile cpu --filter <pattern>` profiles only the filtered benchmark subset.
- `oct fmt <path>` formats one file or a directory tree in place.
- `oct pkg get <git-url>` fetches one package source into cache.
- `oct pkg list` lists cached package entries.
- `oct pkg sync` syncs direct manifest dependencies for the current directory.
- `oct exp run <git-url>` clones and runs an experimental remote package entry workflow.

See also [31 octest](./31-octest.md), [32 ocfmt](./32-ocfmt.md), and [33 oct pkg](./33-oct-pkg.md).

## Examples

```text
oct run App/main.oct
oct build App/main.oct
oct test Language
oct artifact Language
oct bench Language --octagon-out bench.octagon
oct bench Language --filter HotPath
oct bench Language --filter Main.Fast --profile cpu
oct fmt Language/reference
oct pkg get https://example.com/repo.git
oct pkg list
oct pkg sync
oct exp run https://example.com/repo.git
```
