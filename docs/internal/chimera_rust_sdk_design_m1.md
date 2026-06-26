# Chimera Rust SDK M1 API design

## Scope

This note synthesizes the CHIMERA-RUST-SDK-DESIGN review direction for the
repository-local crate at `internal/chimera/rust-sdk`. It covers the Rust SDK
API shape, ChimeraHello adoption, and the Make helper roadmap. It intentionally
keeps Chimera M1 small: scalar numeric panic boundaries and initialization
conventions only.

The requested `Chimera SDK API Recommendation.md` file was not present in this
checkout, so this synthesis evaluates the recommendation points supplied in the
task text rather than an on-disk source file.

## Recommendation synthesis

### Adopt immediately

- Public Rust Chimera SDK functions use PascalCase, not Rust snake_case.
- Rename `return_i32` to `ReturnI32` while the SDK is still early/internal.
- Add `InstallQuietPanicHook` as a one-time quiet panic hook helper.
- Add typed `ReturnF32` and `ReturnF64` helpers.
- Keep `CHIMERA_PANIC_I32` as a Rust constant and expose the same sentinel in
  hand-written C headers that need it.
- Document string and thread-local constraints before adding APIs that could
  accidentally establish unsafe conventions.

These are low-scope SDK improvements that directly improve the current Rust
staticlib producer without adding a full FFI system.

### Defer

- String APIs. The caller-buffer convention should be documented first, then
  implemented only when a real M2 use case needs it.
- Thread-local error APIs. They should remain absent because Go goroutines do
  not have stable OS-thread identity across cgo boundaries.
- Generic return helpers. Typed helpers are clearer for the current API and
  avoid exposing a too-general policy too early.
- Float C sentinel macros. NaN is awkward to define portably in headers and is
  inherently lossy, so headers should document float panic behavior when float
  functions are added.
- C SDK, Zig SDK, Python SDK, and Swift SDK work. Rust is the current C ABI
  producer in ChimeraHello; other languages are future consumers/producers.

### Needs redesign before adoption

- Heap-owned strings with paired frees need header annotations, ownership
  vocabulary, and consumer examples before they are safe enough to standardize.
- Thread-local last-error patterns should be rejected for Go consumers or
  redesigned into explicit status/out-parameter forms.
- `Make.ToolOr` needs careful semantics. Build plans should not silently degrade
  around missing required tools; optional behavior must be explicit in plan data
  or helper names.
- Language-specific Make helpers should not become executor magic. They should
  start as pure helper functions that construct normal `Make.CommandTarget` and
  `Make.CAbiLibrary` records.

### Ownership by area

- Rust SDK: Rust panic hook helper, typed numeric return helpers, Rust-side
  naming policy, panic sentinel constants, and SDK safety documentation.
- Chimera-producing libraries: exported C ABI symbols such as `ChimeraInit`,
  hand-written headers, and the final symbol set for that artifact.
- Make or future Chimera library: pure build-plan helper records/functions.
- Documentation: string conventions, Go/cgo pointer constraints, thread-local
  warnings, and Make helper roadmap.

## Naming convention decision

The Rust Chimera SDK uses PascalCase for public Chimera APIs to match Oct, Go,
and C# conventions. Rust snake_case is not used for public Chimera API
functions.

Specific decisions:

- `return_i32` becomes `ReturnI32`.
- `ReturnF32` is the float32 panic boundary helper.
- `ReturnF64` is the float64 panic boundary helper.
- `InstallQuietPanicHook` installs the quiet panic hook.
- A C ABI init function, when exported by a produced library, is named
  `ChimeraInit` unless a concrete C ABI constraint requires otherwise.
- Constants may remain `SCREAMING_SNAKE_CASE`; that style is normal for Rust
  constants and C defines. The anti-snake rule applies to public Chimera SDK
  functions.

The SDK crate uses `#![allow(non_snake_case)]` deliberately. This is not an
accident or a Rust style oversight; it is Chimera API policy. Rust crate names,
Cargo package names, private implementation details, and source file names may
remain Rust/Cargo-compatible.

## Quiet panic hook and initialization

`std::panic::catch_unwind` prevents unwinding across an `extern "C"` boundary,
but Rust's default panic hook still prints panic text to stderr before the panic
is caught. That pollutes Go/cgo host logs when the panic is already being
converted to an ABI sentinel.

M1 adds:

```rust
pub fn InstallQuietPanicHook()
```

The implementation uses `std::sync::Once`:

```rust
static CHIMERA_PANIC_HOOK_ONCE: Once = Once::new();

pub fn InstallQuietPanicHook() {
    CHIMERA_PANIC_HOOK_ONCE.call_once(|| {
        std::panic::set_hook(Box::new(|_| {}));
    });
}
```

Important behavior:

- Idempotency: repeated calls are safe and install at most once.
- Process-global effect: Rust panic hooks are process-global, not per library.
- Host interaction: this can replace a host's previously configured Rust panic
  hook. For the current ChimeraHello final binary this is acceptable because the
  example owns the Rust staticlib and calls init explicitly.
- Future configurability: configurable hooks may be added later if a real host
  needs structured panic logging.

