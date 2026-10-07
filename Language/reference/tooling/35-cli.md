# CLI

## Overview

`oct` is the primary command surface for compile, run, test, formatting, package scaffolding, package, and experimental remote execution workflows.
`oct build` writes a compiled artifact.
`oct run` executes from source.

## Rules

- `oct run <path>` executes an Oct entry file.
- `oct check <go-package-directory>` validates an optional typed Go semantic
  companion and any committed static bridge without executing Octest. The
  experimental `--generate` mode explicitly writes that deterministic bridge.
- `oct build <path>` compiles and writes a native executable beside the entry
  source. On Windows, `Main.oct` produces `Main.oct.exe`; on Linux and macOS
  the executable has no suffix. `.octbin` is reserved for a future portable
  Oct-owned format and is not emitted by the current GoOct backend.
- `run`, `build`, `test`, and `artifact` share one canonical package import resolver.
- `.octbin` is reserved for a future portable artifact format; it is not a
  current `oct build` output.
- `oct build <path> --target wasm` lowers the same current MIR directly to a
  deterministic core WebAssembly `.wasm` module beside the entry source. It
  does not invoke Go, TinyGo, WAT tooling, WASI, or a browser API. The command
  reports the output path, target, and SHA-256. M0 supports the scalar subset
  documented in `docs/internal/wasm_backend_m0.md`.
- `oct build <path> [--target native|wasm] --opt` runs the opt-in,
  backend-neutral scalar constant optimizer before emission. The default build
  remains unoptimized; `--opt` does not select a multi-level optimization
  policy.
- `oct run` executes program behavior and does not require a prebuilt `.octbin`.
- `oct test <path>` runs `.octest` and `.octfail` suites.
- `oct test <path> --suite <name>` runs only tests tagged with `[Suite("<name>")]`.
- `oct test <path> --execution <auto|compiled|interpreted>` selects the test execution mode.
- `oct test <path> --json` emits one `oct.cli.result.v1` result for a single target, including normalized command diagnostics, discovered test files, counts, execution/fallback information, timing, and exit status.
- `auto` is the default test execution mode; `compiled` is a valid test path and requires each selected `.octest` case to run through compiled execution.
- Compiled test execution may build and run generated compiled artifacts internally, but that internal artifact layout is not a user-facing `.octbin` contract. Some packages still contain unsupported compiled features, and missing sidecars can affect compiled wrapper tests.
- `oct artifact <path> [--output-root <directory>]` type-checks the selected graph, discovers `[Artifact]` functions in stable package/file/function order, evaluates them through the build-time typed interpreter, and publishes confined outputs. Use `--all-packages` to include imported package artifact lanes.
- Artifact evaluation never requires application backend generation or host compilation. The retained `--execution compiled` spelling delegates to the same build-time interpreter for compatibility and reports that delegation.
- `oct artifact <path> --json` emits one `oct.cli.result.v1` result with requested/actual execution plus exact source provenance, paths, produced/unchanged status, MIME types, sizes, and SHA-256 hashes.
- `oct bench <path>` runs `[Benchmark]` functions only.
- `oct bench <path> --filter <pattern>` runs only benchmarks whose qualified name (`Package.Function`) contains `<pattern>`.
- `oct bench <path> --profile` writes a deterministic Oct-native CPU profile artifact (`bench.cpu.octagon`) for the benchmark run.
- `oct bench <path> --profile --profile-format pprof` emits raw Go `pprof` output (`bench.cpu.pprof`).
- `oct bench <path> --profile --profile-format both` emits both artifacts.
- `oct bench <path> --profile --filter <pattern>` profiles only the filtered benchmark subset.

