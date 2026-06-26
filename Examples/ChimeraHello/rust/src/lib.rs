#![allow(non_snake_case)]

#[no_mangle]
pub extern "C" fn ChimeraInit() {
    chimera_rust_sdk::InstallQuietPanicHook();
}

#[no_mangle]
pub extern "C" fn rust_hello_number() -> i32 {
    chimera_rust_sdk::ReturnI32(|| 35)
}
