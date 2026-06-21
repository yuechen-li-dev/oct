# Rust Chimera SDK M0

`chimera_rust_sdk` is a tiny std-only Rust support crate for Chimera C ABI exports.
It owns the first Rust-side conventions for producing Oct-consumable native C
ABI artifacts without making `Make` responsible for Rust-specific ABI rituals.

M0 is intentionally narrow: integer-only exports that are callable through a C
ABI boundary.

## Provides

- `CHIMERA_ABI_VERSION`: ABI version constant for the M0 convention.
- `CHIMERA_PANIC_I32`: documented `i32::MIN` panic sentinel.
- `ChimeraI32`: optional alias for M0 integer values.
- `return_i32`: panic boundary helper for `i32` C ABI exports.

`return_i32` catches Rust panics before they can unwind across an `extern "C"`
boundary. Normal return values pass through unchanged. Panics map to
`CHIMERA_PANIC_I32`.

## Does not provide

- header generation
- bindgen or cbindgen integration
- strings
- pointers
- callbacks
- heap ownership
- structs or structs-by-value
- Octxiliary
- UIBridge
- Make helper APIs

## Minimal example

```rust
#[no_mangle]
pub extern "C" fn chimera_add_35(x: i32) -> i32 {
    chimera_rust_sdk::return_i32(|| x + 35)
}
```

The C header and artifact metadata remain the responsibility of the consuming
example or future tooling. M0 does not overpromise safety beyond the panic
boundary helper for integer-only exports.
