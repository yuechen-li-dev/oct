#ifndef CHIMERA_RUST_H
#define CHIMERA_RUST_H

#include <stdint.h>

#define CHIMERA_PANIC_I32 (-2147483647 - 1)

/*
 * ChimeraHello Rust C ABI surface.
 *
 * ChimeraInit installs the Rust Chimera SDK quiet panic hook for this process.
 * The exported value function uses the C ABI and returns an M1 integer-only
 * value. Its Rust implementation is wrapped with the Rust Chimera SDK M1 panic
 * boundary helper so Rust panics do not unwind across this header's ABI. If a
 * panic reaches that helper, the Rust side returns CHIMERA_PANIC_I32.
 *
 * M1 does not model strings, pointers, callbacks, heap ownership, or structs.
 */
void ChimeraInit(void);
int32_t rust_hello_number(void);

#endif
