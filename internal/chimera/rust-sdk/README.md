# Rust Chimera SDK M1

`chimera_rust_sdk` is a tiny std-only Rust support crate for Chimera C ABI
exports. It owns the first Rust-side conventions for producing Oct-consumable
native C ABI artifacts without making `Make` responsible for Rust-specific ABI
rituals.

M1 is intentionally narrow: scalar numeric exports callable through a C ABI
boundary, plus process initialization for quiet panic handling.

## Naming convention

The Rust Chimera SDK uses PascalCase for public Chimera APIs to match Oct, Go,
and C# conventions. Rust snake_case is not used for public Chimera API
functions. The crate uses `#![allow(non_snake_case)]` deliberately for this
policy. Rust crate names, Cargo package names, private implementation details,
and constants may still follow Rust/Cargo conventions; the public
Chimera-facing function surface follows Oct convention.

## Provides

- `CHIMERA_ABI_VERSION`: ABI version constant for the M1 convention.
- `CHIMERA_PANIC_I32`: documented `i32::MIN` panic sentinel.
- `ChimeraI32`: optional alias for integer values.
- `InstallQuietPanicHook`: one-time process-global hook that suppresses Rust's
  default panic stderr output.
- `ReturnI32`: panic boundary helper for `i32` C ABI exports.
- `ReturnF32`: panic boundary helper for `f32` C ABI exports, returning NaN on
  panic.
- `ReturnF64`: panic boundary helper for `f64` C ABI exports, returning NaN on
  panic.

`ReturnI32`, `ReturnF32`, and `ReturnF64` catch Rust panics before they can
unwind across an `extern "C"` boundary. Normal return values pass through
unchanged. Panics map to the documented sentinel for the return type.

Float NaN panic sentinels are intentionally minimal and lossy: math functions
can legitimately return NaN. Robust future APIs should use explicit status and
out-parameter ABI shapes when NaN is part of the valid domain.

## Quiet panic hook

`std::panic::catch_unwind` catches panics, but Rust's default panic hook prints
panic text to stderr before the unwind is caught. `InstallQuietPanicHook`
installs an empty process-global hook with `std::sync::Once` so repeated calls
are safe.

Because the hook is process-global, final C ABI producers should decide when to
call it. The SDK exposes the Rust helper but does not export a global
`ChimeraInit` symbol itself, avoiding symbol collisions when multiple Chimera
libraries are linked into one final program. A produced library may export its
own C ABI init function, conventionally `ChimeraInit`, that calls
`chimera_rust_sdk::InstallQuietPanicHook()`.

## String convention for future APIs

M1 does not provide string APIs. The preferred future convention is a
caller-buffer pattern:

- caller provides a pointer and buffer length;
- callee returns a required length, written length, or explicit status;
- no Rust heap ownership is transferred across the ABI;
- heap-allocated strings with paired `free` functions are discouraged until
  ownership annotations and header generation are designed;
- static strings are acceptable only for static immutable messages.

Go consumers may allocate C memory with `C.malloc` and pass it as a buffer. Go
pointers must not be stored in C/Rust-owned memory, and no pointer should be
kept past the call unless ownership and lifetime are explicitly modeled.

## Thread-local warning

Rust `thread_local!` state maps to OS threads. Go goroutines are scheduled M:N
over OS threads and may resume on a different OS thread after cgo boundaries.
Thread-local error APIs are therefore unsafe for Go consumers. Chimera should
prefer explicit output parameters or status/result ABI shapes instead of
thread-local error state.

## Does not provide

- header generation
- bindgen or cbindgen integration
- strings
- pointers
- callbacks
- heap ownership
- structs or structs-by-value
- thread-local error APIs
- Octxiliary
- UIBridge
- Make helper APIs

## Minimal example

```rust
#![allow(non_snake_case)]

#[no_mangle]
pub extern "C" fn ChimeraInit() {
    chimera_rust_sdk::InstallQuietPanicHook();
}

#[no_mangle]
pub extern "C" fn chimera_add_35(x: i32) -> i32 {
    chimera_rust_sdk::ReturnI32(|| x + 35)
}
```

The C header and artifact metadata remain the responsibility of the consuming
example or future tooling. M1 does not overpromise safety beyond the panic
boundary helpers for scalar numeric exports.
