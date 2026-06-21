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
- `oct init <experiment|library|wrapper-library>` creates `manifest.oct` in the current existing directory and refuses to overwrite an existing manifest.
- `oct new` and `oct init` use strict PascalCase package names; `oct new` rejects an existing target directory, and `oct init` derives the name from the current directory basename.
- `oct new wrapper-library` creates manifest wrapper metadata and sidecar reference files but does not build or run the sidecar.
- `oct pkg get <git-url>` fetches one package source into cache.
- `oct pkg list` lists cached package entries.
- `oct pkg registry add/list/remove` manages local registry configuration for the current project.
- `oct pkg add <Name>@<exact-version>` adds an exact registry dependency to `manifest.oct`.
- `oct pkg sync` syncs explicit-source dependencies and recursively syncs exact-version registry dependencies for the current directory.
- `oct pkg lock` writes an optional project-root `lock.octagon`; `oct pkg sync --locked` syncs the locked graph.
- `oct pkg wrappers` inspects wrapper metadata without building sidecars.
- `oct pkg build-wrappers --allow-native` explicitly builds declared native wrapper sidecars.
- `oct version` and `oct --version` print the CLI version surface.
- `oct exp run <git-url>` clones and runs an experimental remote package entry workflow.


## v0.1 package-manager commands

The canonical first-party registry is local/source-controlled at `Registry/registry.oct`; it is not hosted in v0.1. Configure a project with:

```text
oct pkg registry add oct <path-to-oct-repo>/Registry
```

`Mathematics` is the canonical math package name. There is no `Math` alias.

Common package commands:

```text
oct pkg registry add oct <path-to-oct-repo>/Registry
oct pkg registry list
oct pkg registry remove oct
oct pkg add Mathematics@0.1.0
oct pkg sync
oct pkg lock
oct pkg sync --locked
oct pkg wrappers
oct pkg build-wrappers --allow-native
```

Package sync does not build wrapper sidecars. Built sidecars require explicit native build permission and runtime discovery through `OCT_WRAPPER_PATH` or sibling discovery.

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

`oct init` initializes an existing current directory by writing only `manifest.oct`:

```text
oct init library
oct init experiment
oct init wrapper-library
```

`oct init` derives the package name from the current directory basename, uses the same manifest conventions as `oct new`, and refuses to overwrite an existing manifest. Existing experiment folders should use `oct init experiment`.
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
oct pkg registry add oct <path-to-oct-repo>/Registry
oct pkg add Mathematics@0.1.0
oct pkg sync
oct pkg lock
oct pkg sync --locked
oct pkg wrappers
oct pkg build-wrappers --allow-native
oct version
oct exp run https://example.com/repo.git
```

## `Make.oct` attributes

`Make.oct` has a small, closed attribute surface for Make tooling. These attributes are valid only in a file whose base name is exactly `Make.oct`; ordinary `.oct` files still reject attributes, and `.octest` files continue to accept only Octest attributes.

Supported Make attributes are:

- `[MakePlan]`
- `[Pure]`
- `[NoWhile]`
- `[RequiresAuthority]`

They are compiler/tool-owned semantic markers, not decorators, macros, reflection metadata, user-defined attributes, or a general metaprogramming system. They do not accept payloads.

Make attributes attach only to function declarations:

```oct
[MakePlan]
[Pure]
[NoWhile]
fn Plan() -> Make.Plan {
    return Make.Plan { ... }
}

[RequiresAuthority]
fn CheckTools() -> Int ! Error {
    let _go = Make.Tool("go")?
    return 0
}
```

`[MakePlan]` marks the conventional Make plan function and must be written on `fn Plan()` with zero parameters and return type `Make.Plan`. A conventional unmarked `fn Plan() -> Make.Plan` remains valid.

`[NoWhile]` is a syntactic restriction: a marked function body must not contain any `while` statement, including nested `while` statements.

`[Pure]` and `[RequiresAuthority]` are metadata-only in the first Make attribute pass. `[Pure]` does not yet enforce an effect-purity system, and `[RequiresAuthority]` is not yet required for Make host primitive calls. They may not be combined on the same function.
