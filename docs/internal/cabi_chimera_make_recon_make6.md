# MAKE6-DESIGN-RECON — C ABI artifact model for mixed Go/Rust/C builds

Status: design reconnaissance only. This report does **not** implement C ABI helpers, the chimera hello-world example, Prometheus native changes, a Ninja backend, executor semantic changes, new Oct syntax, host capability API changes, CMake compatibility, or default Rust/C/C++/cgo CI gates.

## Executive recommendation

The first C ABI milestone should model C ABI build products as typed Oct data before adding convenience helpers. The smallest useful proof should be an opt-in `Examples/ChimeraHello` project where `oct make` orchestrates raw command targets for a Go final executable that calls a Rust C ABI library through cgo. The first implementation should prefer a Rust `staticlib` on Linux/macOS and a Windows import/DLL path only after the Linux static proof is stable. The optional C layer should be designed into the records but deferred from the first proof unless it is needed to expose a missing field.

The key design rule is: `oct make` should move C ABI artifacts through explicit records (`CAbiLibrary`, `CAbiHeader`, `CAbiConsumer`) and expand helpers into ordinary `CommandTarget`s. It should not hide platform policy in shell strings, require Rust/C toolchains in default CI, or blend tool discovery, link flags, runtime copying, and test execution into one ad hoc command.

## Inputs inspected

Required prior notes:

- `docs/internal/oct_make_design_recon_make0.md`
- `docs/internal/oct_make_status_make5c.md`
- `docs/internal/prometheus_oct_make_dogfood_recon_make5a.md`

Repo surfaces inspected:

- `Libraries/Make`
- `internal/makecmd`
- `internal/prometheus/Make.oct`
- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`
- `internal/prometheus/bridge_dlopen_linux.go`
- `internal/prometheus/bridge_dlopen_windows.go`
- `internal/pkgmgr/wrapper_build.go`
- `tools/build_sidecars`
- wrapper/native lifecycle docs and CLI docs found by targeted `rg`

## Part 1 — Current repo/native precedent survey

### Existing native build output patterns

Prometheus is the main native precedent. Linux builds compile C sources to object files under `out/prometheus/native/obj`, link `out/prometheus/native/libprometheus_reactor.so`, copy that shared library into `internal/prometheus/reactor/libprometheus_reactor.so` for bridge discovery, then build `out/prometheus/native/marionette_tests`, `marionette_slow_tests`, and `marionette_benchmarks`. Windows builds create `out/prometheus/native/prometheus_reactor.dll`, `prometheus_reactor.lib`, PDB files, and `.exe` Marionette binaries, then copy the DLL into `internal/prometheus/reactor/prometheus_reactor.dll`.

The Linux script uses `cc -std=c11 -fPIC -O2 -c`, `cc -shared ... -pthread -lm -lvulkan`, and `c++ -std=c++23 ... -o <binary>`. The Windows script uses `cl /TC`, `link /DLL /IMPLIB`, and `cl /TP ... /link`.

Current native output roots are therefore:

- `out/prometheus/native/` for real generated native artifacts;
- `out/prometheus/native/obj/` for intermediates;
- `internal/prometheus/reactor/` as a checked-in source-tree runtime discovery location populated by native build scripts;
- `out/test-artifacts/` and `out/prometheus/...` for benchmark/test outputs.

### Existing Go build patterns

Go builds appear in three forms:

1. ordinary developer commands such as `go run ./cmd/oct ...` and `go test ...`;
2. wrapper sidecar builds via `go build -o <output> .` from a manifest-declared Go module directory;
3. repo sidecar batch builds via `go build -o <dist/sidecars/name[.exe]> ./cmd/<sidecar>`.

The native wrapper lifecycle writes package-local sidecars under `.oct/wrappers/<goos>-<goarch>/<sidecar-command>[.exe]`. Runtime discovery still requires `OCT_WRAPPER_PATH` or sibling placement; `.oct/wrappers` is not automatically searched.

### Existing sidecar/native output locations

Useful conventions for MAKE7:

| Purpose | Current location pattern | Notes |
| --- | --- | --- |
| Prometheus native outputs | `out/prometheus/native/...` | generated, not committed |
| Prometheus runtime copy | `internal/prometheus/reactor/...` | build scripts copy shared library here |
| repo sidecar batch build | `dist/sidecars/<sidecar>[.exe]` | explicit dev/test helper |
| package wrapper sidecars | `.oct/wrappers/<goos>-<goarch>/<sidecar>[.exe]` | explicit native permission path |
| make state/trace | `<StateDir>/state.octagon`, `<StateDir>/trace.octagon` | default `.octmake`, Prometheus uses `.octmake/prometheus` |

For chimera, prefer a generated example-local path, for example `Examples/ChimeraHello/out/<goos>-<goarch>/...`, rather than copying runtime libraries into source directories.

### Platform-specific suffix conventions

Existing explicit suffix conventions:

- wrapper sidecar executables append `.exe` on Windows;
- sidecar platform directory uses `runtime.GOOS + "-" + runtime.GOARCH`;
- Prometheus Linux shared library is `libprometheus_reactor.so`;
- Prometheus Windows shared library is `prometheus_reactor.dll` plus import library `prometheus_reactor.lib`;
- Marionette Windows binaries end with `.exe`;
- no current macOS `.dylib` precedent was found.

There is no central executable/library suffix helper yet. The sidecar builders locally implement executable suffix logic. A future `Make.Platform()` or `Make.Toolchain()` helper should centralize:

- executable suffix: `""` vs `.exe`;
- shared library filename prefix/suffix: `lib*.so`, `*.dll`, `lib*.dylib`;
- static library suffix: `.a` on Unix, `.lib` on MSVC/Windows;
- import library path for Windows DLLs.

### Current CGO / C ABI usage

Prometheus already uses cgo to dynamically load a C ABI reactor. Linux has build tag `linux && cgo`, links `-ldl`, calls `dlopen`/`dlsym`, and wraps function pointers behind typed Go closures. Windows has build tag `windows && cgo`, uses `LoadLibraryA`/`GetProcAddress`, declares `__cdecl` function pointer types, and exposes the same Go-side loader abstraction.

This is a strong ABI precedent: the stable seam is C types, explicit function signatures, integer status codes, explicit handles, and no ambient linking magic. Chimera should be simpler than Prometheus: one integer-returning Rust function and optional integer-returning C function, with no handles, callbacks, buffers, or strings.

## Part 2 — Proposed C ABI artifact records

The candidate records are close but need a few additions to be platform-complete and consumer-friendly.

Recommended M0 records:

```oct
enum CAbiLibraryKind {
    Static
    Shared
}