The SDK should not export `ChimeraInit` directly. If a Rust SDK `rlib` defined a
global exported symbol, multiple Chimera libraries linked into one final program
could collide or imply the SDK owns the final C ABI surface. Instead, M1 exposes
only the Rust helper. Each produced C ABI library may export its own
`ChimeraInit` that calls `InstallQuietPanicHook`.

## Float return helpers

M1 adds typed helpers:

```rust
pub fn ReturnI32<F>(f: F) -> i32
where
    F: FnOnce() -> i32 + std::panic::UnwindSafe

pub fn ReturnF32<F>(f: F) -> f32
where
    F: FnOnce() -> f32 + std::panic::UnwindSafe

pub fn ReturnF64<F>(f: F) -> f64
where
    F: FnOnce() -> f64 + std::panic::UnwindSafe
```

Panic sentinels:

- `ReturnI32`: `CHIMERA_PANIC_I32` (`i32::MIN`).
- `ReturnF32`: `f32::NAN`.
- `ReturnF64`: `f64::NAN`.

NaN panic sentinels are acceptable for M1 because they establish a conventional
minimal panic boundary for scalar floats. They are lossy: valid math functions
can return NaN. Robust APIs should eventually use explicit status/out-parameter
patterns when NaN is part of the legitimate domain.

A generic `ReturnValue<T, F>(panic_value, f)` helper is deferred. It may be
useful internally later, but typed helpers produce a clearer public SDK surface
now.

## C header constants and init convention

Hand-written C headers for Chimera-produced libraries should include C-visible
sentinels they expect consumers to inspect. For integer M1 exports, use:

```c
#define CHIMERA_PANIC_I32 (-2147483647 - 1)
```

This expresses the `int32_t` minimum without relying on implementation-specific
literal parsing of `-2147483648`.

If a produced library exports an init function, declare it explicitly:

```c
void ChimeraInit(void);
```

Float panic sentinels are documented as NaN in Rust SDK documentation. Portable
C `CHIMERA_PANIC_F32` and `CHIMERA_PANIC_F64` macros are deferred until there is
a concrete float C ABI consumer.

## String convention

No string APIs are added in M1.

The preferred future convention is caller-buffer based:

- The caller provides a pointer and buffer length.
- The callee writes into that buffer and returns required length, written
  length, or explicit status.
- Rust heap ownership is not transferred across the ABI.
- Heap-allocated strings with paired free functions are discouraged until
  ownership annotations and header generation are designed.
- Static immutable strings are acceptable only for static messages.

Go consumers may use `C.malloc` for C-owned buffers. Go pointers must not be
stored in C/Rust-owned memory. No pointer should be retained after a call unless
ownership and lifetime are explicitly modeled.

## Thread-local warning

Rust `thread_local!` maps to OS threads. Go goroutines are scheduled M:N over OS
threads, and a goroutine may resume on a different OS thread after cgo calls.
Thread-local last-error APIs are therefore unsafe for Go consumers.

Chimera should avoid thread-local error APIs and prefer explicit return status,
out parameters, or result records once those ABI shapes are designed.

## Make helper roadmap

The reviewed Make helper ideas are useful, but none should be implemented in
this task.

Candidate helpers:

- `RustStaticLib(cargoManifest, profile) -> (CommandTarget, CAbiLibrary)`
- `GoCgoBinary(pkgDir, output, lib) -> CommandTarget`
- `Platform()`
- `ToolOr(...)`

MAKE17 recommendation:

- Prefer a future `Libraries/Chimera` for Chimera-specific pure helper
  functions that return normal `Make.CommandTarget` and `Make.CAbiLibrary`
  records. This keeps interop semantics near Chimera instead of making
  `Libraries/Make` accumulate language-specific helpers.
- Keep `Libraries/Make` focused on general plan schema, host primitives, and
  broadly useful platform/tool discovery. `Make.Platform` may belong there
  because platform discovery is not Chimera-specific.
- Treat `Make.ToolOr` skeptically. If a tool is required, fail explicitly. If a
  tool is optional, the optional behavior should be visible in helper naming or
  typed plan data rather than hidden behind fallback lookup.
- Start with pure helpers only. Do not change `oct make` executor behavior, do
  not add magic Rust or Go build actions, and do not change C ABI records unless
  documentation reveals a schema gap.
- After Rust SDK M1 lands, refactor ChimeraHello to use helper functions in a
  separate MAKE17 task.

## Implemented in this change

- The Rust SDK public function surface now uses PascalCase.
- `ReturnI32` replaces `return_i32`.
- `InstallQuietPanicHook` installs a one-time quiet panic hook.
- `ReturnF32` and `ReturnF64` return NaN on panic.
- ChimeraHello exports `ChimeraInit` from its Rust staticlib and calls it from
  the Go final binary.
- ChimeraHello's C header declares `ChimeraInit` and defines
  `CHIMERA_PANIC_I32`.
- SDK and example documentation describe naming, panic, string, and
  thread-local conventions.

## Deferred work

- No full FFI system.
- No cbindgen or bindgen.
- No procedural macros.
- No external Rust dependencies.
- No string APIs.
- No thread-local error APIs.
- No callbacks.
- No broad pointer or heap ownership model.
- No Make helper APIs.
- No `Make.RustStaticLib` or `Make.GoCgoBinary`.
- No `oct make` executor behavior changes.
- No required native Chimera real runs in default tests.
