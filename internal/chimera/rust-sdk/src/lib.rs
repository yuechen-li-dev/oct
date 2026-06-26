#![allow(non_snake_case)]
//! Rust Chimera SDK M1 support for C ABI exports.
//!
//! This crate is intentionally tiny and std-only. Chimera-facing public
//! functions use PascalCase to match Oct, Go, and C# API conventions rather
//! than Rust snake_case. Rust implementation details may remain idiomatic Rust,
//! but exported SDK helpers deliberately follow the Chimera convention.

use std::sync::Once;

/// M1 ABI version used by the Rust Chimera SDK.
pub const CHIMERA_ABI_VERSION: u32 = 1;

/// Sentinel returned by `i32` exports when a panic reaches the SDK boundary.
pub const CHIMERA_PANIC_I32: i32 = i32::MIN;

/// Alias for integer values passed across the C ABI boundary.
pub type ChimeraI32 = i32;

static CHIMERA_PANIC_HOOK_ONCE: Once = Once::new();

/// Install Chimera's quiet Rust panic hook once for the current process.
///
/// Rust's default panic hook prints panic text to stderr before
/// [`std::panic::catch_unwind`] returns. Chimera C ABI producers often convert
/// panics into ABI sentinels, so this hook suppresses that default stderr noise.
/// The hook is process-global and is installed at most once; consumers that need
/// a custom host panic hook should install their own policy before or instead of
/// calling this helper.
pub fn InstallQuietPanicHook() {
    CHIMERA_PANIC_HOOK_ONCE.call_once(|| {
        std::panic::set_hook(Box::new(|_| {}));
    });
}

/// Run an `i32` export body while preventing Rust panics from crossing C ABI.
///
/// Normal values are returned unchanged. If the body panics, the panic is caught
/// and mapped to [`CHIMERA_PANIC_I32`].
pub fn ReturnI32<F>(f: F) -> ChimeraI32
where
    F: FnOnce() -> ChimeraI32 + std::panic::UnwindSafe,
{
    match std::panic::catch_unwind(f) {
        Ok(value) => value,
        Err(_) => CHIMERA_PANIC_I32,
    }
}

/// Run an `f32` export body while preventing Rust panics from crossing C ABI.
///
/// Normal values are returned unchanged. If the body panics, the panic is caught
/// and mapped to `f32::NAN`. This sentinel is lossy because valid math may also
/// produce NaN; robust future APIs should use explicit status/out-parameter ABI
/// shapes when NaN is a meaningful domain value.
pub fn ReturnF32<F>(f: F) -> f32
where
    F: FnOnce() -> f32 + std::panic::UnwindSafe,
{
    match std::panic::catch_unwind(f) {
        Ok(value) => value,
        Err(_) => f32::NAN,
    }
}

/// Run an `f64` export body while preventing Rust panics from crossing C ABI.
///
/// Normal values are returned unchanged. If the body panics, the panic is caught
/// and mapped to `f64::NAN`. This sentinel is lossy because valid math may also
/// produce NaN; robust future APIs should use explicit status/out-parameter ABI
/// shapes when NaN is a meaningful domain value.
pub fn ReturnF64<F>(f: F) -> f64
where
    F: FnOnce() -> f64 + std::panic::UnwindSafe,
{
    match std::panic::catch_unwind(f) {
        Ok(value) => value,
        Err(_) => f64::NAN,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ReturnI32ReturnsValue() {
        assert_eq!(ReturnI32(|| 42), 42);
    }

    #[test]
    fn ReturnI32MapsPanicToSentinel() {
        let value = ReturnI32(|| panic!("ffi boundary test panic"));
        assert_eq!(value, CHIMERA_PANIC_I32);
    }

    #[test]
    fn ReturnF32ReturnsValue() {
        assert_eq!(ReturnF32(|| 3.5), 3.5);
    }

    #[test]
    fn ReturnF32MapsPanicToNan() {
        let value = ReturnF32(|| panic!("ffi boundary test panic"));
        assert!(value.is_nan());
    }

    #[test]
    fn ReturnF64ReturnsValue() {
        assert_eq!(ReturnF64(|| 7.25), 7.25);
    }

    #[test]
    fn ReturnF64MapsPanicToNan() {
        let value = ReturnF64(|| panic!("ffi boundary test panic"));
        assert!(value.is_nan());
    }

    #[test]
    fn InstallQuietPanicHookIsIdempotent() {
        InstallQuietPanicHook();
        InstallQuietPanicHook();
    }
}
