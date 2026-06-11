# CLI

## Overview

`oct` is the primary command surface for compile, run, test, formatting, package scaffolding, package, and experimental remote execution workflows.
`oct build` writes a compiled artifact.
`oct run` executes from source.

## Rules

- `oct run <path>` executes an Oct entry file.
- `oct build <path>` compiles and writes an artifact at `<path>.octbin`.
- `run`, `build`, `test`, and `artifact` share one canonical package import resolver.
- `.octbin` is the compiled binary artifact produced by `oct build`.
- `oct run` executes program behavior and does not require a prebuilt `.octbin`.
- `oct test <path>` runs `.octest` and `.octfail` suites.
- `oct test <path> --suite <name>` runs only tests tagged with `[Suite("<name>")]`.
- `oct test <path> --execution <auto|compiled|interpreted>` selects the test execution mode.
- `auto` is the default test execution mode; `compiled` is a valid test path and requires each selected `.octest` case to run through compiled execution.
- Compiled test execution may build and run generated compiled artifacts internally, but that internal artifact layout is not a user-facing `.octbin` contract. Some packages still contain unsupported compiled features, and missing sidecars can affect compiled wrapper tests.
- `oct artifact <path>` runs `[Artifact]` functions only.
- `oct bench <path>` runs `[Benchmark]` functions only.
- `oct bench <path> --filter <pattern>` runs only benchmarks whose qualified name (`Package.Function`) contains `<pattern>`.
- `oct bench <path> --profile` writes a deterministic Oct-native CPU profile artifact (`bench.cpu.octagon`) for the benchmark run.
- `oct bench <path> --profile --profile-format pprof` emits raw Go `pprof` output (`bench.cpu.pprof`).
- `oct bench <path> --profile --profile-format both` emits both artifacts.
- `oct bench <path> --profile --filter <pattern>` profiles only the filtered benchmark subset.

- Lane roles are intentionally partitioned: `test` = correctness contracts, `bench` = performance measurement, `artifact` = generated evidence outputs.
- Mixed `.octest` files are allowed; each command still executes only its matching lane attributes.
- `oct fmt <path> [--mode readable|compact|en-llm] [--check]` formats one file or a directory tree in place (or checks formatting with `--check`).
- `oct new <experiment|library|wrapper-library> <Name>` creates a deterministic package scaffold in the current working directory.
- `oct new` uses strict PascalCase package names and rejects an existing target directory.
- `oct new wrapper-library` creates manifest wrapper metadata and sidecar reference files but does not build or run the sidecar.
- `oct pkg get <git-url>` fetches one package source into cache.
- `oct pkg list` lists cached package entries.
- `oct pkg sync` syncs explicit-source dependencies and recursively syncs exact-version registry dependencies for the current directory.
- `oct exp run <git-url>` clones and runs an experimental remote package entry workflow.

## Test execution modes

`oct test` supports `--execution auto`, `--execution compiled`, and `--execution interpreted`.
`auto` is the default and may fall back to interpreted execution for individual `.octest` cases when compiled execution is unsupported.
`compiled` is valid and requires each selected `.octest` case to run through compiled execution.
Compiled test execution may build/run generated artifacts internally; users should not rely on a stable test `.octbin` output path.
Some language/library features and wrapper sidecar scenarios may still be unsupported in compiled mode, so compiled parity is demonstrated by the relevant package/test coverage rather than assumed globally.

## Package scaffolding

`oct new` creates deterministic package scaffolds in the current working directory:

```text
oct new library <Name>
oct new experiment <Name>
oct new wrapper-library <Name>
```

The current command has no flags. `<Name>` must be strict PascalCase (`[A-Z][A-Za-z0-9]*`); invalid names are rejected rather than normalized.
The target directory is always `./<Name>`, and the command fails if that target already exists.
`oct new wrapper-library` writes wrapper manifest metadata and sidecar reference files, but it does not build or run the sidecar. The generated package can be inspected with `oct pkg wrappers`.

See also [31 octest](./31-octest.md), [32 ocfmt](./32-ocfmt.md), and [33 oct pkg](./33-oct-pkg.md).

## Examples

```text
oct run App/main.oct
oct build App/main.oct
oct test Language
oct test Language --execution compiled
oct test Language --execution interpreted
oct artifact Language
oct bench Language --octagon-out bench.octagon
oct bench Language --filter HotPath
oct bench Language --filter Main.Fast --profile
oct bench Language --profile --profile-format pprof
oct fmt Language/reference
oct new library SignalTools
oct new experiment BrownNoiseKalman
oct new wrapper-library OpenCV
oct pkg get https://example.com/repo.git
oct pkg list
oct pkg sync
oct exp run https://example.com/repo.git
```