enum CAbiCallingConvention {
    C
    Stdcall
}

record CAbiHeader {
    Path: String
    IncludeDir: String
}

record CAbiLibrary {
    Name: String
    Kind: CAbiLibraryKind
    Headers: CAbiHeader[]
    IncludeDirs: String[]
    LibraryPath: String
    LinkName: String
    LinkDirs: String[]
    RuntimeFiles: String[]
    ImportLibraryPath: String
    CallingConvention: CAbiCallingConvention
    Defines: String[]
}

record CAbiConsumer {
    IncludeDirs: String[]
    Defines: String[]
    LinkDirs: String[]
    LinkNames: String[]
    LibraryPaths: String[]
    LinkArgs: String[]
    RuntimeFiles: String[]
}
```

### Field audit

| Need | Candidate sufficiency | Recommendation |
| --- | --- | --- |
| Linux static `.a` | Mostly sufficient | Add `LibraryPaths` or allow `LibraryPath` direct linking; `LinkName` alone assumes `-l<name>` and a dir. |
| Linux shared `.so` | Mostly sufficient | `RuntimeFiles` should include the `.so` if it must be copied beside the executable. |
| Windows static `.lib` | Needs clarification | `.lib` may be a static library or import library; add `ImportLibraryPath` and keep `LibraryPath` as the primary producer artifact. |
| Windows DLL `.dll` | Missing import library distinction | Shared library should carry `LibraryPath` for DLL runtime file and `ImportLibraryPath` for link-time `.lib`. |
| macOS static `.a` | Mostly sufficient | Same as Linux static. |
| macOS shared `.dylib` | Mostly sufficient | Runtime search path/install name issues require `RuntimeFiles` plus future `RpathArgs`/`InstallName` fields; do not add until helper needs them. |
| Go cgo consumer | Missing env-ready shape | `CAbiConsumer` needs `LibraryPaths`/`LinkArgs` so helpers can construct `CGO_CFLAGS` and `CGO_LDFLAGS`. |
| Rust `extern "C"` consumer | Missing build-script hints | Future Rust helpers may need `RustcLinkLib`, `RustcLinkSearch`, and build.rs/cargo env patterns, but M0 can derive from `LinkDirs`/`LinkNames`. |
| C/C++ linker consumer | Needs defines and direct paths | Add `Defines`, `LibraryPaths`, `LinkArgs`; include dirs and headers are enough for compilation. |

Do **not** put tool-specific command strings in the artifact record. The artifact record should describe what exists and how it is consumed. Toolchain helpers should translate it to `CGO_CFLAGS`, `CGO_LDFLAGS`, `cargo:rustc-link-*`, `cc` flags, `cl` flags, `ar`, `lib.exe`, or copy steps.

## Part 3 — Chimera hello-world target design

### Recommended source layout

```text
Examples/ChimeraHello/
  manifest.oct
  Make.oct
  Make.octest
  go/
    main.go
  rust/
    Cargo.toml
    src/lib.rs
    chimera_rust.h
  c/
    hello.c
    hello.h
  README.md
