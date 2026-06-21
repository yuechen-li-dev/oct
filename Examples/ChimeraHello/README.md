# ChimeraHello

ChimeraHello is an experimental `oct make` C ABI interop proof for MAKE7-CHIMERA-M0. It is intentionally small: a Go final executable calls a Rust `staticlib` through cgo and the C ABI, then Go owns the final formatted output.

Expected real-run output:

```text
chimera hello: go=7 rust=35 total=42
```

## Scope

- Go is the final executable.
- Rust exports one `extern "C"` integer-only function: `rust_hello_number() -> 35`.
- The ABI passes no strings, pointers, callbacks, structs, slices, or heap ownership.
- `oct make` orchestrates the raw targets.
- `Make.oct` also declares the Rust static library as a `Make.CAbiLibrary` metadata value.
- The active M0 path is Go + Rust only; the C shim layer is deferred.
- JSON, CMake, Ninja, shared-library runtime placement, and universal toolchain helpers are not involved.

## Opt-in gate

Default-safe validation must not require Rust, Cargo, cgo, a C compiler, or a native link step. The real native build is gated by:

```sh
OCT_CHIMERA_HELLO=1
```

Without that variable, the `CheckTools` target stops before the Rust or cgo build. Real execution requires Go, Cargo/Rust, cgo, and a platform C toolchain usable by Go.

## Targets

- `CheckTools` — verifies the opt-in gate and required tools for a real build.
- `BuildRustCAbi` — runs `cargo build --release` for the Rust `staticlib`.
- `BuildGoBinary` — runs `go build` with structured `CGO_ENABLED=1` command environment.
- `RunChimera` — runs the final executable.
- `TestChimera` — default phony target depending on `RunChimera`.
- `Clean` — removes generated example output and make state.

Generated binaries are placed under `Examples/ChimeraHello/out/<goos>-<goarch>/chimera-hello` on Unix-like hosts. Windows `.exe` naming is deferred for this M0 example; the plan keeps list and dry-run stable while documenting that limitation.

## Safe validation

From the repository root:

```sh
go run ./tools/build_sidecars --out dist/sidecars

go run ./cmd/oct test Examples/ChimeraHello/Make.octest --execution interpreted
go run ./cmd/oct test Examples/ChimeraHello/Make.octest --execution compiled

OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file Examples/ChimeraHello/Make.oct --list

OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file Examples/ChimeraHello/Make.oct --dry-run --trace
```

## Real build and run

Only run this when the local machine has Go, Cargo/Rust, cgo, and a working native C toolchain:

```sh
OCT_CHIMERA_HELLO=1 \
OCT_WRAPPER_PATH="$PWD/dist/sidecars" \
go run ./cmd/oct make --file Examples/ChimeraHello/Make.oct TestChimera --trace
```

Known limitations: M0 uses raw command targets, a Rust `staticlib`, cgo, and Unix-like linker flags. It does not add C ABI artifact helper APIs, a C layer, shared library runtime placement, CMake/Ninja compatibility, or cross-platform executable suffix helpers.

## C ABI metadata

`RustArtifact()` returns a `Make.CAbiLibrary` record for the Rust `staticlib` (`rust/target/release/libchimera_rust.a`) and exported header (`rust/chimera_rust.h`). In MAKE11 this is metadata only: the real build commands are unchanged, and no helper automatically consumes the record yet. Future Make helpers can use this shape to derive cgo flags, C/C++ link arguments, Rust build-script link metadata, plan snapshots, and trace evidence.
