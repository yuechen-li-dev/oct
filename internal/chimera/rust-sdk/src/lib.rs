//! Rust Chimera SDK M0 support for C ABI exports.
//!
//! This crate is intentionally tiny and std-only. It provides shared Rust-side
//! conventions for integer-only Chimera C ABI exports, including a panic
//! boundary helper so panics do not unwind across `extern "C"` calls.

/// M0 ABI version used by the Rust Chimera SDK.
pub const CHIMERA_ABI_VERSION: u32 = 0;

/// Sentinel returned by M0 `i32` exports when a panic reaches the SDK boundary.
pub const CHIMERA_PANIC_I32: i32 = i32::MIN;

/// Alias for M0 integer values passed across the C ABI boundary.
pub type ChimeraI32 = i32;

/// Run an `i32` export body while preventing Rust panics from crossing C ABI.
///
/// M0 exports should wrap their implementation with this helper. Normal values
/// are returned unchanged. If the body panics, the panic is caught and mapped to
/// [`CHIMERA_PANIC_I32`].
pub fn return_i32<F>(f: F) -> ChimeraI32
where
    F: FnOnce() -> ChimeraI32 + std::panic::UnwindSafe,
{
    match std::panic::catch_unwind(f) {
        Ok(value) => value,
        Err(_) => CHIMERA_PANIC_I32,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn return_i32_returns_value() {
        assert_eq!(return_i32(|| 42), 42);
    }

    #[test]
    fn return_i32_maps_panic_to_sentinel() {
        let value = return_i32(|| panic!("ffi boundary test panic"));
        assert_eq!(value, CHIMERA_PANIC_I32);
    }
}