```

For the first implementation, `c/` may exist as documented deferred shape or be omitted. If included in MAKE7, it should not be on the critical path unless the goal explicitly includes C compiler coverage.

### Recommended final output path

Use example-local generated output:

```text
Examples/ChimeraHello/out/<goos>-<goarch>/chimera-hello[.exe]
Examples/ChimeraHello/out/<goos>-<goarch>/runtime/*
Examples/ChimeraHello/out/<goos>-<goarch>/rust/...
```

The exact Rust `target/<profile>` path can stay under `rust/target/` for the raw-command M0, but the Make outputs should name the final library and final executable explicitly so staleness and trace evidence are understandable.

### Recommended targets

| Target | Kind | Purpose |
| --- | --- | --- |
| `CheckTools` | `FunctionTarget` initially, later helper-backed | Safe availability audit for `go`, `cargo`, and optionally `cc`; should not build. |
| `BuildRustCAbi` | `CommandTarget` | `cargo build --release` or profile-selected build of `staticlib`/`cdylib`. |
| `BuildCShim` | `CommandTarget` or deferred | Compile/archive the optional C library if included. |
| `BuildGoBinary` | `CommandTarget` | `go build -o out/<platform>/chimera-hello[.exe] ./go` with cgo env. Current `CommandTarget` cannot set per-command env, so MAKE7 should either use a tiny wrapper script only if unavoidable or recommend a narrow future env field; raw command with ambient env is less reproducible. |
| `RunChimera` | `CommandTarget` | Runs the executable. |
| `TestChimera` | `FlowTarget` or `PhonyTarget` | M0 should be `PhonyTarget` depending on build/run; use `FlowTarget` only if exercising flow evidence is valuable. |
| `Clean` | `FunctionTarget` | Deletes `out/` and maybe Rust target outputs if scoped. |

### Static/shared recommendation

First pass: Rust `staticlib` into Go cgo on Linux/macOS. Reasons:

- no runtime library search/copy problem for M0;
- one final executable is easier evidence;
- the ABI surface remains a C function in a native archive;
- it exposes cgo flags without adding dynamic loader variance.

Shared library path should be documented and tested later because Windows DLL/import-library behavior and macOS runtime lookup are important, but they are not the cleanest first proof.

### Whether to include C in first implementation

Defer C from the first critical path. A Go -> Rust staticlib proof already exercises the most important interop seam: Go final binary, cgo, Rust exporting C ABI, artifact path discovery, link flags, and `oct make` orchestration. Add C as `MAKE8` or as an opt-in second target after the Go/Rust proof is stable. If included in MAKE7, use C only as a tiny integer library linked by Rust and keep it behind `OCT_DOGFOOD_CHIMERA_C=1`.

### Go cgo consumption

The Go file should use cgo with a checked, local header and generated build env:

- `CGO_ENABLED=1`
- `CGO_CFLAGS=-I<rust-or-include-dir> [-I<c-dir>]`
- `CGO_LDFLAGS=<direct library path or -L<dir> -l<name>> [platform args]`

For the first static proof, prefer direct archive path in `CGO_LDFLAGS` if supported by the platform/toolchain because it avoids `-l` naming ambiguity. Keep the cgo preamble tiny and do not pass Go pointers to Rust/C.

### Runtime file copying for shared libraries

If shared libraries are used later, `RuntimeFiles` should be copied to the executable output directory or a child `runtime/` directory with platform-specific run configuration:

- Linux: copy `.so`; either run with `LD_LIBRARY_PATH=<out>` or embed/link rpath later;
- macOS: copy `.dylib`; likely set `DYLD_LIBRARY_PATH` for tests or install name/rpath later;
- Windows: copy `.dll` beside `.exe`; link with `.lib` import library.

Current `CommandTarget` has no per-target environment, so shared-library test execution currently needs either ambient env, wrapper command, or a future `Env` field.

### stdout assertion

Use deterministic stdout containing only integer values, for example:

```text
chimera hello: go=7 rust=35 total=42
```

Avoid strings across the C ABI. Let Go format the final message.

## Part 4 — Target model for chimera

Recommended raw `Make.Plan` shape for MAKE7:

```text
CheckTools       FunctionTarget  no outputs; safe tool availability audit
BuildRustCAbi    CommandTarget   deps: CheckTools; outputs: Rust static library
BuildGoBinary    CommandTarget   deps: BuildRustCAbi; outputs: final executable
RunChimera       CommandTarget   deps: BuildGoBinary; no outputs; runs binary
TestChimera      PhonyTarget     deps: RunChimera; default target
Clean            FunctionTarget  removes generated output
```

If C is included:

```text
BuildCShim       CommandTarget   deps: CheckTools; outputs: C archive/object
BuildRustCAbi    CommandTarget   deps: CheckTools, BuildCShim
```

Avoid shell strings. Use `Program` plus `Args`. If current `CommandTarget` cannot express necessary env values, the report for MAKE7 should either (1) use a tiny checked-in script as an explicit compromise or (2) add only a plan-shape record and keep real build opt-in until `CommandTarget.Env` exists. The better future executor change is an explicit `Env: String[]` or `Env: Make.EnvVar[]` field, but MAKE6 does not implement it.

## Part 5 — Toolchain and platform audit

### Go helper requirements

A future `Make.GoBinaryWithCAbi` helper must know:

- `go build` program path;
- package directory and output path;
- `CGO_ENABLED=1`;
- `CGO_CFLAGS` from include dirs/defines;
- `CGO_LDFLAGS` from library dirs, direct library paths, link names, and extra args;
- build tags if needed;
- executable suffix;
- runtime files to copy before `Run` targets;
- captured stdout/stderr/exit code for test evidence.

### Rust helper requirements

A future `Make.RustCAbiLibrary` helper must know:

- `cargo build` command and manifest/package directory;
- profile (`debug`, `release`);
- crate type (`staticlib`, `cdylib`);
- expected artifact names:
  - Linux: `lib<name>.a`, `lib<name>.so`;
  - macOS: `lib<name>.a`, `lib<name>.dylib`;
  - Windows MSVC: `<name>.lib` for staticlib, `<name>.dll` plus import `<name>.dll.lib` or `<name>.lib` depending cargo/rustc behavior;
- header strategy: manual header for M0, `cbindgen` later;
- ABI safety policy: `extern "C"`, `#[no_mangle]`/unsafe attribute as required by Rust edition, no panic/unwind across boundary.

### C helper requirements

A future C helper must know:

- compiler tool names (`cc`, `clang`, `gcc`, `cl`);
- object suffix (`.o` vs `.obj`);
- static archive tools (`ar rcs` vs `lib.exe /OUT:`);
- shared library flags (`-shared -fPIC` vs `link /DLL`);
- include dirs and defines;
- output naming and import-library naming.

### Minimum tools by platform

Linux M0:

- `go` with cgo support;
- `cargo`/`rustc` if the real opt-in build is enabled;
- a C toolchain usable by cgo (`cc`, `gcc`, or `clang`);
- optional `ar` only if building C static libraries directly.

Windows M0:

- `go`;
- Rust MSVC toolchain through `cargo`;
- MSVC Build Tools (`cl`, `link`, `lib.exe`) or another cgo-compatible C compiler;
- PATH initialized through a developer shell or explicit setup;
- DLL/import-library handling if shared path is tested.

macOS:

- document expected behavior but do not make it default-tested initially;
- likely requires Xcode command line tools, Go, and Rust;
- `.dylib` runtime search is distinct enough to defer real CI until Linux is stable.

### Environment gates

Default tests should not require native interop tools. Suggested gates:

```text
OCT_DOGFOOD_CHIMERA=1      run real chimera build/run
OCT_DOGFOOD_RUST=1         allow Rust toolchain use
OCT_DOGFOOD_CGO=1          allow cgo/native compiler use
OCT_DOGFOOD_CHIMERA_C=1    include optional C shim path
```

Default CI should run only plan-shape, list, dry-run, and trace tests.

## Part 6 — C ABI safety rules

Recommended M0 ABI:

- fixed-width integers where possible (`int32_t`/`uint32_t`) or clearly documented C `int` only for toy examples;
- floats only as scalar values;
- no strings across ABI in M0;
- no heap allocation ownership across ABI;
- no Go pointers crossing into Rust/C;
- no Rust panics crossing the ABI;
- no callbacks, closures, threads, handles, or async tasks;
- no structs by value until alignment/layout tests exist;
- functions return status/value directly and remain deterministic.

For chimera, use integer-returning functions only:

```c
int32_t rust_hello_number(void);
int32_t c_hello_number(void); // deferred optional C layer
```

Docs should warn that C ABI is not a type-safety boundary. Ownership, lifetimes, allocator pairing, string encoding, callback threading, and panic/unwind behavior must be explicit before leaving toy scope.

## Part 7 — Future Make helper design implications

### Helper order

1. Add pure records first: `CAbiHeader`, `CAbiLibraryKind`, `CAbiLibrary`, `CAbiConsumer`.
2. Add `Make.CopyRuntimeLibraries(...)` because it is toolchain-agnostic and useful for shared-library tests.
3. Add `Make.RustCAbiLibrary(...)` for `staticlib` first.
4. Add `Make.GoBinaryWithCAbi(...)` once `CommandTarget` can express env or helper expansion can encode env safely.
5. Add `Make.CStaticLibrary(...)` and then `Make.CSharedLibrary(...)`.

### Too ambitious for the first helper

`Make.Toolchain(...)` as a universal abstraction is too broad for the first implementation. It risks becoming a partial CMake clone. Start with artifact records and narrow helpers that expand to visible targets.

### Raw command targets vs helpers

MAKE7 should use raw `CommandTarget`s first and add only records/docs if implementation scope allows. This validates whether the record fields match reality before encoding helper policy. Helper functions should later expand into `CommandTarget[]` plus pure artifact records, not bypass the plan graph.

### Should `CAbiLibrary` live in `Libraries/Make` before helpers?

Yes. It can be a pure record model in `Libraries/Make` without executor support. `Make.octest` can validate plan shape and record construction without invoking toolchains.

## Part 8 — State/trace needs

### Current trace/state coverage

Current `trace.octagon` can already record:

- backend, make file, selected/default target, profile, state dir, dry-run flag;
- per-target name/kind/status/reason/deps/inputs/outputs;
- command program/args/cwd;
- function/flow names;
- exit code/stdout/stderr/error;
- flow history/result metadata.

Current `state.octagon` can record target name/kind/status, input/output path state, existence, modified time, and empty hash placeholder.

### Future trace extensions

C ABI builds would benefit from structured trace fields rather than inferring everything from command args:

- tool path and optional version;
- toolchain family (`go`, `cargo`, `cc`, `msvc`, etc.);
- library kind (`static`, `shared`);
- header paths and include dirs;
- link dirs, link names, direct library paths, extra link args;
- runtime files copied and destinations;
- cgo env values (`CGO_ENABLED`, `CGO_CFLAGS`, `CGO_LDFLAGS`);
- final executable path;
- shared-library runtime search policy used for a run;
- run stdout/stderr/exit code as first-class test evidence.

Golden success evidence for chimera:

- `oct make --list` shows `CheckTools`, `BuildRustCAbi`, `BuildGoBinary`, `RunChimera`, `TestChimera`, and `Clean`;
- `oct make --dry-run --trace` writes trace with the expected target closure and command args;
- opt-in real run exits 0 and prints the exact deterministic stdout line;
- trace records final run target exit code 0 and stdout.

## Part 9 — Test boundaries and CI gates

Default-testable:

- `Make.octest` pure tests for target names, default target, outputs, deps, and C ABI record construction;
- `oct make --list` for `Examples/ChimeraHello`;
- `oct make --dry-run --trace` and a trace-file existence/shape assertion;
- docs lint only if a lightweight repo docs check exists.

Opt-in only:

- `cargo build`;
- `go build` of the cgo executable;
- any C compiler or archiver invocation;
- executing the chimera binary;
- shared library runtime-copy tests;
- Windows/MSVC and macOS dynamic-library tests.

Eventually useful CI matrix:

- Linux default: plan/list/dry-run only;
- Linux opt-in: Go + Rust staticlib + cgo real run;
- Linux opt-in shared: `.so` runtime copy/loader path;
- Windows opt-in: MSVC Rust toolchain, `.exe`, staticlib or DLL/import library;
- macOS opt-in/manual: staticlib first, `.dylib` later.

Windows-specific risks:

- `.lib` ambiguity between static and import libraries;
- Rust MSVC vs GNU toolchain mismatch with Go cgo compiler;
- developer-shell PATH initialization;
- DLL must be beside `.exe` or on PATH;
- cgo with MSVC can be sensitive to `CC`, `CGO_CFLAGS`, and Go version behavior;
- path quoting and spaces in Visual Studio/Vulkan/SDK paths.

## Part 10 — Recommended next implementation prompt

```text
MAKE7-DOGFOOD-CHIMERA-M0 — add opt-in Go/Rust C ABI hello-world build through oct make

You are working in the Oct repository.

Read first:
- docs/internal/cabi_chimera_make_recon_make6.md
- docs/internal/oct_make_status_make5c.md
- Libraries/Make/README.md
- Language/reference relevant to records/enums/arrays/tests

Task:
Add an opt-in chimera hello-world dogfood example that proves `oct make` can orchestrate a Go final executable calling a Rust C ABI static library through cgo.

Scope:
- Add `Examples/ChimeraHello` with `manifest.oct`, `Make.oct`, `Make.octest`, `go/main.go`, `rust/Cargo.toml`, `rust/src/lib.rs`, and a manual C ABI header.
- Use raw `Make.CommandTarget`s and `Make.FunctionTarget`s first; do not add Make helper functions unless absolutely necessary.
- Add pure `Make.octest` tests for plan shape, default target, outputs/deps, and any pure C ABI record construction if records are added.
- Add opt-in real build/run only when `OCT_DOGFOOD_CHIMERA=1` is set. If additional gates are used, prefer `OCT_DOGFOOD_RUST=1` and `OCT_DOGFOOD_CGO=1`.
- Default tests may run `oct make --list` and `oct make --dry-run --trace`; they must not invoke cargo, rustc, go build, cc, cl, or native compilers.
- Use integer-only ABI: Rust exports an `extern "C"` function returning an integer; Go formats the final stdout. Do not pass strings, pointers, callbacks, structs, or heap ownership across the ABI.
- Prefer Rust `staticlib` for the first real proof so the expected successful result is one runnable executable.
- Produce a tiny executable under `Examples/ChimeraHello/out/<goos>-<goarch>/chimera-hello[.exe]`.
- Expected stdout should be deterministic, for example `chimera hello: go=7 rust=35 total=42`.

Non-goals:
- Do not implement C ABI helper functions beyond pure records if records are needed.
- Do not implement a C shim unless explicitly gated and trivial.
- Do not modify Prometheus native code.
- Do not implement a Ninja backend.
- Do not add new Oct syntax.
- Do not require Rust/C/cgo in default CI.

Tests:
- Run the relevant pure `Make.octest` lane.
- Run `oct make --list` for the example.
- Run `oct make --dry-run --trace` for the example.
- Run the real build/run only if the opt-in environment gates are set and tools are available.
```