- Lane roles are intentionally partitioned: `test` = correctness contracts, `bench` = performance measurement, `artifact` = generated evidence outputs.
- Mixed `.octest` files are allowed; each command still executes only its matching lane attributes.
- `oct fmt <path> [--mode en-llm|en-llm-compact] [--arrows keep|thin|fat] [--check]` formats one file or a directory tree in place (or checks formatting with `--check`).
- `oct json infer <file.json> [--name <Name>] [--explain]` prints the declarations a JSON document loads into; see [`oct json infer`](#oct-json-infer).
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

## `oct json infer`

```text
oct json infer tickets.json [--name Ticket] [--explain]
```

A program does not infer the shape of JSON: it declares a type, and
`Json.Load<T>` reads the document as that type (see
[17 standard libraries](../language/17-standard-libraries.md)). This command
reads a document the other way round, once, and prints the declarations to
start from.

```oct
record table Tickets {
    Id: String
    Assignee: Option<String>
    Points: Int
}

// Json.Load<Tickets>("tickets.json")?
```

Every line of the output is Oct source or a comment, so it can be pasted as
it is. The last line is the call that loads the document.

- An object is a `record`. Its fields are its keys as Oct fields are
  written: `read_timeout_ms` is `ReadTimeoutMs`, which Json matches back to
  the key.
- An array of objects is a `record table`. Where a table cannot be declared,
  in the cell of a table or the element of an array, it is an array of a
  record.
- A member that is `null` or absent in some objects is an `Option<T>`.
- An object whose keys are data, not field names, is a keyed
  `record table` with the columns `Key` and `Value`, or `Key` and the members
  of its values when those are objects.
- A number is an `Int` when it has no fraction and no exponent, and a place
  that holds both is a `Float`. Rows of numbers, all one length, are a
  `Matrix<Float>`; other arrays of arrays stay arrays.
- A `String` that takes few values has the enum it could be in a comment.
  No enum is declared.
- Where no value says what a type is, an `Option<String>` or a `String[]` is
  printed with a comment that says `String` is a placeholder: a member that
  is `null` everywhere, an array that is empty everywhere.
- The root declaration is named by `--name`, or after the file. The others
  are named after their keys.

Two choices have several signals and no rule: whether an object is a record
or a keyed table, and whether an array of objects is a table or a tagged
array. Each is scored, and `--explain` prints the scores as comments after
the declarations. A record wins a tie, and so does a table. Keys that read
as field names weigh most: an object of nine names with a count each is
printed as a record of nine fields.

A value with no declaration is listed with its place, the declarations
around it are still printed, and the command fails:

- a tagged array, where a member such as `type` says which other members an
  object has. Json reads no enum that carries a payload;
- an array or a column whose values are of different kinds;
- an object that can be neither a record nor a table: a key no field name
  can match (`$schema`), two keys that are one field name, a key written
  twice.

The output is the same for the same document. Contracts:
`Language/Tooling/JsonInfer`, where the output for each of 26 documents is
pasted and the document is loaded with it in both lanes.

## Examples

```text
oct run App/main.oct
oct build App/main.oct
oct build Examples/WasmCompute --target wasm
oct build Examples/WasmCompute --target wasm --opt
oct test Language
oct test Language --execution compiled
oct test Language --execution interpreted
oct test Language/Types/UnitsM1/valid --execution auto --json
oct artifact Language
oct artifact Experiments/OctErgonomicsLab/M0 --output-root out/generated --json
oct bench Language --octagon-out bench.octagon
oct bench Language --filter HotPath
oct bench Language --filter Main.Fast --profile
oct bench Language --profile --profile-format pprof
oct fmt Language/reference
oct json infer tickets.json --name Ticket --explain
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

`[RequiresAuthority]` is required on any `Make.oct` function that directly calls a Make host primitive such as `Make.Exec`, `Make.ExecIn`, `Make.Tool`, `Make.Env`, file/directory primitives, globbing, timestamp, or hashing helpers. This authority check is direct-call-only: it does not yet follow helper calls transitively, and target/config/plan record constructors such as `Make.CommandTarget { ... }` and `Make.Plan { ... }` are data construction, not host primitive calls. `[Pure]` is also direct-evidence-only in this release: a `[Pure]` function that directly calls a Make host primitive gets a purity-specific hard error explaining that the host read/write/process/tool operation must move to a `[RequiresAuthority]` helper or be passed in as data. `[Pure]` does not imply a broad language-wide effect system or transitive call-graph proof. `oct make doctor` warns when a `[Pure]` function directly calls an unmarked helper whose purity is unknown, and does not warn for directly called helpers marked `[Pure]`. Deterministic failure with `error(...)`, command strings as data, target construction, C ABI metadata construction, and ordinary `match`, `switch`, `if`, `for`, `while`, and `when utility` control flow are allowed by `[Pure]`; `[NoWhile]` remains the separate while-loop policy. `[Pure]` and `[RequiresAuthority]` may not be combined on the same function.

Repository Make examples dogfood these markers: `Plan()` is marked `[MakePlan] [Pure] [NoWhile]` when it only constructs plan data, data-only target/config helpers are marked `[Pure] [NoWhile]`, and functions that call Make host primitives such as `Make.Tool`, `Make.Env`, or `Make.Remove` are marked `[RequiresAuthority]`.
