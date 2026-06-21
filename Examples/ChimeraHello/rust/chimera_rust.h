#ifndef CHIMERA_RUST_H
#define CHIMERA_RUST_H

#include <stdint.h>

/*
 * ChimeraHello Rust C ABI surface.
 *
 * The exported function uses the C ABI and returns an M0 integer-only value.
 * Its Rust implementation is wrapped with the Rust Chimera SDK M0 panic
 * boundary helper so Rust panics do not unwind across this header's ABI.
 * If a panic reaches that helper, the Rust side returns the documented
 * chimera_rust_sdk::CHIMERA_PANIC_I32 sentinel, equal to i32::MIN.
 *
 * M0 does not model strings, pointers, callbacks, heap ownership, or structs.
 */
int32_t rust_hello_number(void);

#endif
