#[no_mangle]
pub extern "C" fn rust_hello_number() -> i32 {
    chimera_rust_sdk::return_i32(|| 35)
}
